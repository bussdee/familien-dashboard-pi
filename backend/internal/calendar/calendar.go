package calendar

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-ical"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/teambition/rrule-go"
)

// maxOccurrences caps how many instances a single recurring rule may expand to,
// so a malformed "FREQ=SECONDLY" cannot exhaust memory.
const maxOccurrences = 500

type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Recurring   bool      `json:"recurring"`
	Calendar    string    `json:"calendar"`
	Color       string    `json:"color"`
	// Editable marks appointments entered in the dashboard. Events parsed from
	// an .ics file belong to whoever exported that file and stay read-only.
	Editable bool   `json:"editable"`
	EventID  int    `json:"event_id,omitempty"`
	Repeat   string `json:"repeat,omitempty"`
}

type Service struct {
	icsDir        string
	lookaheadDays int
	watch         bool
	loc           *time.Location
	db            *sql.DB

	mu     sync.RWMutex
	events []Event
}

func NewService(icsDir string, lookaheadDays int, watch bool, timezone string) *Service {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Warn().Str("tz", timezone).Msg("Unknown timezone, falling back to local time")
		loc = time.Local
	}
	if lookaheadDays <= 0 {
		lookaheadDays = 60
	}
	return &Service{
		icsDir:        icsDir,
		lookaheadDays: lookaheadDays,
		watch:         watch,
		loc:           loc,
	}
}

func (s *Service) Start(ctx context.Context) {
	if err := os.MkdirAll(s.icsDir, 0o755); err != nil {
		log.Error().Err(err).Str("dir", s.icsDir).Msg("Cannot create ICS dir")
	}
	s.parseAll()

	if s.watch {
		go s.watchDir(ctx)
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.parseAll()
		}
	}
}

func (s *Service) watchDir(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create ICS watcher")
		return
	}
	defer watcher.Close()

	if err := watcher.Add(s.icsDir); err != nil {
		log.Error().Err(err).Str("dir", s.icsDir).Msg("Failed to watch ICS dir")
		return
	}

	// Editors write .ics files in bursts; debounce so one save re-parses once.
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
			if !strings.HasSuffix(strings.ToLower(event.Name), ".ics") {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(500*time.Millisecond, s.parseAll)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("ICS watcher error")
		}
	}
}

func (s *Service) parseAll() {
	colors := []string{"#3b82f6", "#ec4899", "#f59e0b", "#10b981", "#8b5cf6", "#ef4444"}

	entries, err := os.ReadDir(s.icsDir)
	if err != nil {
		log.Debug().Err(err).Msg("ICS dir not readable")
		return
	}

	var all []Event
	colorIdx := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".ics") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		events, err := s.parseFile(filepath.Join(s.icsDir, entry.Name()), name, colors[colorIdx%len(colors)])
		if err != nil {
			log.Error().Err(err).Str("file", entry.Name()).Msg("Failed to parse ICS")
			continue
		}
		all = append(all, events...)
		colorIdx++
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })

	s.mu.Lock()
	s.events = all
	s.mu.Unlock()

	log.Debug().Int("count", len(all)).Msg("Calendar events parsed")
}

func (s *Service) parseFile(path, calendarName, color string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cal, err := ical.NewDecoder(f).Decode()
	if err != nil {
		return nil, err
	}

	// Keep a little history so "today" still shows this morning's events.
	from := time.Now().In(s.loc).AddDate(0, 0, -1)
	until := time.Now().In(s.loc).AddDate(0, 0, s.lookaheadDays)

	var events []Event
	for _, comp := range cal.Children {
		if comp.Name != ical.CompEvent {
			continue
		}
		events = append(events, s.expandEvent(comp, calendarName, color, from, until)...)
	}
	return events, nil
}

// expandEvent turns one VEVENT into every occurrence inside [from, until].
// Non-recurring events yield at most one.
func (s *Service) expandEvent(comp *ical.Component, calendarName, color string, from, until time.Time) []Event {
	uid := propValue(comp, "UID")
	if uid == "" {
		uid = propValue(comp, "SUMMARY")
	}

	dtstart := comp.Props.Get(ical.PropDateTimeStart)
	if dtstart == nil {
		return nil
	}
	start, allDay, err := s.parseTime(dtstart)
	if err != nil {
		log.Debug().Err(err).Str("uid", uid).Msg("Unparsable DTSTART, skipping event")
		return nil
	}

	duration := time.Hour
	if allDay {
		duration = 24 * time.Hour
	}
	if dtend := comp.Props.Get(ical.PropDateTimeEnd); dtend != nil {
		if end, _, err := s.parseTime(dtend); err == nil && end.After(start) {
			duration = end.Sub(start)
		}
	} else if d := propValue(comp, "DURATION"); d != "" {
		if parsed, err := parseICalDuration(d); err == nil && parsed > 0 {
			duration = parsed
		}
	}

	base := Event{
		Title:       propValue(comp, ical.PropSummary),
		Description: propValue(comp, ical.PropDescription),
		Location:    propValue(comp, ical.PropLocation),
		AllDay:      allDay,
		Calendar:    calendarName,
		Color:       color,
	}
	if base.Title == "" {
		base.Title = "(ohne Titel)"
	}

	rruleStr := propValue(comp, "RRULE")
	if rruleStr == "" {
		if start.After(until) || start.Add(duration).Before(from) {
			return nil
		}
		e := base
		e.ID = uid
		e.Start = start
		e.End = start.Add(duration)
		return []Event{e}
	}

	occurrences, err := expandRRule(rruleStr, start, from, until)
	if err != nil {
		log.Debug().Err(err).Str("uid", uid).Str("rrule", rruleStr).Msg("Unparsable RRULE, using DTSTART only")
		if start.After(until) || start.Add(duration).Before(from) {
			return nil
		}
		e := base
		e.ID = uid
		e.Start = start
		e.End = start.Add(duration)
		e.Recurring = true
		return []Event{e}
	}

	excluded := s.exceptionDates(comp)

	events := make([]Event, 0, len(occurrences))
	for _, occ := range occurrences {
		if _, skip := excluded[occ.Format("20060102T150405")]; skip {
			continue
		}
		if _, skip := excluded[occ.Format("20060102")]; skip {
			continue
		}
		e := base
		e.ID = fmt.Sprintf("%s@%d", uid, occ.Unix())
		e.Start = occ
		e.End = occ.Add(duration)
		e.Recurring = true
		events = append(events, e)
	}
	return events
}

