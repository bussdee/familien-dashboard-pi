// Package times beantwortet die Frage, die am Küchentisch gestellt wird:
// Wann sind wir eigentlich alle da?
//
// Dahinter liegen zwei verschiedene Leben, und deshalb zwei Tabellen.
//
// Ein Kind hat einen Stundenplan. Der ändert sich ein- bis zweimal im Jahr
// und gilt bis dahin jede Woche gleich — ein **Wochenmuster**.
//
// Eltern arbeiten jede Woche anders und tragen einmal im Monat die nächsten
// vier Wochen ein — **konkrete Tage** mit Datum.
//
// Beides in eine Form zu pressen ginge schief. Ein Wochenmuster in konkrete
// Tage aufzulösen hiesse, für das Kind jedes Jahr zweihundertfünfzig Zeilen
// zu schreiben, die alle dasselbe sagen. Umgekehrt liesse sich eine
// Schichtwoche gar nicht als Muster ausdrücken.
//
// Die Auflösung ist einfach: **Ein konkreter Tag sticht das Wochenmuster.**
// Wer für Dienstag etwas eingetragen hat, für den gilt am Dienstag nur das —
// auch ein Eintrag „frei", der dann das Muster aufhebt.
package times

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
)

// Die Arten von Eintrag. "frei" ist die wichtigste Erfindung darin: Sie hebt
// für einen Tag das Wochenmuster auf, ohne es zu ändern. Ein Feiertag löscht
// nicht den Stundenplan.
const (
	KindArbeit    = "arbeit"
	KindSchule    = "schule"
	KindFrei      = "frei"
	KindUrlaub    = "urlaub"
	KindKrank     = "krank"
	KindSonstiges = "sonstiges"
)

func gueltigeArt(v string) bool {
	switch v {
	case KindArbeit, KindSchule, KindFrei, KindUrlaub, KindKrank, KindSonstiges:
		return true
	}
	return false
}

// abwesend sagt, ob eine Art bedeutet, dass jemand wirklich weg ist. "frei",
// "urlaub" und "krank" heissen: da, aber nicht verplanbar — beziehungsweise
// eben nicht unterwegs.
func abwesend(art string) bool {
	return art == KindArbeit || art == KindSchule || art == KindSonstiges
}

type WeeklyEntry struct {
	ID      int    `json:"id"`
	UserID  int    `json:"user_id"`
	Weekday int    `json:"weekday"` // 0 = Montag
	Start   string `json:"start_time"`
	End     string `json:"end_time"`
	Kind    string `json:"kind"`
	Note    string `json:"note"`
}

type DayEntry struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Day    string `json:"day"` // JJJJ-MM-TT
	Start  string `json:"start_time"`
	End    string `json:"end_time"`
	Kind   string `json:"kind"`
	Note   string `json:"note"`
}

