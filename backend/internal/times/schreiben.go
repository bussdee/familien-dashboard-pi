package times

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// darfBearbeiten: Jeder pflegt seine eigenen Zeiten, ein Administrator die
// aller. Das ist die Regel, die zum Alltag passt — die Eltern tragen für das
// Kind ein, das Kind sieht seinen Stundenplan und darf ihn korrigieren.
//
// Am Wandgerät ist niemand angemeldet. Dort wird nur gelesen; die Route hängt
// entsprechend hinter der PersonMiddleware.
func darfBearbeiten(r *http.Request, zielID int) bool {
	rolle, _ := auth.GetUserRole(r)
	if rolle == "admin" {
		return true
	}
	eigene, ok := auth.GetUserID(r)
	return ok && eigene == zielID
}

var uhrzeit = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
var datum = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// pruefeZeiten nimmt Anfang und Ende ab. Beide leer heisst „den ganzen Tag" —
// das ist bei Urlaub und Krankheit der Normalfall und kein Fehler.
func pruefeZeiten(start, ende string) (string, string, error) {
	start, ende = strings.TrimSpace(start), strings.TrimSpace(ende)
	if start == "" && ende == "" {
		return "", "", nil
	}
	if !uhrzeit.MatchString(start) || !uhrzeit.MatchString(ende) {
		return "", "", fmt.Errorf("Uhrzeit muss wie 08:00 aussehen")
	}
	if ende <= start {
		return "", "", fmt.Errorf("Das Ende muss nach dem Anfang liegen")
	}
	return start, ende, nil
}

// ------------------------------------------------------- Wochenmuster

