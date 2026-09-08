package weather

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/rs/zerolog/log"
)

// SettingsStore is the slice of the store this service needs. The chosen
// location is family-wide state, so it belongs in the database rather than in
// config.yaml, which nobody can edit from a phone.
type SettingsStore interface {
	Setting(key string) (string, bool, error)
	SetSetting(key, value string) error
}

const locationKey = "weather.location"

type Location struct {
	Name      string  `json:"name"`
	Region    string  `json:"region,omitempty"`
	Country   string  `json:"country,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

func (l Location) label() string {
	parts := []string{l.Name}
	if l.Region != "" && l.Region != l.Name {
		parts = append(parts, l.Region)
	}
	if l.Country != "" {
		parts = append(parts, l.Country)
	}
	return strings.Join(parts, ", ")
}

const forecastDays = 5

type apiResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		Apparent    float64 `json:"apparent_temperature"`
		Humidity    int     `json:"relative_humidity_2m"`
		WeatherCode int     `json:"weather_code"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		IsDay       int     `json:"is_day"`
		Time        string  `json:"time"`
	} `json:"current"`
	Daily struct {
		Time        []string  `json:"time"`
		WeatherCode []int     `json:"weather_code"`
		TempMax     []float64 `json:"temperature_2m_max"`
		TempMin     []float64 `json:"temperature_2m_min"`
		PrecipProb  []int     `json:"precipitation_probability_max"`
		Sunrise     []string  `json:"sunrise"`
		Sunset      []string  `json:"sunset"`
	} `json:"daily"`
}

type Data struct {
	Current  Current   `json:"current"`
	Forecast []Daily   `json:"forecast"`
	Location Location  `json:"location"`
	Updated  time.Time `json:"updated"`
	Stale    bool      `json:"stale"`
}

type Current struct {
	Temperature float64 `json:"temperature"`
	FeelsLike   float64 `json:"feels_like"`
	Humidity    int     `json:"humidity"`
	WeatherCode int     `json:"weather_code"`
	WindSpeed   float64 `json:"wind_speed"`
	IsDay       bool    `json:"is_day"`
	Icon        string  `json:"icon"`
	Description string  `json:"description"`
}

type Daily struct {
	Date              string  `json:"date"`
	WeatherCode       int     `json:"weather_code"`
	TempMax           float64 `json:"temp_max"`
	TempMin           float64 `json:"temp_min"`
	PrecipProbability int     `json:"precip_probability"`
	Sunrise           string  `json:"sunrise"`
	Sunset            string  `json:"sunset"`
	Icon              string  `json:"icon"`
	Description       string  `json:"description"`
}

type Service struct {
	fallback Location
	cacheTTL time.Duration
	db       *sql.DB
	settings SettingsStore
	client   *http.Client

	// refresh lets a location change trigger an immediate refetch instead of
	// waiting for the next tick.
	refresh chan struct{}

	mu       sync.RWMutex
	cached   *Data
	location Location
}

func NewService(db *sql.DB, settings SettingsStore, lat, lon float64, timezone string, cacheTTL time.Duration) *Service {
	fallback := Location{
		Name:      "Zuhause",
		Latitude:  lat,
		Longitude: lon,
		Timezone:  timezone,
	}
	return &Service{
		fallback: fallback,
		location: fallback,
		cacheTTL: cacheTTL,
		db:       db,
		settings: settings,
		client:   &http.Client{Timeout: 15 * time.Second},
		refresh:  make(chan struct{}, 1),
	}
}

// currentLocation prefers what the family picked in the UI and falls back to
// the coordinates from config.yaml / the environment.
func (s *Service) currentLocation() Location {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.location
}

func (s *Service) loadLocation() {
	if s.settings == nil {
		return
	}
	raw, ok, err := s.settings.Setting(locationKey)
	if err != nil || !ok {
		return
	}
	var loc Location
	if err := json.Unmarshal([]byte(raw), &loc); err != nil || loc.Timezone == "" {
		return
	}

	s.mu.Lock()
	s.location = loc
	s.mu.Unlock()
	log.Info().Str("location", loc.label()).Msg("Weather location restored")
}

func (s *Service) Start(ctx context.Context) {
	s.loadLocation()
	// Serve whatever the last run persisted until the first fetch lands, so a
	// restart without internet still shows something.
	s.loadFromDB()

	s.fetch(ctx)
	ticker := time.NewTicker(s.cacheTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetch(ctx)
		case <-s.refresh:
			s.fetch(ctx)
		}
	}
}

