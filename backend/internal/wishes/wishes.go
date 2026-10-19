// Package wishes ist die Wunschliste der Kinder.
//
// Die Belohnungen legen die Eltern fest — „ein Eis, 80 Punkte". Kinder haben
// aber eigene Wünsche, die auf keiner Liste stehen: ins Kino gehen, ein
// bestimmtes Spiel, bei der Freundin übernachten. Hier trägt das Kind den
// Wunsch ein, die Eltern sagen, was er kostet (oder dass es ihn nicht gibt),
// und sobald das Guthaben reicht, löst das Kind ihn ein.
//
// Eingelöst wird über dieselbe Tabelle wie bei den Belohnungen. Der Wunsch
// erscheint dadurch bei den Eltern unter „Warten auf dich", das Guthaben sinkt
// sofort, und Ablehnen gibt die Punkte zurück — nichts davon ist hier noch
// einmal gebaut.
//
//	offen ──Preis──▶ bepreist ──Kind löst ein──▶ angefragt ──Eltern──▶ erfuellt
//	  │                 ▲                            │
//	  └──Eltern: nein──▶ abgelehnt                   └──abgelehnt/zurückgezogen──▶ bepreist
//
// Wünsche gehören dem Kind: Es sieht nur seine eigenen, die Eltern sehen alle,
// das Wandgerät keinen — ein Geburtstagswunsch gehört nicht in den Flur.
package wishes

