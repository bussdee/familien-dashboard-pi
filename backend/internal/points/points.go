// Package points holds the scoring rules for the family leaderboard. Chores and
// shopping runs both feed the same table, so a single place decides what a
// point is worth and how levels, streaks and badges are derived from it.
package points

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Source labels tell the history apart. They are stored, so keep them stable.
const (
	SourceChore    = "chore"
	SourceShopping = "shopping"
	SourceBonus    = "bonus"
)

// pointsPerLevel is deliberately flat rather than exponential: a child should
// reach the next level often enough for it to feel worth doing.
const pointsPerLevel = 100

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Award records points. It is called inside the caller's transaction where one
// exists, so the score can never drift from the action that earned it.
func Award(tx *sql.Tx, userID int, source string, referenceID int, amount int, note string) error {
	var ref any
	if referenceID > 0 {
		ref = referenceID
	}
	_, err := tx.Exec(`
		INSERT INTO point_events (user_id, source, reference_id, points, note)
		VALUES (?, ?, ?, ?, ?)`, userID, source, ref, amount, note)
	return err
}

// AwardDirect is the same without a surrounding transaction.
func (s *Service) AwardDirect(userID int, source string, referenceID, amount int, note string) error {
	var ref any
	if referenceID > 0 {
		ref = referenceID
	}
	_, err := s.db.Exec(`
		INSERT INTO point_events (user_id, source, reference_id, points, note)
		VALUES (?, ?, ?, ?, ?)`, userID, source, ref, amount, note)
	return err
}

type Badge struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Emoji       string `json:"emoji"`
	Description string `json:"description"`
}

type Score struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	AvatarEmoji string `json:"avatar_emoji"`

	TotalPoints int `json:"total_points"`
	ThisWeek    int `json:"this_week"`
	Today       int `json:"today"`
	Activities  int `json:"activities"`
	ChoreCount  int `json:"chore_count"`
	ShopCount   int `json:"shop_count"`

	Rank      int    `json:"rank"`
	WeekRank  int    `json:"week_rank"`
	Level     int    `json:"level"`
	LevelName string `json:"level_name"`
	// Progress within the current level, 0-100.
	LevelProgress int `json:"level_progress"`
	PointsToNext  int `json:"points_to_next"`

	StreakDays int     `json:"streak_days"`
	LastActive string  `json:"last_active,omitempty"`
	Badges     []Badge `json:"badges"`
}

// levelName gives each level a title, because "Level 4" means less to a seven
// year old than "Profi".
func levelName(level int) string {
	names := []string{
		"Neuling", "Helfer", "Fleißig", "Profi", "Experte",
		"Meister", "Champion", "Legende",
	}
	if level-1 < len(names) {
		return names[level-1]
	}
	return "Legende"
}

func deriveLevel(total int) (level, progress, toNext int) {
	if total < 0 {
		total = 0
	}
	level = total/pointsPerLevel + 1
	within := total % pointsPerLevel
	progress = within * 100 / pointsPerLevel
	toNext = pointsPerLevel - within
	return
}

