// Package tagesbonus belohnt Kinder dafür, das Dashboard jeden Tag zu öffnen.
//
// Am ersten Tag gibt es 1 Punkt, am zweiten Tag in Folge 2, am dritten 3 und
// so weiter. Fehlt ein Tag, beginnt die Serie wieder bei 1. Die Eltern können
// eine Obergrenze setzen (bonus.max), damit der Bonus nach Wochen nicht mehr
// wert ist als die Aufgaben selbst; 0 heißt: keine Grenze.
//
// Dazu kommen:
//
//   - Meilensteine: Bei 7, 14, 30, 100, 200 und 365 Tagen in Folge gibt es
//     einmal einen Extra-Bonus (bonus.meilenstein).
//   - Serienschutz: Auf Wunsch (bonus.joker) verzeiht ein Joker pro Woche
//     einen einzelnen verpassten Tag. Der gerettete Tag steht im Kalender, mit
//     0 Punkten.
//   - Die Tagesaufgabe (aufgabe.go) und der Kalender der Serie.
//
// Abgeholt wird beim Öffnen der App, nicht mit einem Knopf: Der Anreiz ist das
// Vorbeischauen, nicht das Finden eines Knopfes. Der Server zählt nach seiner
// eigenen Uhr und Zeitzone (TZ), nicht nach der des Geräts — sonst ließe sich
// die Serie mit einer verstellten Uhr retten.
//
// Nur Kinder (Rolle member) bekommen den Bonus, und nur persönlich angemeldet;
// am Wandgerät ist niemand. Der Schalter modul.tagesbonus steht
// standardmäßig aus.
//
// Eine Regel für alles hier: Innerhalb einer Transaktion fragt nichts mehr nach
// den Einstellungen. Die Datenbank hat nur eine Verbindung, und die hält dann
// die Transaktion — die Abfrage würde ewig warten. Alle Werte werden vorher
// gelesen (werte).
package tagesbonus

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/rs/zerolog/log"
)

// Quellen der Punkte im Verlauf.
const (
	Quelle            = "tagesbonus"
	QuelleMeilenstein = "meilenstein"
)

const tagFormat = "2006-01-02"

// Meilensteine sind die Serienlängen, bei denen es einen Extra-Bonus gibt.
var Meilensteine = []int{7, 14, 30, 100, 200, 365}

// jokerTage: So viele Tage muss der letzte Joker zurückliegen.
const jokerTage = 7

// Schalter liefert die Einstellungen der Familie.
type Schalter interface {
	An(key string) bool
	Wert(key string) string
}

type Service struct {
	db       *sql.DB
	schalter Schalter
	// jetzt ersetzt in Tests die Uhr.
	jetzt func() time.Time
}

func NewService(db *sql.DB, schalter Schalter) *Service {
	return &Service{db: db, schalter: schalter, jetzt: time.Now}
}

// Stand ist, was die Oberfläche über den Bonus wissen muss.
type Stand struct {
	// Berechtigt: Diese Person bekommt überhaupt einen Bonus.
	Berechtigt bool `json:"eligible"`
	// Neu: Der Bonus wurde gerade eben gutgeschrieben (nur bei POST).
	Neu bool `json:"claimed"`
	// HeuteAbgeholt: Für heute ist der Bonus schon da.
	HeuteAbgeholt bool `json:"claimed_today"`
	// Serie: Tage in Folge, heute eingerechnet, wenn abgeholt.
	Serie int `json:"streak"`
	// Punkte: was es heute gab (oder gibt).
	Punkte int `json:"points"`
	// Morgen: was es morgen gibt, wenn die Serie hält.
	Morgen int `json:"tomorrow"`

	// JokerGenutzt: Der Serienschutz hat gerade den gestrigen Tag gerettet.
	JokerGenutzt bool `json:"joker_used"`
	// JokerFrei: Ein Joker steht bereit (nur, wenn der Serienschutz an ist).
	JokerFrei bool `json:"joker_available"`
	// Meilenstein: gerade erreicht (0 = keiner), mit Extra-Punkten.
	Meilenstein       int `json:"milestone"`
	MeilensteinPunkte int `json:"milestone_points"`
	// NaechsterMeilenstein und wie viele Tage es bis dahin sind.
	NaechsterMeilenstein int `json:"next_milestone"`
	TageBisMeilenstein   int `json:"days_to_milestone"`
}

// werte sind die Einstellungen, die vor jeder Transaktion gelesen werden.
type werte struct {
	max         int
	meilenstein int
	joker       bool
}

