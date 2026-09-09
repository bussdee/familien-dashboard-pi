// Package links stores each family member's bookmarks. A link belongs to the
// person who created it unless they mark it shared, and can be pinned to show
// up directly on the dashboard.
package links

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type Link struct {
	ID          int    `json:"id"`
	OwnerID     *int   `json:"owner_id"`
	OwnerName   string `json:"owner_name,omitempty"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Emoji       string `json:"emoji"`
	Pinned      bool   `json:"pinned"`
	Shared      bool   `json:"shared"`
	Position    int    `json:"position"`
	/** true when the caller may edit or delete it. */
	Editable bool `json:"editable"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const columns = `l.id, l.owner_id, l.title, l.url, l.description, l.category,
	l.emoji, l.pinned, l.shared, l.position, u.name`

// Suggested categories the UI offers; free text is allowed too.
var DefaultCategories = []string{
	"Schule", "Arbeit", "Freizeit", "Einkaufen", "Behörden", "Medien", "Sonstiges",
}

// visible returns everything the caller may see: their own links plus the ones
// shared with the family.
// visible liefert die Links, die jemand sehen darf: die eigenen und die
// geteilten. Mit userID = 0 (Wandgerät) bleiben nur die geteilten übrig —
// die gehören der ganzen Familie und dürfen im Flur hängen.
func (s *Service) visible(userID int, pinnedOnly bool) ([]Link, error) {
	query := `SELECT ` + columns + `
		FROM links l LEFT JOIN users u ON u.id = l.owner_id
		WHERE l.owner_id = ? OR l.shared = 1`
	args := []any{userID}
	if userID <= 0 {
		query = `SELECT ` + columns + `
			FROM links l LEFT JOIN users u ON u.id = l.owner_id
			WHERE l.shared = 1`
		args = nil
	}
	if pinnedOnly {
		query += ` AND l.pinned = 1`
	}
	query += ` ORDER BY l.position, l.title`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Link{}
	for rows.Next() {
		link, err := scan(rows)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan link")
			continue
		}
		link.Editable = link.OwnerID != nil && *link.OwnerID == userID
		result = append(result, link)
	}
	return result, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scan(row scanner) (Link, error) {
	var l Link
	var owner sql.NullInt64
	var ownerName sql.NullString
	if err := row.Scan(&l.ID, &owner, &l.Title, &l.URL, &l.Description, &l.Category,
		&l.Emoji, &l.Pinned, &l.Shared, &l.Position, &ownerName); err != nil {
		return Link{}, err
	}
	if owner.Valid {
		id := int(owner.Int64)
		l.OwnerID = &id
	}
	l.OwnerName = ownerName.String
	return l, nil
}

// ------------------------------------------------------------------ Handlers

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok && !auth.IsDevice(r) {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	pinnedOnly := r.URL.Query().Get("pinned") == "1"
	list, err := s.visible(userID, pinnedOnly)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list links")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	// Categories actually in use, so the UI can offer them without a second call.
	seen := map[string]bool{}
	categories := []string{}
	for _, link := range list {
		name := link.Category
		if name == "" {
			name = "Sonstiges"
		}
		if !seen[name] {
			seen[name] = true
			categories = append(categories, name)
		}
	}

	auth.WriteJSON(w, map[string]any{
		"links":      list,
		"categories": categories,
		"suggested":  DefaultCategories,
	})
}

type payload struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Emoji       string `json:"emoji"`
	Pinned      *bool  `json:"pinned"`
	Shared      *bool  `json:"shared"`
}

