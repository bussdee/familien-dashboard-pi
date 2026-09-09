package calendar

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Repeat covers what a family actually needs: a one-off, the weekly music
// lesson, the monthly rent reminder, a birthday. Anything more exotic belongs
// in an .ics file exported from a real calendar app.
type Repeat string

const (
	RepeatNone    Repeat = "none"
	RepeatWeekly  Repeat = "weekly"
	RepeatMonthly Repeat = "monthly"
	RepeatYearly  Repeat = "yearly"
)

func (r Repeat) valid() bool {
	switch r {
	case RepeatNone, RepeatWeekly, RepeatMonthly, RepeatYearly:
		return true
	}
	return false
}

func (r Repeat) label() string {
	switch r {
	case RepeatWeekly:
		return "Jede Woche"
	case RepeatMonthly:
		return "Jeden Monat"
	case RepeatYearly:
		return "Jedes Jahr"
	default:
		return "Einmalig"
	}
}

// StoredEvent is an appointment entered in the dashboard, as opposed to one
// parsed from an .ics file.
type StoredEvent struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Repeat      Repeat    `json:"repeat"`
	Color       string    `json:"color"`
	CreatedBy   *int      `json:"created_by"`
}

type eventPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Date        string `json:"date"`       // 2026-09-15
	StartTime   string `json:"start_time"` // 18:00, empty when all-day
	EndTime     string `json:"end_time"`   // 20:00, optional
	AllDay      bool   `json:"all_day"`
	Repeat      Repeat `json:"repeat"`
	Color       string `json:"color"`
}

const eventColumns = `id, title, description, location, start_at, end_at, all_day, repeat, color, created_by`

// SetDB gives the calendar access to dashboard-entered events. Without it the
// service stays read-only over the .ics files.
func (s *Service) SetDB(db *sql.DB) {
	s.db = db
}

// storedEvents expands the database events into concrete occurrences inside
// [from, until], the same way ICS events are expanded.
func (s *Service) storedEvents(from, until time.Time) []Event {
	if s.db == nil {
		return nil
	}

	rows, err := s.db.Query(`SELECT ` + eventColumns + ` FROM calendar_events ORDER BY start_at`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load calendar events")
		return nil
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		stored, err := scanEvent(rows)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan calendar event")
			continue
		}
		events = append(events, s.expandStored(stored, from, until)...)
	}
	return events
}

func (s *Service) expandStored(e StoredEvent, from, until time.Time) []Event {
	duration := e.End.Sub(e.Start)
	if duration <= 0 {
		duration = time.Hour
		if e.AllDay {
			duration = 24 * time.Hour
		}
	}

	build := func(start time.Time, recurring bool) Event {
		id := fmt.Sprintf("local-%d", e.ID)
		if recurring {
			id = fmt.Sprintf("local-%d@%d", e.ID, start.Unix())
		}
		return Event{
			ID:          id,
			Title:       e.Title,
			Description: e.Description,
			Location:    e.Location,
			Start:       start,
			End:         start.Add(duration),
			AllDay:      e.AllDay,
			Recurring:   recurring,
			Calendar:    "Dashboard",
			Color:       e.Color,
			Editable:    true,
			EventID:     e.ID,
			Repeat:      string(e.Repeat),
		}
	}

	if e.Repeat == RepeatNone || e.Repeat == "" {
		if e.Start.After(until) || e.Start.Add(duration).Before(from) {
			return nil
		}
		return []Event{build(e.Start, false)}
	}

	var out []Event
	for occ := e.Start; !occ.After(until) && len(out) < maxOccurrences; occ = nextOccurrence(occ, e.Repeat) {
		if !occ.Add(duration).Before(from) {
			out = append(out, build(occ, true))
		}
	}
	return out
}

// nextOccurrence steps one interval forward. AddDate normalises overflow, so a
// monthly event on the 31st lands on the 1st of the following month rather than
// disappearing — visible and correctable, unlike silently skipping it.
func nextOccurrence(t time.Time, repeat Repeat) time.Time {
	switch repeat {
	case RepeatWeekly:
		return t.AddDate(0, 0, 7)
	case RepeatMonthly:
		return t.AddDate(0, 1, 0)
	case RepeatYearly:
		return t.AddDate(1, 0, 0)
	default:
		// Guarantees progress so the caller's loop always terminates.
		return t.AddDate(100, 0, 0)
	}
}

// ------------------------------------------------------------------ Handlers

