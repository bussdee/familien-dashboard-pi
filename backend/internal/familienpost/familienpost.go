// Package familienpost ist die Post von Mama und Papa: eine kurze Nachricht an
// ein Kind, die erst aufgeht, wenn es das Dashboard das nächste Mal öffnet —
// auf Wunsch mit ein paar Punkten. Ein Grund zum Vorbeischauen, der nichts mit
// Punkten zu tun haben muss.
//
// Schreiben dürfen Eltern, persönlich angemeldet (am Wandgerät ist niemand,
// der Absender sein könnte). Lesen und öffnen darf nur, wer die Nachricht
// bekommt. Die Punkte gibt es beim Öffnen, einmal.
package familienpost

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Quelle ist die Kennung der Punkte im Verlauf.
const Quelle = "post"

const (
	maxText      = 280
	maxBonus     = 100
	maxUngelesen = 10
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

// Brief ist eine Nachricht.
type Brief struct {
	ID        int        `json:"id"`
	An        int        `json:"to_user"`
	AnName    string     `json:"to_name"`
	Von       *int       `json:"from_user"`
	VonName   string     `json:"from_name"`
	VonEmoji  string     `json:"from_emoji"`
	Text      string     `json:"text"`
	Bonus     int        `json:"bonus"`
	Erstellt  time.Time  `json:"created_at"`
	Geoeffnet *time.Time `json:"opened_at"`
}

const spalten = `m.id, m.to_user, COALESCE(t.name, ''), m.from_user, COALESCE(f.name, ''), COALESCE(f.avatar_emoji, ''),
	m.text, m.bonus, m.created_at, m.opened_at`

const quelle = ` FROM family_mail m
	LEFT JOIN users t ON t.id = m.to_user
	LEFT JOIN users f ON f.id = m.from_user`

type zeile interface{ Scan(dest ...any) error }

func lesen(z zeile) (Brief, error) {
	var b Brief
	var von sql.NullInt64
	var offen sql.NullTime
	if err := z.Scan(&b.ID, &b.An, &b.AnName, &von, &b.VonName, &b.VonEmoji, &b.Text, &b.Bonus, &b.Erstellt, &offen); err != nil {
		return b, err
	}
	if von.Valid {
		v := int(von.Int64)
		b.Von = &v
	}
	if offen.Valid {
		t := offen.Time
		b.Geoeffnet = &t
	}
	return b, nil
}

func (s *Service) liste(wo string, args ...any) ([]Brief, error) {
	rows, err := s.db.Query(`SELECT `+spalten+quelle+` WHERE `+wo, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Brief{}
	for rows.Next() {
		b, err := lesen(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// List liefert die eigene Post (ungeöffnet zuerst, dann die letzten geöffneten)
// und, für Eltern, was sie geschrieben haben.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}
	eingang, err := s.liste(`m.to_user = ? ORDER BY m.opened_at IS NOT NULL, m.id DESC LIMIT 30`, me)
	if err != nil {
		log.Error().Err(err).Msg("Post konnte nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// Ungeöffnete Post verrät ihren Inhalt erst beim Öffnen: Sonst wäre das
	// Aufmachen nur noch ein Knopf.
	for i := range eingang {
		if eingang[i].Geoeffnet == nil {
			eingang[i].Text = ""
		}
	}
	antwort := map[string]any{"inbox": eingang}
	if rolle, _ := auth.GetUserRole(r); rolle == "admin" {
		gesendet, err := s.liste(`m.from_user = ? ORDER BY m.id DESC LIMIT 30`, me)
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		antwort["sent"] = gesendet
	}
	auth.WriteJSON(w, antwort)
}

// Create schreibt eine Nachricht. Nur Eltern.
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	if rolle, _ := auth.GetUserRole(r); !ok || rolle != "admin" {
		auth.HTTPError(w, http.StatusForbidden, "Post schreiben die Eltern")
		return
	}
	var req struct {
		An    int    `json:"to_user"`
		Text  string `json:"text"`
		Bonus int    `json:"bonus"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	switch {
	case req.Text == "":
		auth.HTTPError(w, http.StatusBadRequest, "Was möchtest du schreiben?")
		return
	case utf8.RuneCountInString(req.Text) > maxText:
		auth.HTTPError(w, http.StatusBadRequest, "Die Nachricht ist zu lang (höchstens 280 Zeichen)")
		return
	case req.Bonus < 0 || req.Bonus > maxBonus:
		auth.HTTPError(w, http.StatusBadRequest, "Der Bonus muss zwischen 0 und 100 Punkten liegen")
		return
	case req.An == me:
		auth.HTTPError(w, http.StatusBadRequest, "Post an dich selbst geht nicht")
		return
	}
	var da int
	if err := s.db.QueryRow(`SELECT 1 FROM users WHERE id = ?`, req.An).Scan(&da); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Unbekanntes Familienmitglied")
		return
	}
	var ungelesen int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM family_mail WHERE to_user = ? AND opened_at IS NULL`, req.An).Scan(&ungelesen)
	if ungelesen >= maxUngelesen {
		auth.HTTPError(w, http.StatusConflict, "Es liegt schon genug ungeöffnete Post bereit")
		return
	}
	res, err := s.db.Exec(`INSERT INTO family_mail (to_user, from_user, text, bonus) VALUES (?, ?, ?, ?)`,
		req.An, me, req.Text, req.Bonus)
	if err != nil {
		log.Error().Err(err).Msg("Post konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	id, _ := res.LastInsertId()
	b, err := lesen(s.db.QueryRow(`SELECT `+spalten+quelle+` WHERE m.id = ?`, id))
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, b)
}

func briefID(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

// Open öffnet eine Nachricht. Nur, wer sie bekommt; die Punkte gibt es einmal.
func (s *Service) Open(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	id, err := briefID(r)
	if !ok || err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var an, bonus int
	var offen sql.NullTime
	err = tx.QueryRow(`SELECT to_user, bonus, opened_at FROM family_mail WHERE id = ?`, id).Scan(&an, &bonus, &offen)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && an != me) {
		auth.HTTPError(w, http.StatusNotFound, "Nachricht nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	neu := false
	if !offen.Valid {
		res, err := tx.Exec(`UPDATE family_mail SET opened_at = ? WHERE id = ? AND opened_at IS NULL`, time.Now(), id)
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			neu = true
			if bonus > 0 {
				if err := points.Award(tx, me, Quelle, id, bonus, ""); err != nil {
					log.Error().Err(err).Msg("Bonus der Post konnte nicht gutgeschrieben werden")
					auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
					return
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	b, err := lesen(s.db.QueryRow(`SELECT `+spalten+quelle+` WHERE m.id = ?`, id))
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, map[string]any{"mail": b, "first_open": neu})
}

// Delete nimmt eine noch ungeöffnete Nachricht zurück. Nur der Absender.
func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	id, err := briefID(r)
	if !ok || err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	res, err := s.db.Exec(`DELETE FROM family_mail WHERE id = ? AND from_user = ? AND opened_at IS NULL`, id, me)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusConflict, "Diese Nachricht lässt sich nicht mehr zurücknehmen")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