func (s *Service) zahl(key string) int {
	n, err := strconv.Atoi(s.schalter.Wert(key))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (s *Service) lesen() werte {
	return werte{max: s.zahl("bonus.max"), meilenstein: s.zahl("bonus.meilenstein"), joker: s.schalter.An("bonus.joker")}
}

// wert ist der Bonus für einen Tag der Serie.
func wert(serie, max int) int {
	if max > 0 && serie > max {
		return max
	}
	return serie
}

func naechsterMeilenstein(serie int) (int, int) {
	for _, m := range Meilensteine {
		if m > serie {
			return m, m - serie
		}
	}
	return 0, 0
}

func istMeilenstein(serie int) bool {
	for _, m := range Meilensteine {
		if m == serie {
			return true
		}
	}
	return false
}

// berechtigt: ein angemeldetes Kind, und der Bereich ist eingeschaltet.
func (s *Service) berechtigt(r *http.Request) (int, bool) {
	uid, ok := auth.GetUserID(r)
	if !ok {
		return 0, false
	}
	if rolle, _ := auth.GetUserRole(r); rolle != "member" {
		return 0, false
	}
	if s.schalter != nil && !s.schalter.An("modul.tagesbonus") {
		return 0, false
	}
	return uid, true
}

type abfrage interface {
	QueryRow(query string, args ...any) *sql.Row
}

func serieAm(q abfrage, uid int, tag string) (serie, punkte int, ok bool, err error) {
	err = q.QueryRow(`SELECT streak, points FROM daily_bonus WHERE user_id = ? AND day = ?`, uid, tag).
		Scan(&serie, &punkte)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, nil
	}
	return serie, punkte, err == nil, err
}

// jokerFrei: Der letzte Joker liegt mindestens eine Woche zurück.
func jokerFrei(q abfrage, uid int, heute time.Time) (bool, error) {
	grenze := heute.AddDate(0, 0, -jokerTage).Format(tagFormat)
	var n int
	err := q.QueryRow(`SELECT COUNT(*) FROM daily_bonus WHERE user_id = ? AND joker = 1 AND day > ?`, uid, grenze).Scan(&n)
	return n == 0, err
}

func (s *Service) ergaenzen(st *Stand, w werte, q abfrage, uid int, heute time.Time) {
	basis := st.Serie
	if !st.HeuteAbgeholt {
		basis = st.Serie + 1
	}
	st.NaechsterMeilenstein, st.TageBisMeilenstein = naechsterMeilenstein(basis)
	if !st.HeuteAbgeholt && istMeilenstein(basis) {
		st.NaechsterMeilenstein, st.TageBisMeilenstein = basis, 1
	}
	if w.joker {
		frei, err := jokerFrei(q, uid, heute)
		st.JokerFrei = err == nil && frei
	}
}

// Get zeigt den Stand, ohne etwas gutzuschreiben.
func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.berechtigt(r)
	if !ok {
		auth.WriteJSON(w, Stand{})
		return
	}
	jetzt := s.jetzt()
	heute := jetzt.Format(tagFormat)
	gestern := jetzt.AddDate(0, 0, -1).Format(tagFormat)
	wt := s.lesen()

	st := Stand{Berechtigt: true}
	serie, punkte, da, err := serieAm(s.db, uid, heute)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if da {
		st.HeuteAbgeholt, st.Serie, st.Punkte, st.Morgen = true, serie, punkte, wert(serie+1, wt.max)
	} else {
		vorher, _, gesternDa, err := serieAm(s.db, uid, gestern)
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		if gesternDa {
			st.Serie = vorher
		}
		st.Punkte = wert(st.Serie+1, wt.max)
		st.Morgen = wert(st.Serie+2, wt.max)
	}
	s.ergaenzen(&st, wt, s.db, uid, jetzt)
	auth.WriteJSON(w, st)
}

