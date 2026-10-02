// Package goals ist das Familienziel: ein gemeinsames Ziel statt eines
// Wettrennens.
//
// Die Rangliste hat eine Schwäche, die in jeder Familie mit Geschwistern
// auffällt: Das Jüngste liegt immer hinten, und irgendwann will es nicht mehr
// mitspielen. Ein Familienziel dreht das um. „300 Punkte zusammen, dann gibt
// es den Pizza-Abend" — jeder Punkt, egal von wem, bringt alle näher dran.
//
// Gezählt wird alles Verdiente seit dem Start des Ziels. Abzüge zählen nicht
// dagegen: Ein Streit zwischen zwei Kindern soll nicht den Ausflug der ganzen
// Familie kosten.
package goals

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
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

const maxTarget = 100000

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

type Goal struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Emoji     string     `json:"emoji"`
	Target    int        `json:"target"`
	StartedAt time.Time  `json:"started_at"`
	ReachedAt *time.Time `json:"reached_at,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

// Beitrag ist, was eine Person zum Ziel beigesteuert hat. Er wird gezeigt,
// aber nicht gerankt: Es geht ums Gemeinsame.
type Beitrag struct {
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
	Emoji  string `json:"avatar_emoji"`
	Color  string `json:"color"`
	Points int    `json:"points"`
}

type Overview struct {
	Goal          *Goal     `json:"goal"`
	Progress      int       `json:"progress"`
	Contributions []Beitrag `json:"contributions"`
	// Die zuletzt geschafften Ziele — was die Familie schon zusammen
	// erreicht hat.
	Reached []Goal `json:"reached"`
}

// ------------------------------------------------------------------ Lesen

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	out, err := s.overview()
	if err != nil {
		log.Error().Err(err).Msg("Familienziel konnte nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, out)
}

func (s *Service) overview() (Overview, error) {
	out := Overview{Contributions: []Beitrag{}, Reached: []Goal{}}

	g, err := s.active()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err == nil {
		out.Goal = &g
		out.Contributions, out.Progress, err = s.beitraege(g.StartedAt)
		if err != nil {
			return out, err
		}
		// Geschafft wird einmal festgehalten — der Zeitpunkt soll stehen
		// bleiben, auch wenn danach weiter Punkte dazukommen.
		if out.Progress >= g.Target && g.ReachedAt == nil {
			now := time.Now()
			if _, err := s.db.Exec(
				`UPDATE family_goals SET reached_at = ? WHERE id = ? AND reached_at IS NULL`,
				now.UTC().Format("2006-01-02 15:04:05"), g.ID); err != nil {
				return out, err
			}
			out.Goal.ReachedAt = &now
		}
	}

	rows, err := s.db.Query(`
		SELECT id, title, emoji, target, started_at, reached_at, closed_at
		FROM family_goals WHERE closed_at IS NOT NULL AND reached_at IS NOT NULL
		ORDER BY closed_at DESC LIMIT 5`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var g Goal
		if err := scanGoal(rows, &g); err != nil {
			return out, err
		}
		out.Reached = append(out.Reached, g)
	}
	return out, rows.Err()
}

func (s *Service) active() (Goal, error) {
	var g Goal
	row := s.db.QueryRow(`
		SELECT id, title, emoji, target, started_at, reached_at, closed_at
		FROM family_goals WHERE closed_at IS NULL ORDER BY id DESC LIMIT 1`)
	return g, scanGoal(row, &g)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanGoal(row scanner, g *Goal) error {
	var reached, closed sql.NullTime
	if err := row.Scan(&g.ID, &g.Title, &g.Emoji, &g.Target, &g.StartedAt, &reached, &closed); err != nil {
		return err
	}
	if reached.Valid {
		g.ReachedAt = &reached.Time
	}
	if closed.Valid {
		g.ClosedAt = &closed.Time
	}
	return nil
}

// beitraege zählt, was jede Person seit dem Start verdient hat — nur das
// Positive, Abzüge bleiben aussen vor.
func (s *Service) beitraege(seit time.Time) ([]Beitrag, int, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.name, u.avatar_emoji, u.color, COALESCE(SUM(p.points), 0)
		FROM point_events p JOIN users u ON u.id = p.user_id
		WHERE p.points > 0 AND p.created_at >= ?
		GROUP BY u.id
		ORDER BY u.id`, seit.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Beitrag{}
	summe := 0
	for rows.Next() {
		var b Beitrag
		if err := rows.Scan(&b.UserID, &b.Name, &b.Emoji, &b.Color, &b.Points); err != nil {
			return nil, 0, err
		}
		summe += b.Points
		list = append(list, b)
	}
	return list, summe, rows.Err()
}

// ------------------------------------------------------- Verwalten (Eltern)

type goalPayload struct {
	Title  string `json:"title"`
	Emoji  string `json:"emoji"`
	Target int    `json:"target"`
}

func pruefe(p *goalPayload) string {
	p.Title = strings.TrimSpace(p.Title)
	p.Emoji = strings.TrimSpace(p.Emoji)
	switch {
	case p.Title == "":
		return "Worauf spart die Familie?"
	case utf8.RuneCountInString(p.Title) > 80:
		return "Der Titel ist zu lang (höchstens 80 Zeichen)"
	case p.Target <= 0:
		return "Ein Ziel braucht mindestens 1 Punkt"
	case p.Target > maxTarget:
		return "So viele Punkte sammelt niemand"
	}
	if p.Emoji == "" {
		p.Emoji = "🎯"
	}
	if utf8.RuneCountInString(p.Emoji) > 8 {
		return "Bitte nur ein Symbol"
	}
	return ""
}

// Create setzt ein neues Ziel. Ein noch laufendes wird dabei abgeschlossen:
// Es gibt immer nur eines, sonst wüsste niemand, wofür er gerade sammelt.
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	var p goalPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if msg := pruefe(&p); msg != "" {
		auth.HTTPError(w, http.StatusBadRequest, msg)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`UPDATE family_goals SET closed_at = CURRENT_TIMESTAMP WHERE closed_at IS NULL`); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	res, err := tx.Exec(
		`INSERT INTO family_goals (title, emoji, target) VALUES (?, ?, ?)`, p.Title, p.Emoji, p.Target)
	if err != nil {
		log.Error().Err(err).Msg("Familienziel konnte nicht angelegt werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]any{"id": id})
}

// Update ändert Titel, Symbol oder Zielwert. Der Startpunkt bleibt: Was schon
// gesammelt ist, bleibt gesammelt.
func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var p goalPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if msg := pruefe(&p); msg != "" {
		auth.HTTPError(w, http.StatusBadRequest, msg)
		return
	}
	// Wird das Ziel höher gesetzt, als bisher gesammelt ist, gilt es wieder
	// als offen.
	res, err := s.db.Exec(`
		UPDATE family_goals SET title = ?, emoji = ?, target = ?,
		       reached_at = CASE WHEN target < ? THEN NULL ELSE reached_at END
		WHERE id = ? AND closed_at IS NULL`, p.Title, p.Emoji, p.Target, p.Target, id)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusNotFound, "Kein laufendes Ziel mit dieser Kennung")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Close schliesst das Ziel ab — eingelöst, oder aufgegeben. Ein geschafftes
// erscheint danach in der Liste der erreichten Ziele.
func (s *Service) Close(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	res, err := s.db.Exec(
		`UPDATE family_goals SET closed_at = CURRENT_TIMESTAMP WHERE id = ? AND closed_at IS NULL`, id)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusNotFound, "Kein laufendes Ziel mit dieser Kennung")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