// ListStored returns the raw appointments (not expanded) for the management UI.
func (s *Service) ListStored(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		auth.WriteJSON(w, []StoredEvent{})
		return
	}

	rows, err := s.db.Query(`SELECT ` + eventColumns + ` FROM calendar_events ORDER BY start_at DESC`)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	events := []StoredEvent{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			continue
		}
		events = append(events, e)
	}
	auth.WriteJSON(w, events)
}

func (s *Service) CreateEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok && !auth.IsDevice(r) {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}
	// Am Wandgerät bleibt der Urheber offen — der Termin gehört der Familie.
	var urheber any
	if ok {
		urheber = userID
	}
	if s.db == nil {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Termine sind nicht verfügbar")
		return
	}

	var req eventPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	start, end, err := s.parsePayload(req)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Repeat.valid() {
		req.Repeat = RepeatNone
	}
	if req.Color == "" {
		req.Color = "#0d9488"
	}

	res, err := s.db.Exec(`
		INSERT INTO calendar_events
			(title, description, location, start_at, end_at, all_day, repeat, color, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(req.Title), req.Description, req.Location,
		start, end, req.AllDay, string(req.Repeat), req.Color, urheber)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create calendar event")
		auth.HTTPError(w, http.StatusInternalServerError, "Termin konnte nicht gespeichert werden")
		return
	}

	id, _ := res.LastInsertId()
	event, err := s.eventByID(int(id))
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, event)
}

func (s *Service) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Termine sind nicht verfügbar")
		return
	}
	id, err := eventIDParam(r)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var req eventPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	start, end, err := s.parsePayload(req)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Repeat.valid() {
		req.Repeat = RepeatNone
	}
	if req.Color == "" {
		req.Color = "#0d9488"
	}

	if _, err := s.db.Exec(`
		UPDATE calendar_events SET
			title = ?, description = ?, location = ?, start_at = ?, end_at = ?,
			all_day = ?, repeat = ?, color = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		strings.TrimSpace(req.Title), req.Description, req.Location,
		start, end, req.AllDay, string(req.Repeat), req.Color, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Termin konnte nicht gespeichert werden")
		return
	}

	event, err := s.eventByID(id)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Termin nicht gefunden")
		return
	}
	auth.WriteJSON(w, event)
}

func (s *Service) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Termine sind nicht verfügbar")
		return
	}
	id, err := eventIDParam(r)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	if _, err := s.db.Exec("DELETE FROM calendar_events WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parsePayload turns the form fields into times in the dashboard's timezone.
func (s *Service) parsePayload(req eventPayload) (time.Time, time.Time, error) {
	if strings.TrimSpace(req.Title) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("Titel fehlt")
	}
	if strings.TrimSpace(req.Date) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("Datum fehlt")
	}

	day, err := time.ParseInLocation("2006-01-02", req.Date, s.loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("Datum muss im Format JJJJ-MM-TT sein")
	}

	if req.AllDay {
		return day, day.AddDate(0, 0, 1), nil
	}

	start, err := time.ParseInLocation("2006-01-02 15:04", req.Date+" "+orDefault(req.StartTime, "09:00"), s.loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("Startzeit muss im Format HH:MM sein")
	}

	end := start.Add(time.Hour)
	if req.EndTime != "" {
		parsed, err := time.ParseInLocation("2006-01-02 15:04", req.Date+" "+req.EndTime, s.loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("Endzeit muss im Format HH:MM sein")
		}
		// An end before the start means the appointment runs past midnight.
		if !parsed.After(start) {
			parsed = parsed.AddDate(0, 0, 1)
		}
		end = parsed
	}
	return start, end, nil
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func eventIDParam(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

func (s *Service) eventByID(id int) (StoredEvent, error) {
	return scanEvent(s.db.QueryRow(`SELECT `+eventColumns+` FROM calendar_events WHERE id = ?`, id))
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEvent(row scanner) (StoredEvent, error) {
	var e StoredEvent
	var repeat string
	var createdBy sql.NullInt64

	if err := row.Scan(&e.ID, &e.Title, &e.Description, &e.Location,
		&e.Start, &e.End, &e.AllDay, &repeat, &e.Color, &createdBy); err != nil {
		return StoredEvent{}, err
	}
	e.Repeat = Repeat(repeat)
	if createdBy.Valid {
		id := int(createdBy.Int64)
		e.CreatedBy = &id
	}
	return e, nil
}
