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
	RepeatDaily   Repeat = "daily"
	RepeatWeekly  Repeat = "weekly"
	RepeatMonthly Repeat = "monthly"
	RepeatYearly  Repeat = "yearly"
)

func (r Repeat) valid() bool {
	switch r {
	case RepeatNone, RepeatDaily, RepeatWeekly, RepeatMonthly, RepeatYearly:
		return true
	}
	return false
}

func (r Repeat) label() string {
	switch r {
	case RepeatDaily:
		return "Jeden Tag"
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

	// UserID ist die Person, für die der Termin gilt — nicht die, die ihn
	// eingetragen hat. Leer heisst: gilt für alle.
	UserID    *int   `json:"user_id"`
	UserName  string `json:"user_name,omitempty"`
	UserColor string `json:"user_color,omitempty"`
	UserEmoji string `json:"user_emoji,omitempty"`
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
	// UserID 0 heisst: für die ganze Familie.
	UserID int `json:"user_id"`
	// Weekday ist der feste Wochentag wöchentlicher Termine (0 = Montag).
	// Ohne Angabe (-1) bleibt es bei dem Tag, auf den das Datum fällt.
	// Gedacht für „immer donnerstags": Man wählt den Tag, nicht das Datum
	// der ersten Stunde.
	Weekday *int `json:"weekday"`
}

// Gelesen wird immer mit der Person dazu. Der LEFT JOIN kostet nichts und
// erspart der Oberfläche den zweiten Weg zur Benutzerliste.
const eventColumns = `e.id, e.title, e.description, e.location, e.start_at, e.end_at,
	e.all_day, e.repeat, e.color, e.created_by, e.user_id, u.name, u.color, u.avatar_emoji`

const eventFrom = ` FROM calendar_events e LEFT JOIN users u ON u.id = e.user_id`

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

	rows, err := s.db.Query(`SELECT ` + eventColumns + eventFrom + ` ORDER BY e.start_at`)
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

	var person int
	if e.UserID != nil {
		person = *e.UserID
	}
	build := func(start time.Time, recurring bool) Event {
		id := fmt.Sprintf("local-%d", e.ID)
		if recurring {
			id = fmt.Sprintf("local-%d@%d", e.ID, start.Unix())
		}
		var serie *time.Time
		if recurring {
			erster := e.Start
			serie = &erster
		}
		return Event{
			SeriesStart: serie,
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
			UserID:      person,
			UserName:    e.UserName,
			UserColor:   e.UserColor,
			UserEmoji:   e.UserEmoji,
		}
	}

	if e.Repeat == RepeatNone || e.Repeat == "" {
		if e.Start.After(until) || e.Start.Add(duration).Before(from) {
			return nil
		}
		return []Event{build(e.Start, false)}
	}

	var out []Event
	start := vorspulen(e.Start, e.Repeat, from, duration)
	for occ := start; !occ.After(until) && len(out) < maxOccurrences; occ = nextOccurrence(occ, e.Repeat) {
		if !occ.Add(duration).Before(from) {
			out = append(out, build(occ, true))
		}
	}
	return out
}

// vorspulen überspringt die Wiederholungen, die längst vorbei sind.
//
// Ohne das zählte ein täglicher Termin vom letzten Jahr Tag für Tag nach
// vorn und wäre bei maxOccurrences am Ende, bevor er das heutige Datum
// erreicht — der Termin verschwände einfach aus dem Kalender.
//
// Gerechnet wird knapp: Die ganzzahlige Division springt eher zu kurz als
// zu weit, und den Rest erledigt die Schleife. Andersherum wäre eine
// Wiederholung übersprungen.
func vorspulen(start time.Time, repeat Repeat, from time.Time, duration time.Duration) time.Time {
	rueckstand := from.Sub(start.Add(duration))
	if rueckstand <= 0 {
		return start
	}
	switch repeat {
	case RepeatDaily:
		if tage := int(rueckstand.Hours() / 24); tage > 0 {
			return start.AddDate(0, 0, tage)
		}
	case RepeatWeekly:
		if wochen := int(rueckstand.Hours() / (24 * 7)); wochen > 0 {
			return start.AddDate(0, 0, wochen*7)
		}
	case RepeatMonthly:
		if monate := int(rueckstand.Hours() / (24 * 31)); monate > 0 {
			return start.AddDate(0, monate, 0)
		}
	case RepeatYearly:
		if jahre := int(rueckstand.Hours() / (24 * 366)); jahre > 0 {
			return start.AddDate(jahre, 0, 0)
		}
	}
	return start
}

