package notes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type Note struct {
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Tags       []string  `json:"tags"`
	Pinned     bool      `json:"pinned"`
	OwnerID    *int      `json:"owner_id"`
	SourceFile string    `json:"source_file,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Pinned  bool     `json:"pinned"`
	Shared  bool     `json:"shared"`
}

type UpdateRequest struct {
	Title   *string   `json:"title"`
	Content *string   `json:"content"`
	Tags    *[]string `json:"tags"`
	Pinned  *bool     `json:"pinned"`
}

type Service struct {
	dir   string
	watch bool
	db    *sql.DB

	// syncMu serialises file<->DB reconciliation against handler writes, so a
	// save and the watcher cannot fight over the same note.
	syncMu sync.Mutex
}

func NewService(db *sql.DB, dir string, watch bool) *Service {
	return &Service{dir: dir, watch: watch, db: db}
}

func (s *Service) Start(ctx context.Context) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		log.Error().Err(err).Str("dir", s.dir).Msg("Cannot create notes dir")
		return
	}
	s.syncFromFiles()

	if s.watch {
		go s.watchFiles(ctx)
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncFromFiles()
		}
	}
}

func (s *Service) watchFiles(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create notes watcher")
		return
	}
	defer watcher.Close()

	if err := watcher.Add(s.dir); err != nil {
		log.Error().Err(err).Msg("Failed to watch notes dir")
		return
	}

	var debounce *time.Timer
	defer func() {
		if debounce != nil {
			debounce.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(strings.ToLower(event.Name), ".md") {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(500*time.Millisecond, s.syncFromFiles)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("Notes watcher error")
		}
	}
}

// syncFromFiles reconciles *.md on disk with the notes table. Files are matched
// by filename (source_file), not by title, so renaming a note's title no longer
// creates a duplicate row.
func (s *Service) syncFromFiles() {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}

	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		seen[entry.Name()] = struct{}{}
		s.upsertFromFile(entry.Name())
	}

	// A file-backed note whose file is gone should disappear too.
	rows, err := s.db.Query("SELECT id, source_file FROM notes WHERE source_file IS NOT NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	var orphans []int
	for rows.Next() {
		var id int
		var file string
		if err := rows.Scan(&id, &file); err != nil {
			continue
		}
		if _, ok := seen[file]; !ok {
			orphans = append(orphans, id)
		}
	}
	for _, id := range orphans {
		if _, err := s.db.Exec("DELETE FROM notes WHERE id = ?", id); err == nil {
			log.Debug().Int("id", id).Msg("Removed note whose file disappeared")
		}
	}
}

func (s *Service) upsertFromFile(filename string) {
	raw, err := os.ReadFile(filepath.Join(s.dir, filename))
	if err != nil {
		return
	}
	title, tags, pinned, body := parseMarkdown(string(raw))
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	_, err = s.db.Exec(`
		INSERT INTO notes (title, content, tags, pinned, owner_id, source_file)
		VALUES (?, ?, ?, ?, NULL, ?)
		ON CONFLICT(source_file) WHERE source_file IS NOT NULL DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			tags = excluded.tags,
			pinned = excluded.pinned,
			updated_at = CURRENT_TIMESTAMP`,
		title, body, strings.Join(tags, ","), pinned, filename)
	if err != nil {
		log.Error().Err(err).Str("file", filename).Msg("Failed to sync note")
	}
}

// parseMarkdown reads the small YAML-ish frontmatter block the dashboard writes.
func parseMarkdown(content string) (title string, tags []string, pinned bool, body string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return firstHeading(content), nil, false, content
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return firstHeading(content), nil, false, content
	}

	for _, line := range lines[1:end] {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "title":
			title = strings.Trim(value, `"'`)
		case "tags":
			for _, t := range strings.Split(strings.Trim(value, "[]"), ",") {
				if t = strings.TrimSpace(strings.Trim(t, `"'`)); t != "" {
					tags = append(tags, t)
				}
			}
		case "pinned":
			pinned = value == "true"
		}
	}

	body = strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")
	if title == "" {
		title = firstHeading(body)
	}
	return title, tags, pinned, body
}

func firstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.TrimSpace(strings.TrimLeft(line, "# "))
	}
	return ""
}

var unsafeFilename = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

// safeFilename derives a filesystem-safe name from a title. Without this a note
// titled "../../etc/passwd" would escape the notes directory.
func safeFilename(title string, id int) string {
	slug := unsafeFilename.ReplaceAllString(strings.TrimSpace(title), "_")
	slug = strings.Trim(slug, "_")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	if slug == "" {
		slug = "notiz"
	}
	return fmt.Sprintf("%s-%d.md", slug, id)
}

// ------------------------------------------------------------------ Handlers