func (s *Service) endpoint(loc Location) string {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.4f", loc.Latitude))
	q.Set("longitude", fmt.Sprintf("%.4f", loc.Longitude))
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m,is_day")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset")
	q.Set("timezone", loc.Timezone)
	q.Set("forecast_days", fmt.Sprint(forecastDays))
	return "https://api.open-meteo.com/v1/forecast?" + q.Encode()
}

func (s *Service) fetch(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	loc := s.currentLocation()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint(loc), nil)
	if err != nil {
		log.Error().Err(err).Msg("Weather request build failed")
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("Weather API unreachable, keeping cached data")
		s.markStale()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("Weather API returned an error, keeping cached data")
		s.markStale()
		return
	}

	var api apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		log.Error().Err(err).Msg("Weather API decode failed")
		s.markStale()
		return
	}

	data := Data{
		Current: Current{
			Temperature: api.Current.Temperature,
			FeelsLike:   api.Current.Apparent,
			Humidity:    api.Current.Humidity,
			WeatherCode: api.Current.WeatherCode,
			WindSpeed:   api.Current.WindSpeed,
			IsDay:       api.Current.IsDay == 1,
			Icon:        CodeToIcon(api.Current.WeatherCode),
			Description: CodeToDescription(api.Current.WeatherCode),
		},
		Forecast: make([]Daily, 0, len(api.Daily.Time)),
		Location: loc,
		Updated:  time.Now(),
	}

	for i := range api.Daily.Time {
		if i >= forecastDays {
			break
		}
		d := Daily{
			Date:              api.Daily.Time[i],
			Icon:              CodeToIcon(at(api.Daily.WeatherCode, i)),
			Description:       CodeToDescription(at(api.Daily.WeatherCode, i)),
			WeatherCode:       at(api.Daily.WeatherCode, i),
			TempMax:           at(api.Daily.TempMax, i),
			TempMin:           at(api.Daily.TempMin, i),
			PrecipProbability: at(api.Daily.PrecipProb, i),
			Sunrise:           atStr(api.Daily.Sunrise, i),
			Sunset:            atStr(api.Daily.Sunset, i),
		}
		data.Forecast = append(data.Forecast, d)
	}

	s.mu.Lock()
	s.cached = &data
	s.mu.Unlock()

	s.saveToDB(&data)
	log.Debug().Float64("temp", data.Current.Temperature).Msg("Weather updated")
}

// at / atStr guard against Open-Meteo returning arrays of differing lengths.
func at[T any](s []T, i int) T {
	var zero T
	if i < len(s) {
		return s[i]
	}
	return zero
}

func atStr(s []string, i int) string { return at(s, i) }

func (s *Service) markStale() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil {
		s.cached.Stale = true
	}
}

func (s *Service) saveToDB(d *Data) {
	if s.db == nil {
		return
	}
	payload, err := json.Marshal(d)
	if err != nil {
		return
	}
	if _, err := s.db.Exec(`
		INSERT INTO weather_cache (id, payload, fetched_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET payload = excluded.payload, fetched_at = excluded.fetched_at`,
		string(payload), d.Updated); err != nil {
		log.Error().Err(err).Msg("Failed to persist weather cache")
	}
}

func (s *Service) loadFromDB() {
	if s.db == nil {
		return
	}
	var payload string
	if err := s.db.QueryRow("SELECT payload FROM weather_cache WHERE id = 1").Scan(&payload); err != nil {
		return
	}
	var d Data
	if err := json.Unmarshal([]byte(payload), &d); err != nil {
		return
	}
	d.Stale = true

	s.mu.Lock()
	s.cached = &d
	s.mu.Unlock()
	log.Info().Time("updated", d.Updated).Msg("Restored weather from cache")
}

func (s *Service) GetWeather(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	data := s.cached
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if data == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Wetterdaten noch nicht verfügbar",
		})
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

// ------------------------------------------------------- Standort-Endpunkte

