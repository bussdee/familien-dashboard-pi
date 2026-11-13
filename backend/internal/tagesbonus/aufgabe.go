package tagesbonus

import (
	"hash/fnv"
	"net/http"
	"strconv"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/rs/zerolog/log"
)

// Die Tagesaufgabe: jeden Tag eine kleine Herausforderung aus echter Arbeit,
// für alle Kinder dieselbe. Wer sie schafft, bekommt einen Bonus
// (bonus.aufgabe). Damit belohnt das tägliche Öffnen auch das Mithelfen.
//
// Gezählt werden nur Erledigungen, die nicht mehr auf die Eltern warten. Ob
// die Aufgabe geschafft ist, prüft der Server bei jeder Abfrage; den Bonus gibt
// es einmal, beim ersten Mal, an dem es so weit ist.
//
// Die Texte stehen in der Oberfläche (übersetzbar); hier gibt es nur Kennung
// und Ziel.

// QuelleAufgabe ist die Kennung der Punkte im Verlauf.
const QuelleAufgabe = "tagesaufgabe"

type aufgabeDef struct {
	ID   string
	Ziel int
}

// Aufgaben ist der Vorrat. Jede muss an jedem Tag mit Aufgaben machbar sein.
var Aufgaben = []aufgabeDef{
	{ID: "eine", Ziel: 1},      // Erledige heute eine Aufgabe
	{ID: "zwei", Ziel: 2},      // Erledige heute zwei Aufgaben
	{ID: "vormittag", Ziel: 1}, // Erledige eine Aufgabe vor 12 Uhr
	{ID: "punkte15", Ziel: 15}, // Sammle heute 15 Punkte mit Aufgaben
}

// aufgabeFuer wählt die Aufgabe eines Tages. Ein Streuwert statt der Reihe
// nach, damit nicht jeden Montag dasselbe dran ist.
func aufgabeFuer(tag string) aufgabeDef {
	h := fnv.New32a()
	_, _ = h.Write([]byte(tag))
	return Aufgaben[int(h.Sum32()%uint32(len(Aufgaben)))]
}

// AufgabeStand ist die Tagesaufgabe für die Oberfläche.
type AufgabeStand struct {
	Berechtigt  bool   `json:"eligible"`
	ID          string `json:"id"`
	Ziel        int    `json:"goal"`
	Fortschritt int    `json:"progress"`
	Geschafft   bool   `json:"done"`
	// Neu: gerade eben geschafft und gutgeschrieben.
	Neu    bool `json:"claimed"`
	Punkte int  `json:"points"`
}

// fortschritt zählt die heutigen Erledigungen. Die Zeitstempel liegen in der
// Datenbank in zwei Textformaten; gefiltert wird deshalb hier, in Ortszeit,
// nicht im SQL.
func (s *Service) fortschritt(uid int, a aufgabeDef, jetzt time.Time) (int, error) {
	rows, err := s.db.Query(`SELECT completed_at, points_awarded FROM chore_completions
		WHERE user_id = ? AND pending = 0 ORDER BY id DESC LIMIT 200`, uid)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	heute := jetzt.Format(tagFormat)
	anzahl, punkte, vormittag := 0, 0, 0
	for rows.Next() {
		var wann time.Time
		var p int
		if err := rows.Scan(&wann, &p); err != nil {
			continue
		}
		lokal := wann.In(jetzt.Location())
		if lokal.Format(tagFormat) != heute {
			continue
		}
		anzahl++
		punkte += p
		if lokal.Hour() < 12 {
			vormittag++
		}
	}
	switch a.ID {
	case "punkte15":
		return punkte, rows.Err()
	case "vormittag":
		return vormittag, rows.Err()
	}
	return anzahl, rows.Err()
}

// Aufgabe zeigt die Tagesaufgabe und schreibt den Bonus gut, wenn sie gerade
// geschafft ist.
func (s *Service) Aufgabe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.GetUserID(r)
	rolle, _ := auth.GetUserRole(r)
	if !ok || rolle != "member" || !s.schalter.An("modul.tagesaufgabe") {
		auth.WriteJSON(w, AufgabeStand{})
		return
	}
	belohnung, _ := strconv.Atoi(s.schalter.Wert("bonus.aufgabe"))
	jetzt := s.jetzt()
	heute := jetzt.Format(tagFormat)
	a := aufgabeFuer(heute)

	st := AufgabeStand{Berechtigt: true, ID: a.ID, Ziel: a.Ziel, Punkte: belohnung}

	var schonDa int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM daily_quest WHERE user_id = ? AND day = ?`, uid, heute).Scan(&schonDa)

	n, err := s.fortschritt(uid, a, jetzt)
	if err != nil {
		log.Error().Err(err).Msg("Tagesaufgabe konnte nicht geprüft werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	st.Fortschritt = min(n, a.Ziel)
	if schonDa > 0 {
		st.Geschafft = true
		auth.WriteJSON(w, st)
		return
	}
	if n < a.Ziel {
		auth.WriteJSON(w, st)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.Exec(`INSERT OR IGNORE INTO daily_quest (user_id, day, quest, points) VALUES (?, ?, ?, ?)`,
		uid, heute, a.ID, belohnung)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if rows, _ := res.RowsAffected(); rows > 0 {
		if belohnung > 0 {
			if err := points.Award(tx, uid, QuelleAufgabe, 0, belohnung, ""); err != nil {
				log.Error().Err(err).Msg("Tagesaufgabe konnte nicht gutgeschrieben werden")
				auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
				return
			}
		}
		st.Neu = true
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	st.Geschafft = true
	auth.WriteJSON(w, st)
}
