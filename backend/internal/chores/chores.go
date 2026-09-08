package chores

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type Chore struct {
	ID           int        `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	IntervalDays int        `json:"interval_days"`
	Points       int        `json:"points"`
	Rotate       bool       `json:"rotate"`
	AssigneeID   *int       `json:"assignee_id"`
	LastDoneAt   *time.Time `json:"last_done_at,omitempty"`
	NextDueAt    *time.Time `json:"next_due_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	AssigneeName  string `json:"assignee_name"`
	AssigneeColor string `json:"assignee_color"`
	AssigneeEmoji string `json:"assignee_emoji"`
	IsOverdue     bool   `json:"is_overdue"`
	DaysUntilDue  int    `json:"days_until_due"`

	// IsDue entscheidet, ob die Oberfläche das Abhaken überhaupt anbietet.
	// LastDoneBy beantwortet die Frage, die dann sofort kommt: von wem denn?
	IsDue      bool   `json:"is_due"`
	LastDoneBy string `json:"last_done_by,omitempty"`
}

type CreateRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	IntervalDays int    `json:"interval_days"`
	Points       int    `json:"points"`
	Rotate       *bool  `json:"rotate"`
	AssigneeID   *int   `json:"assignee_id"`
}

type UpdateRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	IntervalDays *int    `json:"interval_days"`
	Points       *int    `json:"points"`
	Rotate       *bool   `json:"rotate"`
	AssigneeID   *int    `json:"assignee_id"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Start(ctx context.Context) {
	s.rotateDue()
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rotateDue()
		}
	}
}

// rotateDue hands a chore to the next person only once it is actually due and
// still unassigned or overdue. The previous version reassigned every chore
// every hour, so nobody ever kept one long enough to do it.
func (s *Service) rotateDue() {
	users, err := s.rotationPool()
	if err != nil || len(users) == 0 {
		return
	}

	// Die Fälligkeit wird bewusst in Go entschieden und nicht per SQL. In der
	// Datenbank stehen Zeitstempel historisch in zwei Textformaten — einmal
	// von Go geschrieben, einmal von SQLite selbst. Ein Vergleich mit
	// datetime('now') stellt die beiden Formate gegenüber und liefert Unsinn.
	rows, err := s.db.Query(`
		SELECT id, assignee_id, next_due_at FROM chores WHERE rotate = 1`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load chores for rotation")
		return
	}
	defer rows.Close()

	type pending struct {
		id      int
		current sql.NullInt64
	}
	now := time.Now()
	var todo []pending
	for rows.Next() {
		var p pending
		var due sql.NullTime
		if err := rows.Scan(&p.id, &p.current, &due); err != nil {
			continue
		}
		var faellig *time.Time
		if due.Valid {
			faellig = &due.Time
		}
		// Weitergereicht wird nur, was niemandem gehört oder wirklich ansteht.
		if p.current.Valid && !isDue(now, faellig) {
			continue
		}
		todo = append(todo, p)
	}
	if err := rows.Err(); err != nil {
		return
	}

	for _, p := range todo {
		next := users[0]
		if p.current.Valid {
			for i, uid := range users {
				if uid == int(p.current.Int64) {
					next = users[(i+1)%len(users)]
					break
				}
			}
			if next == int(p.current.Int64) && len(users) > 1 {
				continue // only one candidate matched; leave it alone
			}
		}
		if p.current.Valid && next == int(p.current.Int64) {
			continue
		}

		if _, err := s.db.Exec(
			"UPDATE chores SET assignee_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			next, p.id); err != nil {
			log.Error().Err(err).Int("chore_id", p.id).Msg("Failed to rotate chore")
		}
	}
}

// rotationPool prefers members; if the family has none (everyone is an admin)
// it falls back to all users so rotation still works.
func (s *Service) rotationPool() ([]int, error) {
	ids, err := s.userIDs("SELECT id FROM users WHERE role = 'member' ORDER BY id")
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}
	return s.userIDs("SELECT id FROM users ORDER BY id")
}

func (s *Service) userIDs(query string) ([]int, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ------------------------------------------------------------------ Handlers

const choreColumns = `c.id, c.title, c.description, c.interval_days, c.points, c.rotate,
	c.assignee_id, c.last_done_at, c.next_due_at, c.created_at, c.updated_at,
	u.name, u.color, u.avatar_emoji,
	(SELECT du.name FROM chore_completions cc JOIN users du ON du.id = cc.user_id
	  WHERE cc.chore_id = c.id ORDER BY cc.id DESC LIMIT 1)`

// Everyone sees the whole board — a family chore list is only motivating if you
// can see what the others still owe.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT ` + choreColumns + `
		FROM chores c LEFT JOIN users u ON c.assignee_id = u.id
		ORDER BY c.next_due_at IS NULL, c.next_due_at ASC`)
	if err != nil {
		log.Error().Err(err).Msg("DB error listing chores")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	now := time.Now()
	chores := []Chore{}
	for rows.Next() {
		c, err := scanChore(rows, now)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan chore")
			continue
		}
		chores = append(chores, c)
	}
	auth.WriteJSON(w, chores)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
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
	if req.IntervalDays <= 0 {
		req.IntervalDays = 7
	}
	if req.Points <= 0 {
		req.Points = 10
	}
	rotate := true
	if req.Rotate != nil {
		rotate = *req.Rotate
	}

	var assignee any
	if req.AssigneeID != nil && *req.AssigneeID > 0 {
		assignee = *req.AssigneeID
	}

	// Eine frisch angelegte Aufgabe steht heute an. Sie stattdessen erst in
	// einem Intervall fällig zu machen, hieße: anlegen und dann einen Tag
	// warten dürfen, bis man sie abhaken kann.
	nextDue := startOfDay(time.Now())
	res, err := s.db.Exec(
		`INSERT INTO chores (title, description, interval_days, points, rotate, assignee_id, next_due_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.Title, req.Description, req.IntervalDays, req.Points, rotate, assignee, nextDue)
	if err != nil {
		log.Error().Err(err).Msg("DB error creating chore")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	id, _ := res.LastInsertId()
	chore, err := s.byID(int(id))
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, chore)
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

	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []any{}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		sets = append(sets, "title = ?")
		args = append(args, strings.TrimSpace(*req.Title))
	}
	if req.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *req.Description)
	}
	if req.IntervalDays != nil && *req.IntervalDays > 0 {
		sets = append(sets, "interval_days = ?")
		args = append(args, *req.IntervalDays)
	}
	if req.Points != nil && *req.Points > 0 {
		sets = append(sets, "points = ?")
		args = append(args, *req.Points)
	}
	if req.Rotate != nil {
		sets = append(sets, "rotate = ?")
		args = append(args, *req.Rotate)
	}
	// assignee_id = 0 explicitly clears the assignment.
	if req.AssigneeID != nil {
		if *req.AssigneeID > 0 {
			sets = append(sets, "assignee_id = ?")
			args = append(args, *req.AssigneeID)
		} else {
			sets = append(sets, "assignee_id = NULL")
		}
	}
	args = append(args, id)

	if _, err := s.db.Exec("UPDATE chores SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		log.Error().Err(err).Msg("DB error updating chore")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	chore, err := s.byID(id)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Aufgabe nicht gefunden")
		return
	}
	auth.WriteJSON(w, chore)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	if _, err := s.db.Exec("DELETE FROM chores WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) Complete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	// interval_days was missing from this query before, so next_due_at was
	// always set to "now" and every chore stayed permanently overdue.
	var intervalDays, reward int
	var title string
	var assignee sql.NullInt64
	var due sql.NullTime
	err = s.db.QueryRow(
		"SELECT title, interval_days, points, assignee_id, next_due_at FROM chores WHERE id = ?", id,
	).Scan(&title, &intervalDays, &reward, &assignee, &due)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Aufgabe nicht gefunden")
		return
	}

	now := time.Now()

	// Ohne diese Prüfung konnte eine bereits erledigte Aufgabe von jedem
	// weiteren Familienmitglied noch einmal abgehakt werden — jedes Mal mit
	// vollen Punkten. Den Müll bringt man aber nur einmal raus.
	var faellig *time.Time
	if due.Valid {
		faellig = &due.Time
	}
	if !isDue(now, faellig) {
		auth.HTTPError(w, http.StatusConflict, s.bereitsErledigt(id, title, due.Time))
		return
	}

	// Anyone may help out, but the points go to whoever actually did it.
	nextDue := dueDate(now, intervalDays)

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO chore_completions (chore_id, user_id, completed_at, verified, points_awarded)
		 VALUES (?, ?, ?, 0, ?)`, id, userID, now, reward)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// The point event references the completion, not the chore, so revoking it
	// in the admin area can undo both halves of the action.
	completionID, _ := res.LastInsertId()
	if err := points.Award(tx, userID, points.SourceChore, int(completionID), reward, title); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if _, err := tx.Exec(
		`UPDATE chores SET last_done_at = ?, next_due_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		now, nextDue, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	auth.WriteJSON(w, map[string]any{
		"completed_at":   now,
		"next_due_at":    nextDue,
		"points_awarded": reward,
		"title":          title,
	})
}

// bereitsErledigt formuliert die Absage so, dass sie im Alltag weiterhilft:
// wer schon dran war und wann es wieder losgeht. Sortiert wird über die ID,
// nicht über completed_at — die Zeitstempel liegen historisch in zwei
// verschiedenen Textformaten in der Datenbank und sortieren nicht verlässlich.
func (s *Service) bereitsErledigt(choreID int, title string, due time.Time) string {
	var name string
	err := s.db.QueryRow(`
		SELECT u.name FROM chore_completions cc
		JOIN users u ON u.id = cc.user_id
		WHERE cc.chore_id = ?
		ORDER BY cc.id DESC LIMIT 1`, choreID).Scan(&name)

	wann := "am " + due.Format("02.01.2006")
	if tage := daysUntil(time.Now(), due); tage == 1 {
		wann = "morgen"
	}

	if err != nil || name == "" {
		return "„" + title + "\u201c ist noch nicht wieder dran — erst " + wann + "."
	}
	return "„" + title + "\u201c hat " + name + " schon erledigt. Wieder dran ist die Aufgabe " + wann + "."
}

func (s *Service) byID(id int) (Chore, error) {
	row := s.db.QueryRow(`SELECT `+choreColumns+`
		FROM chores c LEFT JOIN users u ON c.assignee_id = u.id WHERE c.id = ?`, id)
	return scanChore(row, time.Now())
}

type scanner interface {
	Scan(dest ...any) error
}

// scanChore handles the nullable LEFT JOIN columns. The old code scanned them
// into plain strings, so any unassigned chore silently vanished from the list.
func scanChore(row scanner, now time.Time) (Chore, error) {
	var c Chore
	var assignee sql.NullInt64
	var lastDone, nextDue sql.NullTime
	var name, color, emoji, doneBy sql.NullString

	if err := row.Scan(&c.ID, &c.Title, &c.Description, &c.IntervalDays, &c.Points, &c.Rotate,
		&assignee, &lastDone, &nextDue, &c.CreatedAt, &c.UpdatedAt,
		&name, &color, &emoji, &doneBy); err != nil {
		return Chore{}, err
	}

	if assignee.Valid {
		id := int(assignee.Int64)
		c.AssigneeID = &id
	}
	if lastDone.Valid {
		c.LastDoneAt = &lastDone.Time
	}
	if nextDue.Valid {
		c.NextDueAt = &nextDue.Time
		c.DaysUntilDue = daysUntil(now, nextDue.Time)
		// Überfällig ist eine Aufgabe erst ab dem Tag NACH dem Stichtag —
		// am Stichtag selbst hat man noch den ganzen Tag Zeit.
		c.IsOverdue = c.DaysUntilDue < 0
		c.IsDue = c.DaysUntilDue <= 0
	} else {
		c.IsDue = true
	}
	c.LastDoneBy = doneBy.String
	c.AssigneeName = name.String
	c.AssigneeColor = color.String
	c.AssigneeEmoji = emoji.String
	return c, nil
}