// Claim holt den Bonus für heute ab. Ein zweiter Aufruf am selben Tag ändert
// nichts und meldet nur den Stand.
func (s *Service) Claim(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.berechtigt(r)
	if !ok {
		auth.WriteJSON(w, Stand{})
		return
	}
	jetzt := s.jetzt()
	heute := jetzt.Format(tagFormat)
	gestern := jetzt.AddDate(0, 0, -1).Format(tagFormat)
	vorgestern := jetzt.AddDate(0, 0, -2).Format(tagFormat)
	wt := s.lesen()

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	fehler := func(err error, was string) {
		log.Error().Err(err).Msg(was)
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
	}

	if serie, punkte, da, err := serieAm(tx, uid, heute); err != nil {
		fehler(err, "Tagesbonus konnte nicht gelesen werden")
		return
	} else if da {
		st := Stand{Berechtigt: true, HeuteAbgeholt: true, Serie: serie, Punkte: punkte, Morgen: wert(serie+1, wt.max)}
		s.ergaenzen(&st, wt, tx, uid, jetzt)
		auth.WriteJSON(w, st)
		return
	}

	vorher, _, gesternDa, err := serieAm(tx, uid, gestern)
	if err != nil {
		fehler(err, "Tagesbonus konnte nicht gelesen werden")
		return
	}

	// Serienschutz: Fehlt genau gestern, war vorgestern noch dabei und ist ein
	// Joker frei, gilt gestern als gerettet — mit 0 Punkten im Kalender.
	jokerGenutzt := false
	if !gesternDa && wt.joker {
		v, _, vorgesternDa, err := serieAm(tx, uid, vorgestern)
		if err != nil {
			fehler(err, "Tagesbonus konnte nicht gelesen werden")
			return
		}
		if vorgesternDa {
			frei, err := jokerFrei(tx, uid, jetzt)
			if err != nil {
				fehler(err, "Serienschutz konnte nicht geprüft werden")
				return
			}
			if frei {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO daily_bonus (user_id, day, streak, points, joker) VALUES (?, ?, ?, 0, 1)`,
					uid, gestern, v+1); err != nil {
					fehler(err, "Serienschutz konnte nicht gespeichert werden")
					return
				}
				vorher, gesternDa, jokerGenutzt = v+1, true, true
			}
		}
	}

	serie := 1
	if gesternDa {
		serie = vorher + 1
	}
	punkte := wert(serie, wt.max)

	// INSERT OR IGNORE: Öffnet das Kind die App auf zwei Geräten gleichzeitig,
	// gibt es den Bonus trotzdem nur einmal.
	res, err := tx.Exec(`INSERT OR IGNORE INTO daily_bonus (user_id, day, streak, points, hour) VALUES (?, ?, ?, ?, ?)`,
		uid, heute, serie, punkte, jetzt.Hour())
	if err != nil {
		fehler(err, "Tagesbonus konnte nicht gespeichert werden")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		st := Stand{Berechtigt: true, HeuteAbgeholt: true, Serie: serie, Punkte: punkte, Morgen: wert(serie+1, wt.max)}
		auth.WriteJSON(w, st)
		return
	}
	if err := points.Award(tx, uid, Quelle, 0, punkte, ""); err != nil {
		fehler(err, "Tagesbonus konnte nicht gutgeschrieben werden")
		return
	}

	st := Stand{Berechtigt: true, Neu: true, HeuteAbgeholt: true, Serie: serie, Punkte: punkte,
		Morgen: wert(serie+1, wt.max), JokerGenutzt: jokerGenutzt}
	if istMeilenstein(serie) && wt.meilenstein > 0 {
		if err := points.Award(tx, uid, QuelleMeilenstein, serie, wt.meilenstein, ""); err != nil {
			fehler(err, "Meilenstein konnte nicht gutgeschrieben werden")
			return
		}
		st.Meilenstein, st.MeilensteinPunkte = serie, wt.meilenstein
	}
	s.ergaenzen(&st, wt, tx, uid, jetzt)
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, st)
}

// ------------------------------------------------------------------ Kalender

// KalenderTag ist ein Tag mit Bonus.
type KalenderTag struct {
	Tag    string `json:"day"`
	Serie  int    `json:"streak"`
	Punkte int    `json:"points"`
	Joker  bool   `json:"joker"`
}

// Kalender liefert die Tage eines Monats (?monat=JJJJ-MM, sonst der laufende)
// und die längste Serie überhaupt.
func (s *Service) Kalender(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}
	monat := strings.TrimSpace(r.URL.Query().Get("monat"))
	if monat == "" {
		monat = s.jetzt().Format("2006-01")
	}
	if _, err := time.Parse("2006-01", monat); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültiger Monat")
		return
	}

	rows, err := s.db.Query(`SELECT day, streak, points, joker FROM daily_bonus
		WHERE user_id = ? AND day LIKE ? ORDER BY day`, uid, monat+"-%")
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()
	tage := []KalenderTag{}
	for rows.Next() {
		var k KalenderTag
		if err := rows.Scan(&k.Tag, &k.Serie, &k.Punkte, &k.Joker); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		tage = append(tage, k)
	}
	var beste, tageGesamt int
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(streak), 0), COUNT(*) FROM daily_bonus WHERE user_id = ? AND joker = 0`, uid).
		Scan(&beste, &tageGesamt)
	auth.WriteJSON(w, map[string]any{
		"month": monat, "days": tage, "best": beste, "total_days": tageGesamt, "milestones": Meilensteine,
	})
}