// nextOccurrence steps one interval forward. AddDate normalises overflow, so a
// monthly event on the 31st lands on the 1st of the following month rather than
// disappearing — visible and correctable, unlike silently skipping it.
func nextOccurrence(t time.Time, repeat Repeat) time.Time {
	switch repeat {
	case RepeatDaily:
		return t.AddDate(0, 0, 1)
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

	rows, err := s.db.Query(`SELECT ` + eventColumns + eventFrom + ` ORDER BY e.start_at DESC`)
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

	person, err := s.personOderNil(req.UserID)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := s.db.Exec(`
		INSERT INTO calendar_events
			(title, description, location, start_at, end_at, all_day, repeat, color, created_by, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(req.Title), req.Description, req.Location,
		start, end, req.AllDay, string(req.Repeat), req.Color, urheber, person)
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

	person, err := s.personOderNil(req.UserID)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := s.db.Exec(`
		UPDATE calendar_events SET
			title = ?, description = ?, location = ?, start_at = ?, end_at = ?,
			all_day = ?, repeat = ?, color = ?, user_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		strings.TrimSpace(req.Title), req.Description, req.Location,
		start, end, req.AllDay, string(req.Repeat), req.Color, person, id); err != nil {
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

	// „Immer donnerstags" ist das, was man sagen will — nicht „ab dem 18.,
	// und der ist zufällig ein Donnerstag". Der gewählte Wochentag schiebt
	// das Datum auf den nächsten passenden Tag vor, und ab da trägt das
	// Startdatum den Wochentag. Zwei Quellen für dieselbe Aussage wären eine
	// zu viel.
	if req.Repeat == RepeatWeekly && req.Weekday != nil && *req.Weekday >= 0 && *req.Weekday <= 6 {
		day = naechsterWochentag(day, *req.Weekday)
		req.Date = day.Format("2006-01-02")
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

// naechsterWochentag schiebt ein Datum auf den nächsten gewünschten
// Wochentag vor — 0 ist Montag, so wie man einen Kalender liest, und nicht
// Sonntag wie time.Weekday.
func naechsterWochentag(tag time.Time, gewuenscht int) time.Time {
	ist := (int(tag.Weekday()) + 6) % 7
	abstand := (gewuenscht - ist + 7) % 7
	return tag.AddDate(0, 0, abstand)
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// personOderNil prüft die gewählte Person. 0 heisst „für alle" und ist der
// Normalfall — ein Familienkalender gehört erst einmal allen.
func (s *Service) personOderNil(userID int) (any, error) {
	if userID <= 0 {
		return nil, nil
	}
	var da int
	if err := s.db.QueryRow("SELECT 1 FROM users WHERE id = ?", userID).Scan(&da); err != nil {
		return nil, fmt.Errorf("Unbekanntes Familienmitglied")
	}
	return userID, nil
}

func eventIDParam(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

func (s *Service) eventByID(id int) (StoredEvent, error) {
	return scanEvent(s.db.QueryRow(`SELECT `+eventColumns+eventFrom+` WHERE e.id = ?`, id))
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEvent(row scanner) (StoredEvent, error) {
	var e StoredEvent
	var repeat string
	var createdBy, userID sql.NullInt64
	var name, color, emoji sql.NullString

	if err := row.Scan(&e.ID, &e.Title, &e.Description, &e.Location,
		&e.Start, &e.End, &e.AllDay, &repeat, &e.Color, &createdBy,
		&userID, &name, &color, &emoji); err != nil {
		return StoredEvent{}, err
	}
	e.Repeat = Repeat(repeat)
	if createdBy.Valid {
		id := int(createdBy.Int64)
		e.CreatedBy = &id
	}
	if userID.Valid {
		id := int(userID.Int64)
		e.UserID = &id
		e.UserName = name.String
		e.UserColor = color.String
		e.UserEmoji = emoji.String
	}
	return e, nil
}