const noteColumns = `id, title, content, tags, pinned, owner_id, source_file, created_at, updated_at`

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok && !auth.IsDevice(r) {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	// Shared notes (owner_id IS NULL) plus the caller's own; admins see all.
	query := `SELECT ` + noteColumns + ` FROM notes
	          WHERE owner_id IS NULL OR owner_id = ?
	          ORDER BY pinned DESC, updated_at DESC`
	args := []any{userID}

	// Am Wandgerät gibt es kein "eigenes": dort erscheinen nur die Notizen,
	// die der ganzen Familie gehören.
	if !ok {
		query = `SELECT ` + noteColumns + ` FROM notes
		         WHERE owner_id IS NULL
		         ORDER BY pinned DESC, updated_at DESC`
		args = nil
	}

	if role, _ := auth.GetUserRole(r); role == "admin" {
		query = `SELECT ` + noteColumns + ` FROM notes ORDER BY pinned DESC, updated_at DESC`
		args = nil
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Error().Err(err).Msg("DB error listing notes")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	notes := []Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan note")
			continue
		}
		notes = append(notes, n)
	}
	auth.WriteJSON(w, notes)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok && !auth.IsDevice(r) {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		auth.HTTPError(w, http.StatusBadRequest, "Titel erforderlich")
		return
	}

	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	// Ohne angemeldete Person bleibt die Notiz ohne Besitzer und damit für
	// alle sichtbar — am Wandgerät ist das genau richtig.
	var owner any
	if ok {
		owner = userID
	}
	if req.Shared {
		owner = nil
	}

	res, err := s.db.Exec(
		`INSERT INTO notes (title, content, tags, pinned, owner_id) VALUES (?, ?, ?, ?, ?)`,
		req.Title, req.Content, strings.Join(req.Tags, ","), req.Pinned, owner)
	if err != nil {
		log.Error().Err(err).Msg("DB error creating note")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	id, _ := res.LastInsertId()
	filename := safeFilename(req.Title, int(id))
	if _, err := s.db.Exec("UPDATE notes SET source_file = ? WHERE id = ?", filename, id); err != nil {
		log.Error().Err(err).Msg("Failed to attach note file")
	}

	note, err := s.byID(int(id))
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	s.writeFile(note)

	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, note)
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	existing, err := s.byID(id)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Notiz nicht gefunden")
		return
	}
	if !s.mayEdit(r, existing) {
		auth.HTTPError(w, http.StatusForbidden, "Keine Berechtigung für diese Notiz")
		return
	}

	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []any{}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		sets = append(sets, "title = ?")
		args = append(args, strings.TrimSpace(*req.Title))
	}
	if req.Content != nil {
		sets = append(sets, "content = ?")
		args = append(args, *req.Content)
	}
	if req.Tags != nil {
		sets = append(sets, "tags = ?")
		args = append(args, strings.Join(*req.Tags, ","))
	}
	if req.Pinned != nil {
		sets = append(sets, "pinned = ?")
		args = append(args, *req.Pinned)
	}
	args = append(args, id)

	if _, err := s.db.Exec("UPDATE notes SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		log.Error().Err(err).Msg("DB error updating note")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	note, err := s.byID(id)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Notiz nicht gefunden")
		return
	}
	s.writeFile(note)
	auth.WriteJSON(w, note)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	note, err := s.byID(id)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Notiz nicht gefunden")
		return
	}
	if !s.mayEdit(r, note) {
		auth.HTTPError(w, http.StatusForbidden, "Keine Berechtigung für diese Notiz")
		return
	}

	if _, err := s.db.Exec("DELETE FROM notes WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// Remove the file too, otherwise the next sync resurrects the note.
	if note.SourceFile != "" {
		if err := os.Remove(filepath.Join(s.dir, filepath.Base(note.SourceFile))); err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to delete note file")
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) mayEdit(r *http.Request, note Note) bool {
	if role, _ := auth.GetUserRole(r); role == "admin" {
		return true
	}
	if note.OwnerID == nil {
		return true // shared note
	}
	userID, ok := auth.GetUserID(r)
	return ok && *note.OwnerID == userID
}

func (s *Service) writeFile(note Note) {
	if note.SourceFile == "" {
		return
	}
	path := filepath.Join(s.dir, filepath.Base(note.SourceFile))
	content := fmt.Sprintf("---\ntitle: %s\ntags: [%s]\npinned: %v\n---\n\n%s\n",
		note.Title, strings.Join(note.Tags, ", "), note.Pinned, note.Content)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		log.Error().Err(err).Str("file", path).Msg("Failed to write note file")
	}
}

func (s *Service) byID(id int) (Note, error) {
	return scanNote(s.db.QueryRow(`SELECT `+noteColumns+` FROM notes WHERE id = ?`, id))
}

type scanner interface {
	Scan(dest ...any) error
}

func scanNote(row scanner) (Note, error) {
	var n Note
	var tags string
	var owner sql.NullInt64
	var sourceFile sql.NullString

	if err := row.Scan(&n.ID, &n.Title, &n.Content, &tags, &n.Pinned,
		&owner, &sourceFile, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return Note{}, err
	}
	if owner.Valid {
		id := int(owner.Int64)
		n.OwnerID = &id
	}
	n.SourceFile = sourceFile.String
	n.Tags = []string{}
	if tags != "" {
		n.Tags = strings.Split(tags, ",")
	}
	return n, nil
}