// Block ist ein aufgelöster Eintrag für einen bestimmten Tag — egal, ob er
// aus dem Wochenmuster oder aus einem konkreten Tag stammt.
type Block struct {
	UserID    int    `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmoji string `json:"user_emoji"`
	UserColor string `json:"user_color"`
	Start     string `json:"start_time"`
	End       string `json:"end_time"`
	Kind      string `json:"kind"`
	Note      string `json:"note"`
	// AusMuster sagt, ob der Block aus dem Wochenplan kommt. Die Oberfläche
	// darf das leiser darstellen als einen ausdrücklich eingetragenen Tag.
	AusMuster bool `json:"from_pattern"`

	// Eine Nachtschicht läuft über Mitternacht und gehört deshalb in ZWEI
	// Tage der Übersicht: abends in den einen, morgens in den anderen. Diese
	// beiden Angaben sagen der Oberfläche, welches Ende sie gerade sieht.
	//
	//   GehtWeiter  — der Block endet nicht an diesem Tag, sondern morgen
	//   KommtVonGestern — der Block hat gestern begonnen
	GehtWeiter      bool `json:"continues_tomorrow"`
	KommtVonGestern bool `json:"from_yesterday"`
}

type Day struct {
	Date   string  `json:"date"`
	Blocks []Block `json:"blocks"`
	// AlleDaAb ist die Uhrzeit, ab der niemand mehr unterwegs ist. Leer, wenn
	// ohnehin alle da sind — oder wenn jemand den ganzen Tag weg ist.
	AlleDaAb string `json:"all_home_from,omitempty"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

// ------------------------------------------------------------------ Auflösen

// Overview löst für einen Zeitraum auf, wer wann weg ist.
func (s *Service) Overview(w http.ResponseWriter, r *http.Request) {
	von := heute()
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		if _, err := time.Parse("2006-01-02", v); err == nil {
			von = v
		}
	}
	tage := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 60 {
			tage = n
		}
	}

	start, err := time.Parse("2006-01-02", von)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültiges Datum")
		return
	}
	bis := start.AddDate(0, 0, tage-1).Format("2006-01-02")

	personen, err := s.personen()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	muster, err := s.musterNachPerson()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// Einen Tag früher anfangen: Eine Nachtschicht, die am Vortag begonnen
	// hat, ragt in den ersten angezeigten Tag hinein. Ohne den Vortag fehlte
	// sie dort.
	vortag := start.AddDate(0, 0, -1).Format("2006-01-02")
	konkret, err := s.tageZwischen(vortag, bis)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	ergebnis := make([]Day, 0, tage)
	for i := 0; i < tage; i++ {
		datum := start.AddDate(0, 0, i)
		schluessel := datum.Format("2006-01-02")
		// time.Weekday zählt ab Sonntag, wir ab Montag.
		wochentag := (int(datum.Weekday()) + 6) % 7

		gestern := datum.AddDate(0, 0, -1)
		gesternSchluessel := gestern.Format("2006-01-02")
		gesternWochentag := (int(gestern.Weekday()) + 6) % 7

		tag := Day{Date: schluessel, Blocks: []Block{}}
		for _, p := range personen {
			// Was an diesem Tag beginnt.
			for _, b := range bloeckeFuer(p, konkret[schluessel][p.ID], muster[p.ID], wochentag) {
				tag.Blocks = append(tag.Blocks, b)
			}
			// Und was gestern begann und bis heute reicht — die Nachtschicht.
			for _, b := range bloeckeFuer(p, konkret[gesternSchluessel][p.ID], muster[p.ID], gesternWochentag) {
				if !ueberMitternacht(b.Start, b.End) {
					continue
				}
				b.KommtVonGestern = true
				b.GehtWeiter = false
				// Am heutigen Tag ist davon nur der Morgen übrig.
				b.Start = "00:00"
				tag.Blocks = append(tag.Blocks, b)
			}
		}

		sort.Slice(tag.Blocks, func(a, b int) bool {
			if tag.Blocks[a].Start != tag.Blocks[b].Start {
				return tag.Blocks[a].Start < tag.Blocks[b].Start
			}
			return tag.Blocks[a].UserName < tag.Blocks[b].UserName
		})
		tag.AlleDaAb = alleDaAb(tag.Blocks)
		ergebnis = append(ergebnis, tag)
	}

	auth.WriteJSON(w, map[string]any{"days": ergebnis, "people": personen})
}

// alleDaAb ist die späteste Rückkehr des Tages. Genau die Zahl, die man beim
// Planen braucht: ab wann sind alle da.
//
// Drei Fälle geben keine solche Zeit her, und dann bleibt das Feld leer,
// statt eine zu erfinden:
//
//   - Jemand ist ohne Uhrzeit eingetragen, also ganztägig weg.
//   - Jemand geht in die Nachtschicht und kommt an diesem Tag nicht zurück.
//     Die späteste Uhrzeit wäre dann sein Dienstbeginn — als Rückkehr
//     gelesen wäre das schlicht falsch.
//   - Niemand ist unterwegs. Dann braucht es die Zeile gar nicht.
func alleDaAb(blocks []Block) string {
	spaeteste := ""
	for _, b := range blocks {
		if !abwesend(b.Kind) {
			continue
		}
		if b.End == "" || b.GehtWeiter {
			return ""
		}
		if b.End > spaeteste {
			spaeteste = b.End
		}
	}
	return spaeteste
}

