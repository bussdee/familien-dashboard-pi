// Package tagesbonus belohnt Kinder dafür, das Dashboard jeden Tag zu öffnen.
//
// Am ersten Tag gibt es 1 Punkt, am zweiten Tag in Folge 2, am dritten 3 und
// so weiter. Fehlt ein Tag, beginnt die Serie wieder bei 1. Die Eltern können
// eine Obergrenze setzen (bonus.max), damit der Bonus nach Wochen nicht mehr
// wert ist als die Aufgaben selbst; 0 heißt: keine Grenze.
//
// Abgeholt wird beim Öffnen der App, nicht mit einem Knopf: Der Anreiz ist das
// Vorbeischauen, nicht das Finden eines Knopfes. Der Server zählt nach seiner
// eigenen Uhr und Zeitzone (TZ), nicht nach der des Geräts — sonst ließe sich
// die Serie mit einer verstellten Uhr retten.
//
// Nur Kinder (Rolle member) bekommen den Bonus, und nur persönlich angemeldet;
// am Wandgerät ist niemand. Der Schalter modul.tagesbonus steht
// standardmäßig aus.
package tagesbonus

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/rs/zerolog/log"
)

// Quelle ist die Kennung der Punkte im Verlauf.
const Quelle = "tagesbonus"

const tagFormat = "2006-01-02"

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
}

func (s *Service) obergrenze() int {
	n, err := strconv.Atoi(s.schalter.Wert("bonus.max"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// wert ist der Bonus für einen Tag der Serie. Die Obergrenze wird vorher
// gelesen und hereingereicht: Innerhalb einer Transaktion darf nichts mehr
// die Datenbank fragen — sie hat nur eine Verbindung, und die hält gerade
// die Transaktion.
func wert(serie, max int) int {
	if max > 0 && serie > max {
		return max
	}
	return serie
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

	max := s.obergrenze()
	st := Stand{Berechtigt: true}
	serie, punkte, da, err := serieAm(s.db, uid, heute)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if da {
		st.HeuteAbgeholt, st.Serie, st.Punkte, st.Morgen = true, serie, punkte, wert(serie+1, max)
		auth.WriteJSON(w, st)
		return
	}
	vorher, _, gesternDa, err := serieAm(s.db, uid, gestern)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if gesternDa {
		st.Serie = vorher
	}
	st.Punkte = wert(st.Serie+1, max)
	st.Morgen = wert(st.Serie+2, max)
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
	max := s.obergrenze()

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if serie, punkte, da, err := serieAm(tx, uid, heute); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	} else if da {
		auth.WriteJSON(w, Stand{Berechtigt: true, HeuteAbgeholt: true, Serie: serie, Punkte: punkte, Morgen: wert(serie+1, max)})
		return
	}

	vorher, _, gesternDa, err := serieAm(tx, uid, gestern)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	serie := 1
	if gesternDa {
		serie = vorher + 1
	}
	punkte := wert(serie, max)

	// INSERT OR IGNORE: Öffnet das Kind die App auf zwei Geräten gleichzeitig,
	// gibt es den Bonus trotzdem nur einmal.
	res, err := tx.Exec(`INSERT OR IGNORE INTO daily_bonus (user_id, day, streak, points) VALUES (?, ?, ?, ?)`,
		uid, heute, serie, punkte)
	if err != nil {
		log.Error().Err(err).Msg("Tagesbonus konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.WriteJSON(w, Stand{Berechtigt: true, HeuteAbgeholt: true, Serie: serie, Punkte: punkte, Morgen: wert(serie+1, max)})
		return
	}
	if err := points.Award(tx, uid, Quelle, 0, punkte, ""); err != nil {
		log.Error().Err(err).Msg("Tagesbonus konnte nicht gutgeschrieben werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, Stand{Berechtigt: true, Neu: true, HeuteAbgeholt: true, Serie: serie, Punkte: punkte, Morgen: wert(serie+1, max)})
}