// SearchLocations proxies Open-Meteo's geocoding API. Going through the backend
// keeps the browser off the internet and avoids a CORS round trip.
func (s *Service) SearchLocations(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < 2 {
		auth.WriteJSON(w, []Location{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	q := url.Values{}
	q.Set("name", query)
	q.Set("count", "8")
	q.Set("language", "de")
	q.Set("format", "json")
	endpoint := "https://geocoding-api.open-meteo.com/v1/search?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("Geocoding request failed")
		auth.HTTPError(w, http.StatusServiceUnavailable,
			"Ortssuche nicht erreichbar - braucht kurz Internet")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Ortssuche antwortet nicht")
		return
	}

	var payload struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Timezone  string  `json:"timezone"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		auth.HTTPError(w, http.StatusBadGateway, "Ortssuche lieferte ungültige Daten")
		return
	}

	results := make([]Location, 0, len(payload.Results))
	for _, hit := range payload.Results {
		if hit.Timezone == "" {
			hit.Timezone = "Europe/Vienna"
		}
		results = append(results, Location{
			Name:      hit.Name,
			Region:    hit.Admin1,
			Country:   hit.Country,
			Latitude:  hit.Latitude,
			Longitude: hit.Longitude,
			Timezone:  hit.Timezone,
		})
	}
	auth.WriteJSON(w, results)
}

func (s *Service) GetLocation(w http.ResponseWriter, r *http.Request) {
	auth.WriteJSON(w, s.currentLocation())
}

// SetLocation stores the family's location and refetches straight away, so the
// widget updates without waiting for the next refresh tick.
func (s *Service) SetLocation(w http.ResponseWriter, r *http.Request) {
	var loc Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	loc.Name = strings.TrimSpace(loc.Name)
	if loc.Name == "" {
		auth.HTTPError(w, http.StatusBadRequest, "Name des Ortes fehlt")
		return
	}
	if loc.Latitude < -90 || loc.Latitude > 90 {
		auth.HTTPError(w, http.StatusBadRequest, "Breitengrad muss zwischen -90 und 90 liegen")
		return
	}
	if loc.Longitude < -180 || loc.Longitude > 180 {
		auth.HTTPError(w, http.StatusBadRequest, "Längengrad muss zwischen -180 und 180 liegen")
		return
	}
	if strings.TrimSpace(loc.Timezone) == "" {
		loc.Timezone = s.fallback.Timezone
	}
	if _, err := time.LoadLocation(loc.Timezone); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Unbekannte Zeitzone: "+loc.Timezone)
		return
	}

	encoded, err := json.Marshal(loc)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if s.settings != nil {
		if err := s.settings.SetSetting(locationKey, string(encoded)); err != nil {
			log.Error().Err(err).Msg("Failed to persist weather location")
			auth.HTTPError(w, http.StatusInternalServerError, "Standort konnte nicht gespeichert werden")
			return
		}
	}

	s.mu.Lock()
	s.location = loc
	// Drop the cache: it belongs to the old place and would otherwise show
	// yesterday's weather for a town 500 km away.
	s.cached = nil
	s.mu.Unlock()

	select {
	case s.refresh <- struct{}{}:
	default:
	}

	log.Info().Str("location", loc.label()).Msg("Weather location changed")
	auth.WriteJSON(w, loc)
}

func CodeToIcon(code int) string {
	switch {
	case code == 0:
		return "sun"
	case code == 1:
		return "cloud-sun"
	case code == 2 || code == 3:
		return "cloud"
	case code == 45 || code == 48:
		return "cloud-fog"
	case code >= 51 && code <= 57:
		return "cloud-drizzle"
	case code >= 61 && code <= 67, code >= 80 && code <= 82:
		return "cloud-rain"
	case code >= 71 && code <= 77, code == 85 || code == 86:
		return "cloud-snow"
	case code >= 95:
		return "cloud-lightning"
	default:
		return "cloud"
	}
}

func CodeToDescription(code int) string {
	descriptions := map[int]string{
		0: "Klar", 1: "Überwiegend klar", 2: "Teilweise bewölkt", 3: "Bewölkt",
		45: "Neblig", 48: "Nebel mit Raureif",
		51: "Leichter Nieselregen", 53: "Mäßiger Nieselregen", 55: "Starker Nieselregen",
		56: "Gefrierender Nieselregen", 57: "Starker gefrierender Nieselregen",
		61: "Leichter Regen", 63: "Mäßiger Regen", 65: "Starker Regen",
		66: "Gefrierender Regen", 67: "Starker gefrierender Regen",
		71: "Leichter Schneefall", 73: "Mäßiger Schneefall", 75: "Starker Schneefall",
		77: "Schneegriesel",
		80: "Leichte Regenschauer", 81: "Mäßige Regenschauer", 82: "Starke Regenschauer",
		85: "Leichte Schneeschauer", 86: "Starke Schneeschauer",
		95: "Gewitter", 96: "Gewitter mit leichtem Hagel", 99: "Gewitter mit starkem Hagel",
	}
	if d, ok := descriptions[code]; ok {
		return d
	}
	return "Unbekannt"
}