func expandRRule(rule string, dtstart, from, until time.Time) ([]time.Time, error) {
	opts, err := rrule.StrToROption(strings.TrimPrefix(rule, "RRULE:"))
	if err != nil {
		return nil, err
	}
	opts.Dtstart = dtstart

	r, err := rrule.NewRRule(*opts)
	if err != nil {
		return nil, err
	}

	occ := r.Between(from, until, true)
	if len(occ) > maxOccurrences {
		occ = occ[:maxOccurrences]
	}
	return occ, nil
}

func (s *Service) exceptionDates(comp *ical.Component) map[string]struct{} {
	excluded := map[string]struct{}{}
	for _, prop := range comp.Props["EXDATE"] {
		for _, raw := range strings.Split(prop.Value, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			excluded[strings.TrimSuffix(raw, "Z")] = struct{}{}
			if t, _, err := s.parseRawTime(raw, prop.Params.Get("VALUE") == "DATE"); err == nil {
				excluded[t.Format("20060102T150405")] = struct{}{}
				excluded[t.Format("20060102")] = struct{}{}
			}
		}
	}
	return excluded
}

func (s *Service) parseTime(prop *ical.Prop) (time.Time, bool, error) {
	isDate := prop.Params.Get("VALUE") == "DATE"
	t, allDay, err := s.parseRawTime(prop.Value, isDate)
	if err != nil {
		return time.Time{}, false, err
	}
	// A TZID param names the event's own zone; fall back to the dashboard's.
	if tzid := prop.Params.Get("TZID"); tzid != "" && !allDay && !strings.HasSuffix(prop.Value, "Z") {
		if loc, lerr := time.LoadLocation(tzid); lerr == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
		}
	}
	return t, allDay, nil
}

func (s *Service) parseRawTime(value string, isDate bool) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	switch {
	case isDate || !strings.Contains(value, "T"):
		t, err := time.ParseInLocation("20060102", value, s.loc)
		return t, true, err
	case strings.HasSuffix(value, "Z"):
		t, err := time.Parse("20060102T150405Z", value)
		return t.In(s.loc), false, err
	default:
		t, err := time.ParseInLocation("20060102T150405", value, s.loc)
		return t, false, err
	}
}

// parseICalDuration understands the RFC 5545 subset that calendar exports use
// in practice: an optional sign, then weeks/days/hours/minutes/seconds.
func parseICalDuration(v string) (time.Duration, error) {
	v = strings.TrimSpace(strings.ToUpper(v))
	neg := strings.HasPrefix(v, "-")
	v = strings.TrimLeft(v, "+-")
	if !strings.HasPrefix(v, "P") {
		return 0, fmt.Errorf("not a duration: %q", v)
	}
	v = strings.TrimPrefix(v, "P")

	var total time.Duration
	var num strings.Builder
	inTime := false
	for _, c := range v {
		switch {
		case c == 'T':
			inTime = true
		case c >= '0' && c <= '9':
			num.WriteRune(c)
		default:
			n, err := strconv.Atoi(num.String())
			if err != nil {
				return 0, fmt.Errorf("bad duration %q", v)
			}
			num.Reset()
			switch c {
			case 'W':
				total += time.Duration(n) * 7 * 24 * time.Hour
			case 'D':
				total += time.Duration(n) * 24 * time.Hour
			case 'H':
				total += time.Duration(n) * time.Hour
			case 'M':
				if inTime {
					total += time.Duration(n) * time.Minute
				}
			case 'S':
				total += time.Duration(n) * time.Second
			}
		}
	}
	if neg {
		return -total, nil
	}
	return total, nil
}

func propValue(comp *ical.Component, name string) string {
	if p := comp.Props.Get(name); p != nil {
		return p.Value
	}
	return ""
}

func (s *Service) GetEvents(w http.ResponseWriter, r *http.Request) {
	days := 14
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	now := time.Now().In(s.loc)
	cutoff := now.AddDate(0, 0, days)
	floor := now.Add(-12 * time.Hour)

	s.mu.RLock()
	source := s.events
	s.mu.RUnlock()

	filtered := make([]Event, 0, len(source))
	for _, e := range source {
		if e.Start.After(cutoff) || e.End.Before(floor) {
			continue
		}
		filtered = append(filtered, e)
	}

	// Appointments entered in the dashboard sit alongside the .ics ones.
	filtered = append(filtered, s.storedEvents(floor, cutoff)...)
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Start.Before(filtered[j].Start) })

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"events": filtered,
		"now":    now,
	})
}
