package chores

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Wer ist für eine Aufgabe zuständig? Die Reihum-Verteilung ist praktisch,
// passt aber nicht überall: Wer wenig Zeit hat, steht sonst ständig im Plan.
const (
	AssignRotate   = "rotate"   // reihum an die Familie
	AssignPerson   = "person"   // eine feste Person
	AssignEveryone = "everyone" // alle gemeinsam
	AssignNobody   = "nobody"   // niemand fest, wer mag
)

func gueltigeZuweisung(v string) bool {
	switch v {
	case AssignRotate, AssignPerson, AssignEveryone, AssignNobody:
		return true
	}
	return false
}

type Chore struct {
	ID           int        `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	IntervalDays int        `json:"interval_days"`
	Points       int        `json:"points"`
	Rotate       bool       `json:"rotate"`
	Assignment   string     `json:"assignment"`
	AssigneeID   *int       `json:"assignee_id"`
	LastDoneAt   *time.Time `json:"last_done_at,omitempty"`
	NextDueAt    *time.Time `json:"next_due_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// OneOff: eine Aufgabe, die genau einmal ansteht — Keller aufräumen,
	// Fahrrad zur Werkstatt. Sie kommt nach dem Abhaken nicht wieder.
	OneOff bool `json:"one_off"`
	// Erledigt ist nur für einmalige Aufgaben interessant: Sie bleiben in
	// der Liste stehen, durchgestrichen, bis jemand aufräumt. Wer sie sofort
	// verschwinden liesse, könnte am Monatsende nicht mehr nachsehen, was
	// eigentlich alles geschafft wurde.
	Erledigt bool `json:"done"`

	AssigneeName  string `json:"assignee_name"`
	AssigneeColor string `json:"assignee_color"`
	AssigneeEmoji string `json:"assignee_emoji"`
	IsOverdue     bool   `json:"is_overdue"`
	DaysUntilDue  int    `json:"days_until_due"`

	// IsDue entscheidet, ob die Oberfläche das Abhaken überhaupt anbietet.
	// LastDoneBy beantwortet die Frage, die dann sofort kommt: von wem denn?
	IsDue      bool   `json:"is_due"`
	LastDoneBy string `json:"last_done_by,omitempty"`

	// NeedsCheck: Ein Kind bekommt die Punkte erst, wenn ein Elternteil
	// nachgesehen hat. PendingCheck: Die letzte Erledigung wartet genau
	// darauf.
	NeedsCheck   bool `json:"needs_check"`
	PendingCheck bool `json:"pending_check"`
}

type CreateRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	IntervalDays int    `json:"interval_days"`
	Points       int    `json:"points"`
	Rotate       *bool  `json:"rotate"`
	Assignment   string `json:"assignment"`
	AssigneeID   *int   `json:"assignee_id"`
	OneOff       bool   `json:"one_off"`
	NeedsCheck   bool   `json:"needs_check"`
}

type UpdateRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	IntervalDays *int    `json:"interval_days"`
	Points       *int    `json:"points"`
	Rotate       *bool   `json:"rotate"`
	Assignment   *string `json:"assignment"`
	AssigneeID   *int    `json:"assignee_id"`
	OneOff       *bool   `json:"one_off"`
	NeedsCheck   *bool   `json:"needs_check"`
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
	//
	// Erledigte einmalige Aufgaben bleiben aussen vor. Sie haben keinen
	// Stichtag mehr, und „kein Stichtag" gilt hier sonst als fällig — die
	// Aufgabe würde bis in alle Ewigkeit weitergereicht.
	rows, err := s.db.Query(`
		SELECT id, assignee_id, next_due_at FROM chores
		WHERE assignment = 'rotate' AND NOT (one_off = 1 AND last_done_at IS NOT NULL)`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load chores for rotation")
		return
	}
	defer rows.Close()

	type pending struct {
		id      int
		current sql.NullInt64
	}
	// Wer noch mitmacht — nachschlagbar, ohne die Liste jedes Mal zu durchlaufen.
	imKreis := make(map[int]bool, len(users))
	for _, uid := range users {
		imKreis[uid] = true
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
		// Ausnahme: Wer aus der Reihum-Verteilung genommen wurde, gibt die
		// Aufgabe sofort ab. Sonst stünde die Person noch bis zur nächsten
		// Fälligkeit im Plan — also genau so lange, wie es stört.
		if p.current.Valid && imKreis[int(p.current.Int64)] && !isDue(now, faellig) {
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
//
// Ausgenommen ist, wer den Schalter "nimmt an der Reihum-Verteilung teil"
// abgewählt hat. Nimmt niemand teil, bleibt die Liste leer und es wird gar
// nicht verteilt — das ist gewollt und keine Notlage, die einen Ersatz braucht.
func (s *Service) rotationPool() ([]int, error) {
	ids, err := s.userIDs(
		"SELECT id FROM users WHERE role = 'member' AND in_rotation = 1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}
	return s.userIDs("SELECT id FROM users WHERE in_rotation = 1 ORDER BY id")
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
	c.assignment, c.assignee_id, c.last_done_at, c.next_due_at, c.created_at, c.updated_at,
	c.one_off, u.name, u.color, u.avatar_emoji,
	(SELECT du.name FROM chore_completions cc JOIN users du ON du.id = cc.user_id
	  WHERE cc.chore_id = c.id ORDER BY cc.id DESC LIMIT 1),
	c.needs_check,
	COALESCE((SELECT cc.pending FROM chore_completions cc
	  WHERE cc.chore_id = c.id ORDER BY cc.id DESC LIMIT 1), 0)`

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
	// Zuweisung bestimmen. Ältere Fassungen kannten nur "rotate" als
	// Ja/Nein — die werden weiter verstanden, damit nichts bricht.
	zuweisung := req.Assignment
	if !gueltigeZuweisung(zuweisung) {
		switch {
		case req.Rotate != nil && !*req.Rotate && req.AssigneeID != nil && *req.AssigneeID > 0:
			zuweisung = AssignPerson
		case req.Rotate != nil && !*req.Rotate:
			zuweisung = AssignNobody
		default:
			zuweisung = AssignRotate
		}
	}
	rotate := zuweisung == AssignRotate

	// Nur eine feste Person trägt einen Namen. Bei "alle", "niemand" und
	// "reihum" bleibt das Feld leer — reihum füllt es der Hintergrundlauf.
	var assignee any
	if zuweisung == AssignPerson && req.AssigneeID != nil && *req.AssigneeID > 0 {
		assignee = *req.AssigneeID
	}

	// Eine frisch angelegte Aufgabe steht heute an. Sie stattdessen erst in
	// einem Intervall fällig zu machen, hieße: anlegen und dann einen Tag
	// warten dürfen, bis man sie abhaken kann.
	nextDue := startOfDay(time.Now())
	res, err := s.db.Exec(
		`INSERT INTO chores (title, description, interval_days, points, rotate, assignment, assignee_id, next_due_at, one_off, needs_check)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Title, req.Description, req.IntervalDays, req.Points, rotate, zuweisung, assignee, nextDue,
		req.OneOff, req.NeedsCheck)
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
	if req.Assignment != nil {
		if !gueltigeZuweisung(*req.Assignment) {
			auth.HTTPError(w, http.StatusBadRequest, "Unbekannte Zuständigkeit")
			return
		}
		sets = append(sets, "assignment = ?", "rotate = ?")
		args = append(args, *req.Assignment, *req.Assignment == AssignRotate)
		// Bei "alle" und "niemand" gehört kein Name mehr an die Aufgabe,
		// sonst bliebe der alte stehen und stiftet Verwirrung.
		if *req.Assignment == AssignEveryone || *req.Assignment == AssignNobody {
			sets = append(sets, "assignee_id = NULL")
		}
	}
	if req.OneOff != nil {
		sets = append(sets, "one_off = ?")
		args = append(args, *req.OneOff)
		// Aus einmalig wird wiederkehrend: Die Aufgabe braucht wieder einen
		// Stichtag, sonst stünde sie ohne einen da und gälte als nie erledigt.
		if !*req.OneOff {
			sets = append(sets, "next_due_at = COALESCE(next_due_at, ?)")
			args = append(args, startOfDay(time.Now()))
		}
	}
	if req.NeedsCheck != nil {
		sets = append(sets, "needs_check = ?")
		args = append(args, *req.NeedsCheck)
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

// DeleteDone räumt die erledigten einmaligen Aufgaben weg.
//
// Sie bleiben bewusst durchgestrichen stehen, bis jemand aufräumt: Am
// Monatsende will man sehen, was tatsächlich geschafft wurde. Ist der Monat
// abgeschlossen und die Rangliste festgeschrieben, kann die Liste wieder
// leer werden — ohne dass jemand zwanzig Häkchen einzeln löschen muss.
//
// Wiederkehrende Aufgaben sind nie betroffen. „Erledigt" heisst bei ihnen
// nur „heute schon gemacht".
func (s *Service) DeleteDone(w http.ResponseWriter, r *http.Request) {
	res, err := s.db.Exec(
		"DELETE FROM chores WHERE one_off = 1 AND last_done_at IS NOT NULL")
	if err != nil {
		log.Error().Err(err).Msg("DB error clearing finished one-off chores")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	entfernt, _ := res.RowsAffected()
	auth.WriteJSON(w, map[string]any{"deleted": entfernt})
}

// completeRequest trägt am Wandgerät die Person, die gerade abgehakt hat.
// Angemeldete Personen brauchen das Feld nicht — für sie zählt ihre Sitzung.
type completeRequest struct {
	UserID int `json:"user_id"`
}

func (s *Service) Complete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		if !auth.IsDevice(r) {
			auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
			return
		}
		// Am Wandtablet ist niemand angemeldet. Die Oberfläche fragt deshalb
		// "Wer war das?" und schickt die Antwort mit — sonst bekäme immer
		// derjenige die Punkte, der sich zuletzt angemeldet hat.
		var req completeRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.UserID <= 0 {
			auth.HTTPError(w, http.StatusBadRequest, "Wer hat das erledigt?")
			return
		}
		var da int
		if err := s.db.QueryRow("SELECT 1 FROM users WHERE id = ?", req.UserID).Scan(&da); err != nil {
			auth.HTTPError(w, http.StatusBadRequest, "Unbekanntes Familienmitglied")
			return
		}
		userID = req.UserID
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
	var due, lastDone sql.NullTime
	var oneOff, needsCheck bool
	err = s.db.QueryRow(
		`SELECT title, interval_days, points, assignee_id, next_due_at, last_done_at, one_off, needs_check
		 FROM chores WHERE id = ?`, id,
	).Scan(&title, &intervalDays, &reward, &assignee, &due, &lastDone, &oneOff, &needsCheck)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Aufgabe nicht gefunden")
		return
	}

	now := time.Now()

	// Eine einmalige Aufgabe lässt sich genau einmal abhaken. Ohne diese
	// Prüfung wäre sie danach stichtagslos — und stichtagslos heisst sonst
	// „noch nie erledigt", also beliebig oft Punkte für dieselbe Sache.
	if oneOff && lastDone.Valid {
		auth.HTTPError(w, http.StatusConflict,
			"\u201e"+title+"\u201c ist bereits erledigt.")
		return
	}

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
	// Eine einmalige Aufgabe bekommt keinen neuen Stichtag — sie ist fertig.
	var nextDue any
	if !oneOff {
		nextDue = dueDate(now, intervalDays)
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Muss ein Elternteil nachsehen? Nur, wenn die Aufgabe so markiert ist
	// und nicht gerade ein Elternteil selbst abhakt — sich selbst zu
	// bestätigen wäre ein Umweg ohne Sinn.
	var rolle string
	// Über tx, nicht s.db: Die Datenbank hat genau eine Verbindung, und die
	// hält gerade diese Transaktion. Eine Abfrage daneben wartete ewig.
	_ = tx.QueryRow(`SELECT role FROM users WHERE id = ?`, userID).Scan(&rolle)
	wartet := needsCheck && rolle != "admin"

	res, err := tx.Exec(
		`INSERT INTO chore_completions (chore_id, user_id, completed_at, verified, points_awarded, pending)
		 VALUES (?, ?, ?, ?, ?, ?)`, id, userID, now, !wartet, reward, wartet)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// The point event references the completion, not the chore, so revoking it
	// in the admin area can undo both halves of the action.
	//
	// Wartet die Erledigung auf ein Elternteil, gibt es noch keinen
	// Punkteeintrag: Er entsteht erst beim Bestätigen.
	completionID, _ := res.LastInsertId()
	if !wartet {
		if err := points.Award(tx, userID, points.SourceChore, int(completionID), reward, title); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
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

	vergeben := reward
	if wartet {
		vergeben = 0
	}
	auth.WriteJSON(w, map[string]any{
		"completed_at":   now,
		"next_due_at":    nextDue,
		"points_awarded": vergeben,
		"points_pending": reward - vergeben,
		"pending":        wartet,
		"title":          title,
	})
}

// ---------------------------------------------------------- Bestätigen

// Pending ist eine Erledigung, die auf ein Elternteil wartet.
type Pending struct {
	CompletionID int       `json:"completion_id"`
	ChoreID      int       `json:"chore_id"`
	Title        string    `json:"title"`
	UserID       int       `json:"user_id"`
	UserName     string    `json:"user_name"`
	UserEmoji    string    `json:"user_emoji"`
	Points       int       `json:"points"`
	CompletedAt  time.Time `json:"completed_at"`
}

// ListPending zeigt, was auf Bestätigung wartet. Lesen darf jeder, auch das
// Wandgerät: „Lena hat das Zimmer aufgeräumt — wartet auf Mama" ist dort genau
// die richtige Nachricht. Bestätigen dürfen nur Eltern.
func (s *Service) ListPending(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT cc.id, cc.chore_id, c.title, cc.user_id, u.name, u.avatar_emoji,
		       cc.points_awarded, cc.completed_at
		FROM chore_completions cc
		JOIN chores c ON c.id = cc.chore_id
		JOIN users u ON u.id = cc.user_id
		WHERE cc.pending = 1
		ORDER BY cc.id`)
	if err != nil {
		log.Error().Err(err).Msg("Ausstehende Aufgaben konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	list := []Pending{}
	for rows.Next() {
		var p Pending
		if err := rows.Scan(&p.CompletionID, &p.ChoreID, &p.Title, &p.UserID, &p.UserName,
			&p.UserEmoji, &p.Points, &p.CompletedAt); err != nil {
			continue
		}
		list = append(list, p)
	}
	auth.WriteJSON(w, list)
}

// Approve bestätigt eine Erledigung: Jetzt gibt es die Punkte. Der Eintrag
// bekommt den Zeitpunkt der Erledigung, nicht den der Bestätigung — sonst
// risse die Serie eines Kindes, nur weil Mama erst am nächsten Morgen
// nachgesehen hat. Liegt die Erledigung in einem schon abgeschlossenen Monat,
// zählt sie zum laufenden: Ein festgeschriebener Monat ändert sich nicht mehr.
func (s *Service) Approve(w http.ResponseWriter, r *http.Request) {
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

	var userID, reward int
	var title string
	var done time.Time
	err = tx.QueryRow(`
		SELECT cc.user_id, cc.points_awarded, c.title, cc.completed_at
		FROM chore_completions cc JOIN chores c ON c.id = cc.chore_id
		WHERE cc.id = ? AND cc.pending = 1`, id).Scan(&userID, &reward, &title, &done)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusConflict, "Diese Aufgabe ist schon entschieden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	wann := done
	if done.Format("2006-01") != time.Now().Format("2006-01") {
		wann = time.Now()
	}
	if _, err := tx.Exec(`
		INSERT INTO point_events (user_id, source, reference_id, points, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, points.SourceChore, id, reward, title, wann.UTC().Format("2006-01-02 15:04:05")); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if _, err := tx.Exec(
		`UPDATE chore_completions SET pending = 0, verified = 1 WHERE id = ?`, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, map[string]any{"user_id": userID, "points_awarded": reward, "title": title})
}

// Reject lehnt ab: noch nicht richtig gemacht. Die Erledigung verschwindet,
// die Aufgabe ist wieder fällig, Punkte gab es keine.
func (s *Service) Reject(w http.ResponseWriter, r *http.Request) {
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

	var offen int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM chore_completions WHERE id = ? AND pending = 1`, id).Scan(&offen); err != nil || offen == 0 {
		auth.HTTPError(w, http.StatusConflict, "Diese Aufgabe ist schon entschieden")
		return
	}
	if err := points.RevokeCompletion(tx, id); err != nil {
		log.Error().Err(err).Msg("Ablehnen fehlgeschlagen")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		&c.Assignment, &assignee, &lastDone, &nextDue, &c.CreatedAt, &c.UpdatedAt,
		&c.OneOff, &name, &color, &emoji, &doneBy, &c.NeedsCheck, &c.PendingCheck); err != nil {
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
	// Eine einmalige Aufgabe ist nach dem Abhaken fertig — für immer. Ohne
	// Stichtag stünde sie sonst wieder als offen da, weil „kein Stichtag"
	// sonst „noch nie erledigt" heisst.
	if c.OneOff && lastDone.Valid {
		c.Erledigt = true
		c.IsDue = false
		c.IsOverdue = false
	}
	c.LastDoneBy = doneBy.String
	c.AssigneeName = name.String
	c.AssigneeColor = color.String
	c.AssigneeEmoji = emoji.String
	return c, nil
}