// normalise fixes the two things people get wrong when typing a bookmark:
// leaving off the scheme, and pasting with stray whitespace.
func (p *payload) normalise() error {
	p.Title = strings.TrimSpace(p.Title)
	p.URL = strings.TrimSpace(p.URL)
	p.Category = strings.TrimSpace(p.Category)
	p.Emoji = strings.TrimSpace(p.Emoji)

	if p.Title == "" {
		return fmt.Errorf("Titel fehlt")
	}
	if p.URL == "" {
		return fmt.Errorf("Adresse fehlt")
	}
	if !strings.Contains(p.URL, "://") {
		p.URL = "https://" + p.URL
	}

	parsed, err := url.Parse(p.URL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("Das sieht nicht nach einer Web-Adresse aus")
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("Nur http:// und https:// sind erlaubt")
	}

	if p.Emoji == "" {
		p.Emoji = "🔗"
	}
	if len([]rune(p.Emoji)) > 4 {
		return fmt.Errorf("Symbol darf höchstens 4 Zeichen haben")
	}
	if p.Category == "" {
		p.Category = "Sonstiges"
	}
	return nil
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var req payload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := req.normalise(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	var maxPos sql.NullInt64
	_ = s.db.QueryRow("SELECT MAX(position) FROM links WHERE owner_id = ?", userID).Scan(&maxPos)

	res, err := s.db.Exec(`
		INSERT INTO links (owner_id, title, url, description, category, emoji, pinned, shared, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, req.Title, req.URL, req.Description, req.Category, req.Emoji,
		boolOr(req.Pinned, false), boolOr(req.Shared, false), int(maxPos.Int64)+1)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create link")
		auth.HTTPError(w, http.StatusInternalServerError, "Link konnte nicht gespeichert werden")
		return
	}

	id, _ := res.LastInsertId()
	link, err := s.byID(int(id), userID)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, link)
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := s.ownedLink(w, r)
	if !ok {
		return
	}

	var req payload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := req.normalise(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	current, err := s.byID(id, userID)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Link nicht gefunden")
		return
	}

	if _, err := s.db.Exec(`
		UPDATE links SET title = ?, url = ?, description = ?, category = ?, emoji = ?,
			pinned = ?, shared = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		req.Title, req.URL, req.Description, req.Category, req.Emoji,
		boolOr(req.Pinned, current.Pinned), boolOr(req.Shared, current.Shared), id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Link konnte nicht gespeichert werden")
		return
	}

	link, err := s.byID(id, userID)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Link nicht gefunden")
		return
	}
	auth.WriteJSON(w, link)
}

// TogglePin is its own endpoint because pinning from the dashboard should not
// require sending the whole link back.
func (s *Service) TogglePin(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := s.ownedLink(w, r)
	if !ok {
		return
	}

	if _, err := s.db.Exec(
		"UPDATE links SET pinned = NOT pinned, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	link, err := s.byID(id, userID)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Link nicht gefunden")
		return
	}
	auth.WriteJSON(w, link)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	_, id, ok := s.ownedLink(w, r)
	if !ok {
		return
	}
	if _, err := s.db.Exec("DELETE FROM links WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) Reorder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var req struct {
		IDs []int `json:"ids"`
	}
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

	// Scoped to the caller so reordering cannot touch someone else's links.
	for position, id := range req.IDs {
		if _, err := tx.Exec(
			"UPDATE links SET position = ? WHERE id = ? AND owner_id = ?",
			position, id, userID); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ownedLink resolves the id and confirms the caller owns it. A shared link
// stays editable only by the person who created it.
func (s *Service) ownedLink(w http.ResponseWriter, r *http.Request) (userID, id int, ok bool) {
	userID, authed := auth.GetUserID(r)
	if !authed {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return 0, 0, false
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return 0, 0, false
	}

	var owner sql.NullInt64
	if err := s.db.QueryRow("SELECT owner_id FROM links WHERE id = ?", id).Scan(&owner); err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Link nicht gefunden")
		return 0, 0, false
	}
	if !owner.Valid || int(owner.Int64) != userID {
		auth.HTTPError(w, http.StatusForbidden, "Das ist der Link von jemand anderem")
		return 0, 0, false
	}
	return userID, id, true
}

func (s *Service) byID(id, userID int) (Link, error) {
	row := s.db.QueryRow(`SELECT `+columns+`
		FROM links l LEFT JOIN users u ON u.id = l.owner_id WHERE l.id = ?`, id)
	link, err := scan(row)
	if err != nil {
		return Link{}, err
	}
	link.Editable = link.OwnerID != nil && *link.OwnerID == userID
	return link, nil
}

func boolOr(value *bool, fallback bool) bool {
	if value != nil {
		return *value
	}
	return fallback
}