import (
	"database/sql"
	"encoding/json"
	"errors"
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

const (
	StatusOffen     = "offen"
	StatusBepreist  = "bepreist"
	StatusAngefragt = "angefragt"
	StatusErfuellt  = "erfuellt"
	StatusAbgelehnt = "abgelehnt"

	maxCost    = 100000
	maxTitle   = 80
	maxNote    = 200
	maxOffene  = 25
	vorgabeSym = "⭐"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

type Wish struct {
	ID        int        `json:"id"`
	UserID    int        `json:"user_id"`
	UserName  string     `json:"user_name"`
	UserEmoji string     `json:"user_emoji"`
	Title     string     `json:"title"`
	Emoji     string     `json:"emoji"`
	Cost      *int       `json:"cost"`
	Status    string     `json:"status"`
	Note      string     `json:"note"`
	CreatedAt time.Time  `json:"created_at"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}

// istElternteil: Ein angemeldeter Administrator. Das Wandgerät zählt nicht —
// es ist niemand.
func istElternteil(r *http.Request) bool {
	rolle, ok := auth.GetUserRole(r)
	return ok && rolle == "admin" && !auth.IsDevice(r)
}

// ------------------------------------------------------------------ Lesen

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	userID, person := auth.GetUserID(r)
	if !person {
		// Das Wandgerät sieht keine Wünsche.
		auth.WriteJSON(w, []Wish{})
		return
	}

	query := `
		SELECT w.id, w.user_id, COALESCE(u.name, ''), COALESCE(u.avatar_emoji, ''),
		       w.title, w.emoji, w.cost, w.status, w.note, w.created_at, w.decided_at
		FROM wishes w LEFT JOIN users u ON u.id = w.user_id`
	var args []any
	if !istElternteil(r) {
		query += ` WHERE w.user_id = ?`
		args = append(args, userID)
	}
	// Was auf die Eltern wartet, steht oben, dann das Neueste.
	query += `
		ORDER BY CASE w.status
			WHEN 'offen' THEN 0 WHEN 'angefragt' THEN 1 WHEN 'bepreist' THEN 2
			WHEN 'abgelehnt' THEN 3 ELSE 4 END,
			w.created_at DESC, w.id DESC
		LIMIT 200`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Wünsche konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	out := []Wish{}
	for rows.Next() {
		var wi Wish
		var cost sql.NullInt64
		var decided sql.NullTime
		if err := rows.Scan(&wi.ID, &wi.UserID, &wi.UserName, &wi.UserEmoji, &wi.Title, &wi.Emoji,
			&cost, &wi.Status, &wi.Note, &wi.CreatedAt, &decided); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		if cost.Valid {
			c := int(cost.Int64)
			wi.Cost = &c
		}
		if decided.Valid {
			t := decided.Time
			wi.DecidedAt = &t
		}
		out = append(out, wi)
	}
	if err := rows.Err(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, out)
}

// ---------------------------------------------------------------- Eintragen

type createRequest struct {
	Title string `json:"title"`
	Emoji string `json:"emoji"`
	// Nur ein Elternteil darf für ein Kind eintragen.
	UserID int `json:"user_id"`
}

// bereinige prüft Titel und Symbol und sagt, was daran nicht stimmt.
func bereinige(title, emoji string) (string, string, string) {
	title = strings.TrimSpace(title)
	emoji = strings.TrimSpace(emoji)
	switch {
	case title == "":
		return "", "", "Was wünschst du dir?"
	case utf8.RuneCountInString(title) > maxTitle:
		return "", "", "Der Wunsch ist zu lang (höchstens 80 Zeichen)"
	}
	if emoji == "" {
		emoji = vorgabeSym
	}
	if utf8.RuneCountInString(emoji) > 8 {
		return "", "", "Bitte nur ein Symbol"
	}
	return title, emoji, ""
}

// Create trägt einen Wunsch ein: für sich selbst, ein Elternteil auch für ein
// Kind.
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusForbidden, "Zum Wünschen bitte anmelden")
		return
	}
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	title, emoji, msg := bereinige(req.Title, req.Emoji)
	if msg != "" {
		auth.HTTPError(w, http.StatusBadRequest, msg)
		return
	}

	owner := me
	if req.UserID > 0 && req.UserID != me {
		if !istElternteil(r) {
			auth.HTTPError(w, http.StatusForbidden, "Du kannst nur für dich selbst etwas wünschen")
			return
		}
		var da int
		if err := s.db.QueryRow(`SELECT 1 FROM users WHERE id = ?`, req.UserID).Scan(&da); err != nil {
			auth.HTTPError(w, http.StatusBadRequest, "Unbekanntes Familienmitglied")
			return
		}
		owner = req.UserID
	}

	var offene int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM wishes WHERE user_id = ? AND status IN ('offen', 'bepreist')`, owner,
	).Scan(&offene); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if offene >= maxOffene {
		auth.HTTPError(w, http.StatusConflict, "Die Wunschliste ist voll — erst ein paar Wünsche abarbeiten")
		return
	}

	res, err := s.db.Exec(
		`INSERT INTO wishes (user_id, title, emoji) VALUES (?, ?, ?)`, owner, title, emoji)
	if err != nil {
		log.Error().Err(err).Msg("Wunsch konnte nicht angelegt werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]any{"id": id, "title": title, "emoji": emoji, "status": StatusOffen})
}

// Withdraw nimmt einen eigenen Wunsch zurück. Ein Wunsch, den das Kind schon
// eingelöst hat, bleibt: Erst die Anfrage zurückziehen.
func (s *Service) Withdraw(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusForbidden, "Dafür bitte anmelden")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	res, err := s.db.Exec(
		`DELETE FROM wishes WHERE id = ? AND user_id = ? AND status != 'angefragt'`, id, me)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusConflict, "Dieser Wunsch lässt sich nicht mehr zurücknehmen")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ------------------------------------------------------- Entscheiden (Eltern)

type decideRequest struct {
	Title *string `json:"title"`
	Emoji *string `json:"emoji"`
	// Der Preis in Punkten. Fehlt er, bleibt der bisherige.
	Cost *int    `json:"cost"`
	Note *string `json:"note"`
	// reject: „Nein, den gibt es nicht." Der Hinweis sagt dem Kind, warum.
	Reject bool `json:"reject"`
}

// Decide setzt den Preis eines Wunsches — oder lehnt ihn ab. Nur Eltern.
func (s *Service) Decide(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var req decideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var title, emoji, note, status string
	var cost sql.NullInt64
	err = tx.QueryRow(`SELECT title, emoji, note, status, cost FROM wishes WHERE id = ?`, id).
		Scan(&title, &emoji, &note, &status, &cost)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Wunsch nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// Ab der Anfrage steht der Preis fest: Das Guthaben ist schon abgebucht.
	if status == StatusAngefragt || status == StatusErfuellt {
		auth.HTTPError(w, http.StatusConflict, "Dieser Wunsch ist schon eingelöst oder angefragt")
		return
	}

	if req.Title != nil || req.Emoji != nil {
		neuTitel, neuEmoji := title, emoji
		if req.Title != nil {
			neuTitel = *req.Title
		}
		if req.Emoji != nil {
			neuEmoji = *req.Emoji
		}
		var msg string
		title, emoji, msg = bereinige(neuTitel, neuEmoji)
		if msg != "" {
			auth.HTTPError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if req.Note != nil {
		note = strings.TrimSpace(*req.Note)
		if utf8.RuneCountInString(note) > maxNote {
			auth.HTTPError(w, http.StatusBadRequest, "Der Hinweis ist zu lang (höchstens 200 Zeichen)")
			return
		}
	}

	var neuerPreis any
	switch {
	case req.Reject:
		status = StatusAbgelehnt
		neuerPreis = nil
	default:
		preis := 0
		switch {
		case req.Cost != nil:
			preis = *req.Cost
		case cost.Valid:
			preis = int(cost.Int64)
		}
		switch {
		case preis <= 0:
			auth.HTTPError(w, http.StatusBadRequest, "Ein Wunsch kostet mindestens 1 Punkt")
			return
		case preis > maxCost:
			auth.HTTPError(w, http.StatusBadRequest, "So viele Punkte sammelt niemand")
			return
		}
		status = StatusBepreist
		neuerPreis = preis
	}

	if _, err := tx.Exec(`
		UPDATE wishes
		   SET title = ?, emoji = ?, note = ?, status = ?, cost = ?, decided_at = CURRENT_TIMESTAMP
		 WHERE id = ?`, title, emoji, note, status, neuerPreis, id); err != nil {
		log.Error().Err(err).Msg("Wunsch konnte nicht entschieden werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Remove löscht einen Wunsch (nur Eltern). Eine bereits gestellte Anfrage
// bleibt bei den Belohnungen stehen und wird dort entschieden.
func (s *Service) Remove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	if _, err := s.db.Exec(`DELETE FROM wishes WHERE id = ?`, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ------------------------------------------------------------- Einlösen

// Redeem löst einen bepreisten Wunsch ein. Das Guthaben sinkt jetzt, damit
// niemand denselben Wunsch und dieselben Punkte zweimal ausgibt; ein
// Elternteil bestätigt, wenn es so weit ist.
func (s *Service) Redeem(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusForbidden, "Zum Einlösen bitte anmelden")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var title, emoji, status string
	var cost sql.NullInt64
	err = tx.QueryRow(
		`SELECT title, emoji, status, cost FROM wishes WHERE id = ? AND user_id = ?`, id, me).
		Scan(&title, &emoji, &status, &cost)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Wunsch nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if status != StatusBepreist || !cost.Valid || cost.Int64 <= 0 {
		auth.HTTPError(w, http.StatusConflict, "Für diesen Wunsch haben die Eltern noch keinen Preis festgelegt")
		return
	}
	preis := int(cost.Int64)

	balance, err := points.Balance(tx, me)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if balance < preis {
		auth.HTTPError(w, http.StatusConflict,
			"Dafür fehlen noch "+strconv.Itoa(preis-balance)+" Punkte")
		return
	}

	res, err := tx.Exec(`
		INSERT INTO reward_redemptions (reward_id, user_id, title, emoji, cost, wish_id)
		VALUES (NULL, ?, ?, ?, ?, ?)`, me, title, emoji, preis, id)
	if err != nil {
		log.Error().Err(err).Msg("Wunsch konnte nicht eingelöst werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	redemptionID, _ := res.LastInsertId()
	if _, err := tx.Exec(
		`UPDATE wishes SET status = ?, redemption_id = ? WHERE id = ?`,
		StatusAngefragt, redemptionID, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]any{
		"id":      redemptionID,
		"title":   title,
		"cost":    preis,
		"balance": balance - preis,
	})
}
