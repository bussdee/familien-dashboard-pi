package devices

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Target is a device as configured in the admin area.
type Target struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	Link         string `json:"link"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ExpectStatus int    `json:"expect_status"`
	Icon         string `json:"icon"`
	Position     int    `json:"position"`
	Enabled      bool   `json:"enabled"`
}

// Device is a target plus the result of the last health check.
type Device struct {
	Target
	Status    string    `json:"status"`
	LatencyMs int       `json:"latency_ms"`
	LastCheck time.Time `json:"last_check"`
	Error     string    `json:"error,omitempty"`
}

type Service struct {
	checkInterval time.Duration
	timeout       time.Duration
	db            *sql.DB
	client        *http.Client

	// recheck lets a configuration change take effect at once instead of
	// leaving a stale tile up for the rest of the interval.
	recheck chan struct{}

	mu      sync.RWMutex
	results []Device
}

const deviceColumns = `id, name, type, url, link, host, port, expect_status, icon, position, enabled`

func NewService(db *sql.DB, checkInterval, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Service{
		checkInterval: checkInterval,
		timeout:       timeout,
		db:            db,
		client: &http.Client{
			Timeout: timeout,
			// Health checks must never follow a redirect into another service.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		recheck: make(chan struct{}, 1),
		results: []Device{},
	}
}

// Seed copies the config.yaml entries into the database the first time the
// dashboard runs. After that the admin area is the only source.
func (s *Service) Seed(targets []config.DeviceTarget) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&count); err != nil {
		return err
	}
	if count > 0 || len(targets) == 0 {
		return nil
	}

	for i, t := range targets {
		if _, err := s.db.Exec(`
			INSERT INTO devices (name, type, url, link, host, port, expect_status, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			t.Name, t.Type, t.URL, t.Link, t.Host, t.Port, t.ExpectStatus, i); err != nil {
			return err
		}
	}
	log.Info().Int("count", len(targets)).Msg("Seeded devices from config.yaml")
	return nil
}

func (s *Service) targets(onlyEnabled bool) ([]Target, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices`
	if onlyEnabled {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY position, id`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := []Target{}
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.URL, &t.Link, &t.Host,
			&t.Port, &t.ExpectStatus, &t.Icon, &t.Position, &t.Enabled); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (s *Service) Start(ctx context.Context) {
	s.checkAll(ctx)

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAll(ctx)
		case <-s.recheck:
			s.checkAll(ctx)
		}
	}
}

func (s *Service) triggerRecheck() {
	select {
	case s.recheck <- struct{}{}:
	default:
	}
}

func (s *Service) checkAll(ctx context.Context) {
	targets, err := s.targets(true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load devices")
		return
	}

	results := make([]Device, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t Target) {
			defer wg.Done()
			results[idx] = s.check(ctx, t)
		}(i, target)
	}
	wg.Wait()

	s.mu.Lock()
	s.results = results
	s.mu.Unlock()

	s.persist(results)
}

func (s *Service) check(ctx context.Context, target Target) Device {
	start := time.Now()
	d := Device{Target: target, LastCheck: start}

	switch target.Type {
	case "http", "https":
		d.Status, d.Error = s.checkHTTP(ctx, target)
	case "tcp":
		d.Status, d.Error = s.checkTCP(ctx, target)
	default:
		d.Status, d.Error = "unknown", fmt.Sprintf("unbekannter Typ %q", target.Type)
	}

	d.LatencyMs = int(time.Since(start).Milliseconds())
	return d
}

func (s *Service) checkHTTP(ctx context.Context, target Target) (string, string) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return "down", err.Error()
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "down", friendlyError(err)
	}
	defer resp.Body.Close()

	if target.ExpectStatus > 0 && resp.StatusCode != target.ExpectStatus {
		return "down", fmt.Sprintf("HTTP %d (erwartet %d)", resp.StatusCode, target.ExpectStatus)
	}
	// Without an explicit expectation, anything the server answers counts as
	// reachable — several of these endpoints reply 401 by design.
	if target.ExpectStatus == 0 && resp.StatusCode >= 500 {
		return "down", fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return "up", ""
}

func (s *Service) checkTCP(ctx context.Context, target Target) (string, string) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, strconv.Itoa(target.Port)))
	if err != nil {
		return "down", friendlyError(err)
	}
	_ = conn.Close()
	return "up", ""
}

// friendlyError trims Go's network errors down to something a family member
// can act on.
func friendlyError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "context deadline exceeded"), strings.Contains(msg, "timeout"):
		return "Zeitüberschreitung"
	case strings.Contains(msg, "connection refused"):
		return "Verbindung abgelehnt"
	case strings.Contains(msg, "no such host"):
		return "Adresse nicht gefunden"
	case strings.Contains(msg, "no route to host"), strings.Contains(msg, "network is unreachable"):
		return "Nicht erreichbar"
	}
	return msg
}