// Leaderboard returns every family member with their score, ranked. Members
// without a single point are included so the board shows the whole family from
// day one rather than looking broken.
func (s *Service) Leaderboard() ([]Score, error) {
	rows, err := s.db.Query(`
		SELECT
			u.id, u.name, u.color, u.avatar_emoji,
			COALESCE(SUM(p.points), 0)                                            AS total,
			COALESCE(SUM(CASE WHEN p.created_at >= datetime('now', '-7 days')
			                  THEN p.points ELSE 0 END), 0)                       AS week,
			COALESCE(SUM(CASE WHEN date(p.created_at) = date('now')
			                  THEN p.points ELSE 0 END), 0)                       AS today,
			COUNT(p.id)                                                           AS activities,
			COALESCE(SUM(CASE WHEN p.source = 'chore'    THEN 1 ELSE 0 END), 0)   AS chores,
			COALESCE(SUM(CASE WHEN p.source = 'shopping' THEN 1 ELSE 0 END), 0)   AS shops,
			MAX(p.created_at)                                                     AS last_active
		FROM users u
		LEFT JOIN point_events p ON p.user_id = u.id
		GROUP BY u.id
		ORDER BY total DESC, u.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scores := []Score{}
	for rows.Next() {
		var sc Score
		var lastActive sql.NullString
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Color, &sc.AvatarEmoji,
			&sc.TotalPoints, &sc.ThisWeek, &sc.Today, &sc.Activities,
			&sc.ChoreCount, &sc.ShopCount, &lastActive); err != nil {
			return nil, err
		}
		sc.LastActive = lastActive.String
		sc.Level, sc.LevelProgress, sc.PointsToNext = deriveLevel(sc.TotalPoints)
		sc.LevelName = levelName(sc.Level)
		scores = append(scores, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	streaks, err := s.streaks()
	if err != nil {
		log.Error().Err(err).Msg("Failed to compute streaks")
	}

	// Ranks share a position on a tie, so two people on 40 points are both 2nd.
	assignRanks(scores, func(sc Score) int { return sc.TotalPoints },
		func(sc *Score, rank int) { sc.Rank = rank })

	weekly := make([]Score, len(scores))
	copy(weekly, scores)
	sortByWeek(weekly)
	weekRank := map[int]int{}
	assignRanks(weekly, func(sc Score) int { return sc.ThisWeek },
		func(sc *Score, rank int) { weekRank[sc.ID] = rank })

	weekLeader := 0
	if len(weekly) > 0 && weekly[0].ThisWeek > 0 {
		weekLeader = weekly[0].ID
	}

	for i := range scores {
		scores[i].StreakDays = streaks[scores[i].ID]
		scores[i].WeekRank = weekRank[scores[i].ID]
		scores[i].Badges = badgesFor(scores[i], weekLeader)
	}
	return scores, nil
}

func assignRanks(scores []Score, value func(Score) int, set func(*Score, int)) {
	rank, previous := 0, -1
	for i := range scores {
		v := value(scores[i])
		if v != previous {
			rank = i + 1
			previous = v
		}
		set(&scores[i], rank)
	}
}

func sortByWeek(scores []Score) {
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].ThisWeek > scores[j-1].ThisWeek; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}
}

// streaks counts consecutive days ending today (or yesterday, so an evening
// person does not lose the streak before they have had a chance to act).
func (s *Service) streaks() (map[int]int, error) {
	rows, err := s.db.Query(`
		SELECT user_id, date(created_at) AS day
		FROM point_events
		WHERE created_at >= datetime('now', '-120 days')
		GROUP BY user_id, day
		ORDER BY user_id, day DESC`)
	if err != nil {
		return map[int]int{}, err
	}
	defer rows.Close()

	days := map[int][]string{}
	for rows.Next() {
		var userID int
		var day string
		if err := rows.Scan(&userID, &day); err != nil {
			continue
		}
		days[userID] = append(days[userID], day)
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	result := map[int]int{}
	for userID, list := range days {
		if len(list) == 0 || (list[0] != today && list[0] != yesterday) {
			continue
		}
		streak := 1
		cursor, err := time.Parse("2006-01-02", list[0])
		if err != nil {
			continue
		}
		for _, day := range list[1:] {
			expected := cursor.AddDate(0, 0, -1).Format("2006-01-02")
			if day != expected {
				break
			}
			streak++
			cursor = cursor.AddDate(0, 0, -1)
		}
		result[userID] = streak
	}
	return result, nil
}

// badgesFor turns raw numbers into things worth showing off.
func badgesFor(sc Score, weekLeader int) []Badge {
	badges := []Badge{}
	add := func(id, emoji, label, description string) {
		badges = append(badges, Badge{ID: id, Emoji: emoji, Label: label, Description: description})
	}

	if sc.ChoreCount >= 1 {
		add("first-chore", "🌱", "Angefangen", "Erste Aufgabe erledigt")
	}
	if sc.ChoreCount >= 10 {
		add("chores-10", "💪", "Fleißig", "10 Aufgaben erledigt")
	}
	if sc.ChoreCount >= 50 {
		add("chores-50", "🏅", "Ausdauernd", "50 Aufgaben erledigt")
	}
	if sc.ShopCount >= 1 {
		add("first-shop", "🛒", "Eingekauft", "Einen Einkauf erledigt")
	}
	if sc.ShopCount >= 5 {
		add("shop-5", "🥕", "Einkaufsheld", "5 Einkäufe erledigt")
	}
	if sc.StreakDays >= 3 {
		add("streak-3", "🔥", fmt.Sprintf("%d Tage Serie", sc.StreakDays),
			"An mehreren Tagen hintereinander aktiv")
	}
	if sc.TotalPoints >= 100 {
		add("points-100", "⭐", "100 Punkte", "Insgesamt 100 Punkte gesammelt")
	}
	if sc.TotalPoints >= 500 {
		add("points-500", "🌟", "500 Punkte", "Insgesamt 500 Punkte gesammelt")
	}
	if sc.Rank == 1 && sc.TotalPoints > 0 {
		add("leader", "👑", "Spitzenreiter", "Aktuell die meisten Punkte")
	}
	if weekLeader == sc.ID {
		add("week-leader", "🏆", "Woche gewonnen", "Diese Woche die meisten Punkte")
	}
	return badges
}

// ------------------------------------------------------------------ Handlers

func (s *Service) Scoreboard(w http.ResponseWriter, r *http.Request) {
	scores, err := s.Leaderboard()
	if err != nil {
		log.Error().Err(err).Msg("Failed to build leaderboard")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, scores)
}

type Activity struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	UserName  string    `json:"user_name"`
	UserEmoji string    `json:"user_emoji"`
	Source    string    `json:"source"`
	Points    int       `json:"points"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// History powers the "was zuletzt passiert ist" feed, which does more for
// motivation than a number on its own.
func (s *Service) History(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT p.id, p.user_id, u.name, u.avatar_emoji, p.source, p.points, p.note, p.created_at
		FROM point_events p
		JOIN users u ON u.id = p.user_id
		ORDER BY p.created_at DESC
		LIMIT 20`)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	items := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.UserID, &a.UserName, &a.UserEmoji,
			&a.Source, &a.Points, &a.Note, &a.CreatedAt); err != nil {
			continue
		}
		items = append(items, a)
	}
	auth.WriteJSON(w, items)
}

// ------------------------------------------------------- Verwaltung (Admin)

type adjustment struct {
	UserID int    `json:"user_id"`
	Points int    `json:"points"`
	Note   string `json:"note"`
}

// Adjust books a manual correction. Negative values are allowed — that is the
// whole point of it, for when somebody tapped the wrong thing.
func (s *Service) Adjust(w http.ResponseWriter, r *http.Request) {
	var req adjustment
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if req.UserID <= 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Kein Benutzer gewählt")
		return
	}
	if req.Points == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Punktzahl darf nicht 0 sein")
		return
	}
	if req.Points < -10000 || req.Points > 10000 {
		auth.HTTPError(w, http.StatusBadRequest, "Punktzahl liegt außerhalb des zulässigen Bereichs")
		return
	}

	var exists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", req.UserID).Scan(&exists); err != nil || exists == 0 {
		auth.HTTPError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}

	note := strings.TrimSpace(req.Note)
	if note == "" {
		if req.Points > 0 {
			note = "Bonus"
		} else {
			note = "Korrektur"
		}
	}

	if err := s.AwardDirect(req.UserID, SourceBonus, 0, req.Points, note); err != nil {
		log.Error().Err(err).Msg("Failed to adjust points")
		auth.HTTPError(w, http.StatusInternalServerError, "Punkte konnten nicht gebucht werden")
		return
	}

	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]any{"user_id": req.UserID, "points": req.Points, "note": note})
}

// Revoke removes one entry from the history. For a chore it also deletes the
// completion and rewinds the chore's due date, so an accidental tap does not
// leave the task looking done for the next two weeks.
func (s *Service) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var source string
	var reference sql.NullInt64
	if err := s.db.QueryRow(
		"SELECT source, reference_id FROM point_events WHERE id = ?", id,
	).Scan(&source, &reference); err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Eintrag nicht gefunden")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM point_events WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	if source == SourceChore && reference.Valid {
		if err := revokeCompletion(tx, int(reference.Int64)); err != nil {
			log.Error().Err(err).Msg("Failed to revoke chore completion")
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

// revokeCompletion deletes the completion and restores the chore's dates from
// whatever completion came before it — or clears them, making the chore due
// again, if that was the only one.
func revokeCompletion(tx *sql.Tx, completionID int) error {
	var choreID int
	if err := tx.QueryRow(
		"SELECT chore_id FROM chore_completions WHERE id = ?", completionID).Scan(&choreID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // already gone; nothing left to undo
		}
		return err
	}

	if _, err := tx.Exec("DELETE FROM chore_completions WHERE id = ?", completionID); err != nil {
		return err
	}

	var intervalDays int
	if err := tx.QueryRow(
		"SELECT interval_days FROM chores WHERE id = ?", choreID).Scan(&intervalDays); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	var previous sql.NullTime
	if err := tx.QueryRow(
		"SELECT MAX(completed_at) FROM chore_completions WHERE chore_id = ?", choreID,
	).Scan(&previous); err != nil {
		return err
	}

	if previous.Valid {
		_, err := tx.Exec(
			`UPDATE chores SET last_done_at = ?, next_due_at = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`,
			previous.Time, previous.Time.AddDate(0, 0, intervalDays), choreID)
		return err
	}

	// Never completed: due right away.
	_, err := tx.Exec(
		`UPDATE chores SET last_done_at = NULL, next_due_at = datetime('now'),
		 updated_at = CURRENT_TIMESTAMP WHERE id = ?`, choreID)
	return err
}

// ResetUser wipes one person's score, or everyone's when user_id is 0.
func (s *Service) ResetUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int `json:"user_id"`
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

	var removed int64
	if req.UserID > 0 {
		res, err := tx.Exec("DELETE FROM point_events WHERE user_id = ?", req.UserID)
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		removed, _ = res.RowsAffected()
		if _, err := tx.Exec("DELETE FROM chore_completions WHERE user_id = ?", req.UserID); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
	} else {
		res, err := tx.Exec("DELETE FROM point_events")
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		removed, _ = res.RowsAffected()
		if _, err := tx.Exec("DELETE FROM chore_completions"); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	log.Info().Int("user_id", req.UserID).Int64("removed", removed).Msg("Points reset")
	auth.WriteJSON(w, map[string]int64{"removed": removed})
}

// AdminHistory is the full, filterable ledger behind the admin screen.
func (s *Service) AdminHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	query := `
		SELECT p.id, p.user_id, u.name, u.avatar_emoji, p.source, p.points, p.note, p.created_at
		FROM point_events p JOIN users u ON u.id = p.user_id`
	args := []any{}
	if v := r.URL.Query().Get("user_id"); v != "" {
		if userID, err := strconv.Atoi(v); err == nil && userID > 0 {
			query += ` WHERE p.user_id = ?`
			args = append(args, userID)
		}
	}
	query += ` ORDER BY p.created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	items := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.UserID, &a.UserName, &a.UserEmoji,
			&a.Source, &a.Points, &a.Note, &a.CreatedAt); err != nil {
			continue
		}
		items = append(items, a)
	}
	auth.WriteJSON(w, items)
}