func (s *Service) ListWeekly(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, user_id, weekday, start_time, end_time, kind, note
		FROM weekly_times ORDER BY user_id, weekday, start_time`)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	liste := []WeeklyEntry{}
	for rows.Next() {
		var e WeeklyEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Weekday, &e.Start, &e.End, &e.Kind, &e.Note); err != nil {
			continue
		}
		liste = append(liste, e)
	}
	auth.WriteJSON(w, liste)
}

type weeklyPayload struct {
	UserID  int    `json:"user_id"`
	Weekday int    `json:"weekday"`
	Start   string `json:"start_time"`
	End     string `json:"end_time"`
	Kind    string `json:"kind"`
	Note    string `json:"note"`
}

func (s *Service) CreateWeekly(w http.ResponseWriter, r *http.Request) {
	var req weeklyPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if req.UserID <= 0 {
		if eigene, ok := auth.GetUserID(r); ok {
			req.UserID = eigene
		}
	}
	if !darfBearbeiten(r, req.UserID) {
		auth.HTTPError(w, http.StatusForbidden, "Das sind nicht deine Zeiten")
		return
	}
	if req.Weekday < 0 || req.Weekday > 6 {
		auth.HTTPError(w, http.StatusBadRequest, "Unbekannter Wochentag")
		return
	}
	// Ein Wochenmuster ohne Uhrzeit ergibt keinen Sinn: „jeden Montag
	// irgendwann weg" hilft beim Planen niemandem.
	start, ende, err := pruefeZeiten(req.Start, req.End)
	if err != nil || start == "" {
		auth.HTTPError(w, http.StatusBadRequest, "Anfang und Ende angeben, z. B. 08:00 bis 13:00")
		return
	}
	if !gueltigeArt(req.Kind) {
		req.Kind = KindSchule
	}

	res, err := s.db.Exec(`
		INSERT INTO weekly_times (user_id, weekday, start_time, end_time, kind, note)
		VALUES (?, ?, ?, ?, ?, ?)`,
		req.UserID, req.Weekday, start, ende, req.Kind, strings.TrimSpace(req.Note))
	if err != nil {
		log.Error().Err(err).Msg("Zeiten: Wochenmuster nicht speicherbar")
		auth.HTTPError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, WeeklyEntry{
		ID: int(id), UserID: req.UserID, Weekday: req.Weekday,
		Start: start, End: ende, Kind: req.Kind, Note: strings.TrimSpace(req.Note),
	})
}

func (s *Service) DeleteWeekly(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var besitzer int
	err = s.db.QueryRow("SELECT user_id FROM weekly_times WHERE id = ?", id).Scan(&besitzer)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Eintrag nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if !darfBearbeiten(r, besitzer) {
		auth.HTTPError(w, http.StatusForbidden, "Das sind nicht deine Zeiten")
		return
	}
	if _, err := s.db.Exec("DELETE FROM weekly_times WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Löschen fehlgeschlagen")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ------------------------------------------------------- Konkrete Tage

func (s *Service) ListDays(w http.ResponseWriter, r *http.Request) {
	von := strings.TrimSpace(r.URL.Query().Get("from"))
	bis := strings.TrimSpace(r.URL.Query().Get("to"))
	if !datum.MatchString(von) {
		von = heute()
	}
	if !datum.MatchString(bis) {
		// Vier Wochen — genau der Zeitraum, den Eltern am Stück eintragen.
		t, _ := time.Parse("2006-01-02", von)
		bis = t.AddDate(0, 0, 27).Format("2006-01-02")
	}

	rows, err := s.db.Query(`
		SELECT id, user_id, day, start_time, end_time, kind, note
		FROM day_times WHERE day >= ? AND day <= ?
		ORDER BY day, user_id, start_time`, von, bis)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	liste := []DayEntry{}
	for rows.Next() {
		var e DayEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Day, &e.Start, &e.End, &e.Kind, &e.Note); err != nil {
			continue
		}
		liste = append(liste, e)
	}
	auth.WriteJSON(w, map[string]any{"from": von, "to": bis, "entries": liste})
}

type dayPayload struct {
	UserID int    `json:"user_id"`
	Day    string `json:"day"`
	Start  string `json:"start_time"`
	End    string `json:"end_time"`
	Kind   string `json:"kind"`
	Note   string `json:"note"`
}

// SaveDays nimmt gleich einen ganzen Schwung entgegen.
//
// Das ist der Kern der Bedienung: Eltern tragen einmal im Monat vier Wochen
// ein. Achtundzwanzig einzelne Anfragen wären achtundzwanzig Gelegenheiten,
// dass eine davon danebengeht und der Plan halb gefüllt zurückbleibt. Also
// alles in einem Rutsch, in einer Transaktion.
//
// Ein Tag mit leerem Anfang UND leerem Ende UND ohne besondere Art wird
// gelöscht statt gespeichert — so räumt man einen versehentlichen Eintrag
// wieder weg, ohne einen eigenen Knopf dafür zu brauchen.
func (s *Service) SaveDays(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []dayPayload `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if len(req.Entries) == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Nichts zu speichern")
		return
	}
	// Vier Wochen mal fünf Personen sind 140. Alles darüber ist kein
	// Wochenplan mehr, sondern ein Versehen oder ein Angriff.
	if len(req.Entries) > 400 {
		auth.HTTPError(w, http.StatusRequestEntityTooLarge, "Zu viele Einträge auf einmal")
		return
	}

	eigene, _ := auth.GetUserID(r)
	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	gespeichert, geloescht := 0, 0
	for _, e := range req.Entries {
		if e.UserID <= 0 {
			e.UserID = eigene
		}
		if !darfBearbeiten(r, e.UserID) {
			auth.HTTPError(w, http.StatusForbidden, "Das sind nicht deine Zeiten")
			return
		}
		if !datum.MatchString(e.Day) {
			auth.HTTPError(w, http.StatusBadRequest, "Datum muss wie 2026-09-10 aussehen")
			return
		}
		if !gueltigeArt(e.Kind) {
			e.Kind = KindArbeit
		}

		start, ende, err := pruefeZeiten(e.Start, e.End)
		if err != nil {
			auth.HTTPError(w, http.StatusBadRequest, e.Day+": "+err.Error())
			return
		}

		// Leer und gewöhnlich heisst: weg damit.
		if start == "" && (e.Kind == KindArbeit || e.Kind == KindSchule || e.Kind == KindSonstiges) {
			res, err := tx.Exec("DELETE FROM day_times WHERE user_id = ? AND day = ?", e.UserID, e.Day)
			if err != nil {
				auth.HTTPError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
				return
			}
			if n, _ := res.RowsAffected(); n > 0 {
				geloescht += int(n)
			}
			continue
		}

		// Ein Tag je Person wird ersetzt, nicht ergänzt. Sonst sammeln sich
		// beim wiederholten Speichern Doppel an.
		if _, err := tx.Exec("DELETE FROM day_times WHERE user_id = ? AND day = ?", e.UserID, e.Day); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
			return
		}
		if _, err := tx.Exec(`
			INSERT INTO day_times (user_id, day, start_time, end_time, kind, note)
			VALUES (?, ?, ?, ?, ?, ?)`,
			e.UserID, e.Day, start, ende, e.Kind, strings.TrimSpace(e.Note)); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
			return
		}
		gespeichert++
	}

	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
		return
	}
	auth.WriteJSON(w, map[string]int{"saved": gespeichert, "removed": geloescht})
}
