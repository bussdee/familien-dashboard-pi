// Package rewards gibt den Punkten einen Sinn: Wer genug gesammelt hat,
// löst sie gegen etwas ein, das die Eltern festgelegt haben — Bildschirmzeit,
// ein Eis, den Film am Familienabend.
//
// Der Ablauf ist bewusst zweistufig. Ein Kind fragt an, ein Elternteil löst
// ein (oder lehnt ab). Sonst stünde „Ein Eis" als erledigt da, während die
// Gefriertruhe leer ist. Das Guthaben sinkt schon bei der Anfrage, damit
// niemand dasselbe Guthaben dreimal anfragt.
package rewards

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
	StatusEingelöst = "eingeloest"
	StatusAbgelehnt = "abgelehnt"

	maxCost = 100000
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

type Reward struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Emoji    string `json:"emoji"`
	Cost     int    `json:"cost"`
	Active   bool   `json:"active"`
	Position int    `json:"position"`
}

type Redemption struct {
	ID        int        `json:"id"`
	RewardID  *int       `json:"reward_id"`
	UserID    int        `json:"user_id"`
	UserName  string     `json:"user_name"`
	UserEmoji string     `json:"user_emoji"`
	Title     string     `json:"title"`
	Emoji     string     `json:"emoji"`
	Cost      int        `json:"cost"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}

// Overview ist alles, was die Belohnungsseite auf einmal braucht.
type Overview struct {
	Rewards []Reward `json:"rewards"`
	// Guthaben der angemeldeten Person; am Wandgerät nil — dort ist niemand.
	Balance *int `json:"balance"`
	// Für ein Elternteil alle offenen Anfragen der Familie, sonst die eigenen
	// der letzten Wochen.
	Redemptions []Redemption `json:"redemptions"`
}

// ------------------------------------------------------------------ Lesen

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	isAdmin := false
	if role, ok := auth.GetUserRole(r); ok && role == "admin" {
		isAdmin = true
	}

	// Eltern sehen auch abgeschaltete Belohnungen, um sie wieder
	// einzuschalten. Alle anderen nur, was man tatsächlich einlösen kann.
	query := `SELECT id, title, emoji, cost, active, position FROM rewards`
	if !isAdmin {
		query += ` WHERE active = 1`
	}
	query += ` ORDER BY position, cost, id`

	rows, err := s.db.Query(query)
	if err != nil {
		log.Error().Err(err).Msg("Belohnungen konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	out := Overview{Rewards: []Reward{}, Redemptions: []Redemption{}}
	for rows.Next() {
		var rw Reward
		if err := rows.Scan(&rw.ID, &rw.Title, &rw.Emoji, &rw.Cost, &rw.Active, &rw.Position); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		out.Rewards = append(out.Rewards, rw)
	}
	if err := rows.Err(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	userID, person := auth.GetUserID(r)
	if person {
		b, err := points.Balance(s.db, userID)
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		out.Balance = &b
	}

	switch {
	case isAdmin:
		// Offene Anfragen aller, dazu die letzten Entscheidungen, damit man
		// sieht, was zuletzt eingelöst wurde.
		out.Redemptions, err = s.redemptions(`
			WHERE rr.status = 'offen' OR rr.created_at >= datetime('now', '-14 days')`)
	case person:
		out.Redemptions, err = s.redemptions(`
			WHERE rr.user_id = ? AND (rr.status = 'offen' OR rr.created_at >= datetime('now', '-30 days'))`,
			userID)
	}
	if err != nil {
		log.Error().Err(err).Msg("Einlösungen konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	auth.WriteJSON(w, out)
}

func (s *Service) redemptions(where string, args ...any) ([]Redemption, error) {
	rows, err := s.db.Query(`
		SELECT rr.id, rr.reward_id, rr.user_id, COALESCE(u.name, ''), COALESCE(u.avatar_emoji, ''),
		       rr.title, rr.emoji, rr.cost, rr.status, rr.created_at, rr.decided_at
		FROM reward_redemptions rr
		LEFT JOIN users u ON u.id = rr.user_id
		`+where+`
		ORDER BY CASE rr.status WHEN 'offen' THEN 0 ELSE 1 END, rr.created_at DESC
		LIMIT 50`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Redemption{}
	for rows.Next() {
		var rd Redemption
		var rewardID sql.NullInt64
		var decided sql.NullTime
		if err := rows.Scan(&rd.ID, &rewardID, &rd.UserID, &rd.UserName, &rd.UserEmoji,
			&rd.Title, &rd.Emoji, &rd.Cost, &rd.Status, &rd.CreatedAt, &decided); err != nil {
			return nil, err
		}
		if rewardID.Valid {
			id := int(rewardID.Int64)
			rd.RewardID = &id
		}
		if decided.Valid {
			t := decided.Time
			rd.DecidedAt = &t
		}
		list = append(list, rd)
	}
	return list, rows.Err()
}

// ------------------------------------------------------- Verwalten (Admin)

type rewardPayload struct {
	Title  string `json:"title"`
	Emoji  string `json:"emoji"`
	Cost   int    `json:"cost"`
	Active *bool  `json:"active"`
}

// pruefe normalisiert eine Belohnung und sagt, was daran nicht stimmt.
func pruefe(p *rewardPayload) string {
	p.Title = strings.TrimSpace(p.Title)
	p.Emoji = strings.TrimSpace(p.Emoji)
	switch {
	case p.Title == "":
		return "Wofür gibt es die Belohnung?"
	case utf8.RuneCountInString(p.Title) > 80:
		return "Der Titel ist zu lang (höchstens 80 Zeichen)"
	case p.Cost <= 0:
		return "Eine Belohnung kostet mindestens 1 Punkt"
	case p.Cost > maxCost:
		return "So viele Punkte sammelt niemand"
	}
	if p.Emoji == "" {
		p.Emoji = "🎁"
	}
	if utf8.RuneCountInString(p.Emoji) > 8 {
		return "Bitte nur ein Symbol"
	}
	return ""
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	var p rewardPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if msg := pruefe(&p); msg != "" {
		auth.HTTPError(w, http.StatusBadRequest, msg)
		return
	}
	active := p.Active == nil || *p.Active

	res, err := s.db.Exec(`
		INSERT INTO rewards (title, emoji, cost, active, position)
		VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM rewards))`,
		p.Title, p.Emoji, p.Cost, active)
	if err != nil {
		log.Error().Err(err).Msg("Belohnung konnte nicht angelegt werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, Reward{ID: int(id), Title: p.Title, Emoji: p.Emoji, Cost: p.Cost, Active: active})
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var p rewardPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if msg := pruefe(&p); msg != "" {
		auth.HTTPError(w, http.StatusBadRequest, msg)
		return
	}

	sets := "title = ?, emoji = ?, cost = ?, updated_at = CURRENT_TIMESTAMP"
	args := []any{p.Title, p.Emoji, p.Cost}
	if p.Active != nil {
		sets += ", active = ?"
		args = append(args, *p.Active)
	}
	args = append(args, id)

	res, err := s.db.Exec("UPDATE rewards SET "+sets+" WHERE id = ?", args...)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusNotFound, "Belohnung nicht gefunden")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete entfernt eine Belohnung aus dem Angebot. Bereits angefragte oder
// eingelöste bleiben mit Titel und Preis stehen.
func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	if _, err := s.db.Exec("DELETE FROM rewards WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ------------------------------------------------------------- Einlösen

// Redeem fragt eine Belohnung an. Nur für eine angemeldete Person: Am
// Wandgerät könnte sonst jeder das Guthaben der Schwester ausgeben.
func (s *Service) Redeem(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
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

	var rw Reward
	err = tx.QueryRow(`SELECT id, title, emoji, cost, active FROM rewards WHERE id = ?`, id).
		Scan(&rw.ID, &rw.Title, &rw.Emoji, &rw.Cost, &rw.Active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !rw.Active) {
		auth.HTTPError(w, http.StatusNotFound, "Diese Belohnung gibt es nicht mehr")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	balance, err := points.Balance(tx, userID)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if balance < rw.Cost {
		auth.HTTPError(w, http.StatusConflict,
			"Dafür fehlen noch "+strconv.Itoa(rw.Cost-balance)+" Punkte")
		return
	}

	res, err := tx.Exec(`
		INSERT INTO reward_redemptions (reward_id, user_id, title, emoji, cost)
		VALUES (?, ?, ?, ?, ?)`, rw.ID, userID, rw.Title, rw.Emoji, rw.Cost)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	newID, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]any{
		"id":      newID,
		"title":   rw.Title,
		"cost":    rw.Cost,
		"balance": balance - rw.Cost,
	})
}

// Decide löst eine Anfrage ein oder lehnt sie ab (nur Eltern). Abgelehnt
// heisst: Die Punkte stehen wieder zur Verfügung.
func (s *Service) Decide(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if req.Status != StatusEingelöst && req.Status != StatusAbgelehnt {
		auth.HTTPError(w, http.StatusBadRequest, "Unbekannte Entscheidung")
		return
	}

	res, err := s.db.Exec(`
		UPDATE reward_redemptions SET status = ?, decided_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'offen'`, req.Status, id)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusConflict, "Diese Anfrage ist schon entschieden")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Cancel zieht die eigene, noch offene Anfrage zurück — verklickt, oder
// doch lieber auf den Film sparen.
func (s *Service) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
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
		`DELETE FROM reward_redemptions WHERE id = ? AND user_id = ? AND status = 'offen'`,
		id, userID)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusConflict, "Diese Anfrage lässt sich nicht mehr zurückziehen")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