// ueberMitternacht: Endet ein Block VOR seinem Anfang, läuft er in den
// nächsten Tag. Das ist die Verabredung, mit der Dienstpläne seit jeher
// arbeiten — 20:00 bis 07:00 ist eine Nachtschicht und keine Fehleingabe.
// Gleiche Zeiten sind mehrdeutig (null oder vierundzwanzig Stunden) und
// werden beim Eintragen abgelehnt.
func ueberMitternacht(start, ende string) bool {
	return start != "" && ende != "" && ende < start
}

// bloeckeFuer liefert die Blöcke einer Person an einem Tag. Ein konkreter
// Eintrag sticht das Wochenmuster — auch ein Eintrag "frei", der es damit für
// diesen einen Tag aufhebt, ohne es zu löschen.
func bloeckeFuer(p Person, eintraege []DayEntry, muster []WeeklyEntry, wochentag int) []Block {
	var raus []Block
	if len(eintraege) == 0 {
		for _, m := range muster {
			if m.Weekday != wochentag {
				continue
			}
			b := blockAus(p, m.Start, m.End, m.Kind, m.Note, true)
			b.GehtWeiter = ueberMitternacht(m.Start, m.End)
			raus = append(raus, b)
		}
		return raus
	}
	for _, e := range eintraege {
		b := blockAus(p, e.Start, e.End, e.Kind, e.Note, false)
		b.GehtWeiter = ueberMitternacht(e.Start, e.End)
		raus = append(raus, b)
	}
	return raus
}

func blockAus(p Person, start, ende, art, notiz string, ausMuster bool) Block {
	return Block{
		UserID: p.ID, UserName: p.Name, UserEmoji: p.Emoji, UserColor: p.Color,
		Start: start, End: ende, Kind: art, Note: notiz, AusMuster: ausMuster,
	}
}

type Person struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Emoji string `json:"avatar_emoji"`
	Color string `json:"color"`
}

func (s *Service) personen() ([]Person, error) {
	rows, err := s.db.Query("SELECT id, name, avatar_emoji, color FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	liste := []Person{}
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Emoji, &p.Color); err != nil {
			continue
		}
		liste = append(liste, p)
	}
	return liste, rows.Err()
}

func (s *Service) musterNachPerson() (map[int][]WeeklyEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, weekday, start_time, end_time, kind, note
		FROM weekly_times ORDER BY weekday, start_time`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nach := map[int][]WeeklyEntry{}
	for rows.Next() {
		var e WeeklyEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Weekday, &e.Start, &e.End, &e.Kind, &e.Note); err != nil {
			continue
		}
		nach[e.UserID] = append(nach[e.UserID], e)
	}
	return nach, rows.Err()
}

func (s *Service) tageZwischen(von, bis string) (map[string]map[int][]DayEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, day, start_time, end_time, kind, note
		FROM day_times WHERE day >= ? AND day <= ? ORDER BY day, start_time`, von, bis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nach := map[string]map[int][]DayEntry{}
	for rows.Next() {
		var e DayEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Day, &e.Start, &e.End, &e.Kind, &e.Note); err != nil {
			continue
		}
		if nach[e.Day] == nil {
			nach[e.Day] = map[int][]DayEntry{}
		}
		nach[e.Day][e.UserID] = append(nach[e.Day][e.UserID], e)
	}
	return nach, rows.Err()
}

func heute() string { return time.Now().Format("2006-01-02") }