// persist keeps one row per device so the table stays small.
func (s *Service) persist(results []Device) {
	for _, d := range results {
		_, err := s.db.Exec(`
			INSERT INTO device_status (name, type, status, latency_ms, last_check, error)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				type = excluded.type, status = excluded.status,
				latency_ms = excluded.latency_ms, last_check = excluded.last_check,
				error = excluded.error`,
			d.Name, d.Type, d.Status, d.LatencyMs, d.LastCheck, d.Error)
		if err != nil {
			log.Error().Err(err).Str("device", d.Name).Msg("Failed to persist device status")
		}
	}
}

// ------------------------------------------------------------------ Handlers

func (s *Service) GetStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	results := make([]Device, len(s.results))
	copy(results, s.results)
	s.mu.RUnlock()

	auth.WriteJSON(w, results)
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	targets, err := s.targets(false)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, targets)
}

type devicePayload struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	Link         string `json:"link"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ExpectStatus int    `json:"expect_status"`
	Icon         string `json:"icon"`
	Enabled      *bool  `json:"enabled"`
	Position     *int   `json:"position"`
}

// validate rejects the mistakes that would otherwise show up as a permanently
// red tile with a cryptic Go error underneath it.
func (p *devicePayload) validate() error {
	p.Name = strings.TrimSpace(p.Name)
	p.URL = strings.TrimSpace(p.URL)
	p.Link = strings.TrimSpace(p.Link)
	p.Host = strings.TrimSpace(p.Host)

	if p.Name == "" {
		return fmt.Errorf("Name fehlt")
	}
	if p.Type != "tcp" {
		p.Type = "http"
	}

	if p.Type == "http" {
		if p.URL == "" {
			return fmt.Errorf("Adresse für die Prüfung fehlt")
		}
		if err := checkURL(p.URL); err != nil {
			return fmt.Errorf("Prüf-Adresse: %w", err)
		}
		if p.ExpectStatus != 0 && (p.ExpectStatus < 100 || p.ExpectStatus > 599) {
			return fmt.Errorf("Erwarteter Status muss zwischen 100 und 599 liegen")
		}
	} else {
		if p.Host == "" {
			return fmt.Errorf("Host fehlt")
		}
		if p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("Port muss zwischen 1 und 65535 liegen")
		}
	}

	if p.Link != "" {
		if err := checkURL(p.Link); err != nil {
			return fmt.Errorf("Link zur Oberfläche: %w", err)
		}
	}
	return nil
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("keine gültige Adresse")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("muss mit http:// oder https:// beginnen")
	}
	if u.Host == "" {
		return fmt.Errorf("keine Adresse angegeben")
	}
	return nil
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	var req devicePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := req.validate(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	var maxPos sql.NullInt64
	_ = s.db.QueryRow("SELECT MAX(position) FROM devices").Scan(&maxPos)

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	res, err := s.db.Exec(`
		INSERT INTO devices (name, type, url, link, host, port, expect_status, icon, position, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Type, req.URL, req.Link, req.Host, req.Port,
		req.ExpectStatus, req.Icon, int(maxPos.Int64)+1, enabled)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create device")
		auth.HTTPError(w, http.StatusInternalServerError, "Gerät konnte nicht gespeichert werden")
		return
	}

	id, _ := res.LastInsertId()
	s.triggerRecheck()
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]int64{"id": id})
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var req devicePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := req.validate(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if _, err := s.db.Exec(`
		UPDATE devices SET name = ?, type = ?, url = ?, link = ?, host = ?, port = ?,
			expect_status = ?, icon = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		req.Name, req.Type, req.URL, req.Link, req.Host, req.Port,
		req.ExpectStatus, req.Icon, enabled, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Gerät konnte nicht gespeichert werden")
		return
	}

	s.triggerRecheck()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var name string
	_ = s.db.QueryRow("SELECT name FROM devices WHERE id = ?", id).Scan(&name)

	if _, err := s.db.Exec("DELETE FROM devices WHERE id = ?", id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// The status row is keyed by name, so it would otherwise linger forever.
	if name != "" {
		_, _ = s.db.Exec("DELETE FROM device_status WHERE name = ?", name)
	}

	s.triggerRecheck()
	w.WriteHeader(http.StatusNoContent)
}

// Reorder accepts the ids in their new order.
func (s *Service) Reorder(w http.ResponseWriter, r *http.Request) {
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

	for position, id := range req.IDs {
		if _, err := tx.Exec("UPDATE devices SET position = ? WHERE id = ?", position, id); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	s.triggerRecheck()
	w.WriteHeader(http.StatusNoContent)
}

// Test probes a device without saving it, so the admin form can say whether the
// address works before anyone commits to it.
func (s *Service) Test(w http.ResponseWriter, r *http.Request) {
	var req devicePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := req.validate(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := s.check(r.Context(), Target{
		Name: req.Name, Type: req.Type, URL: req.URL,
		Host: req.Host, Port: req.Port, ExpectStatus: req.ExpectStatus,
	})
	auth.WriteJSON(w, map[string]any{
		"status":     result.Status,
		"latency_ms": result.LatencyMs,
		"error":      result.Error,
	})
}
