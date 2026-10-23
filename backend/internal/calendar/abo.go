package calendar

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/emersion/go-ical"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Kalender-Abos: Das Dashboard holt einen Kalender (Schule, Müllabfuhr,
// Vereinsplan) von einer Adresse und zeigt ihn neben den eigenen Terminen.
//
// Das ist die einzige Stelle, an der der Server auf Wunsch einer Person eine
// fremde Adresse aufruft. Ohne Vorkehrungen wäre er damit ein Werkzeug, das
// das Heimnetz abklopft (Router, Plex, der Pi selbst). Deshalb gilt:
//
//   - nur http und https, keine Zugangsdaten in der Adresse,
//   - gesperrt wird die IP, die tatsächlich angewählt wird — nicht der Name,
//     den jemand so wählen kann, dass er erst beim Abruf ins Heimnetz zeigt,
//   - Weiterleitungen, Zeit und Größe sind begrenzt,
//   - die Antwort muss ein Kalender sein, sonst wird sie verworfen,
//   - nur Administratoren legen Abos an.
const (
	aboMaxBytes         = 5 << 20
	aboZeitlimit        = 15 * time.Second
	aboMaxWeiterleitung = 3
	aboMaxAnzahl        = 10
	aboIntervall        = 6 * time.Hour
	aboErsterAbruf      = 20 * time.Second
)

// Meldungen, die bei der Person ankommen. Bewusst feste Texte: Sie lassen sich
// übersetzen, und die Einzelheiten stehen im Log, nicht in der Oberfläche.
var (
	errAdresseFehlt     = errors.New("Adresse fehlt")
	errAdresseUngueltig = errors.New("Die Adresse ist ungültig")
	errAdresseZuLang    = errors.New("Die Adresse ist zu lang")
	errAdresseSchema    = errors.New("Die Adresse muss mit http:// oder https:// beginnen")
	errAdresseZugang    = errors.New("Zugangsdaten gehören nicht in die Adresse")
	errGesperrt         = errors.New("Die Adresse zeigt ins Heimnetz oder ist nicht öffentlich")
	errWeiterleitung    = errors.New("Der Kalender leitet zu oft weiter")
	errZuGross          = errors.New("Der Kalender ist größer als 5 MB")
	errKeinKalender     = errors.New("Die Adresse liefert keinen Kalender")
	errServerFehler     = errors.New("Der Kalender antwortet mit einem Fehler")
	errNichtErreichbar  = errors.New("Der Kalender ist nicht erreichbar")
)

// ---------------------------------------------------------------- Adresse

// adresseBereinigen macht aus der Eingabe eine Adresse, die der Server
// aufrufen darf. webcal:// ist die Schreibweise der Kalender-Apps für https.
func adresseBereinigen(roh string) (string, error) {
	roh = strings.TrimSpace(roh)
	if roh == "" {
		return "", errAdresseFehlt
	}
	if len(roh) > 2000 {
		return "", errAdresseZuLang
	}

	kleinbuchstaben := strings.ToLower(roh)
	switch {
	case strings.HasPrefix(kleinbuchstaben, "webcals://"):
		roh = "https://" + roh[len("webcals://"):]
	case strings.HasPrefix(kleinbuchstaben, "webcal://"):
		roh = "https://" + roh[len("webcal://"):]
	}

	u, err := url.Parse(roh)
	if err != nil {
		return "", errAdresseUngueltig
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errAdresseSchema
	}
	if u.User != nil {
		return "", errAdresseZugang
	}
	if u.Hostname() == "" {
		return "", errAdresseUngueltig
	}
	u.Fragment = ""
	return u.String(), nil
}

// ------------------------------------------------------------- Sperrliste

var gesperrteBereiche = mustPrefixe(
	// IPv4
	"0.0.0.0/8",       // „dieses Netz"
	"100.64.0.0/10",   // CGNAT, auch Tailscale
	"192.0.0.0/24",    // IETF-Protokollzuweisungen
	"192.0.2.0/24",    // Dokumentation
	"192.88.99.0/24",  // 6to4-Relay
	"198.18.0.0/15",   // Messnetze
	"198.51.100.0/24", // Dokumentation
	"203.0.113.0/24",  // Dokumentation
	"240.0.0.0/4",     // reserviert, samt Broadcast
	// IPv6
	"64:ff9b::/96",   // NAT64: steckt eine IPv4-Adresse, auch eine private
	"64:ff9b:1::/48", // NAT64, lokal
	"100::/64",       // verwerfen
	"2001::/32",      // Teredo
	"2001:db8::/32",  // Dokumentation
	"2002::/16",      // 6to4: steckt eine IPv4-Adresse
)

func mustPrefixe(liste ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(liste))
	for _, s := range liste {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

// erlaubteAdresse sagt, ob der Server diese IP anwählen darf: nur öffentliche
// Adressen. Eine als IPv6 verpackte IPv4-Adresse (::ffff:127.0.0.1) wird als
// das behandelt, was sie ist.
func erlaubteAdresse(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	for _, p := range gesperrteBereiche {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// sichererClient liefert einen HTTP-Client, der nur öffentliche Adressen
// anwählt. Die Prüfung sitzt im Dialer, nach der Namensauflösung und vor dem
// Verbindungsaufbau: Sie sieht die IP, die wirklich benutzt wird, und gilt
// damit auch für jede Weiterleitung. Eine Umgebungsvariable wie HTTP_PROXY
// wird nicht beachtet.
func sichererClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || !erlaubteAdresse(ap.Addr()) {
				return errGesperrt
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: aboZeitlimit,
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            dialer.DialContext,
			TLSHandshakeTimeout:    10 * time.Second,
			ResponseHeaderTimeout:  aboZeitlimit,
			DisableKeepAlives:      true,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > aboMaxWeiterleitung {
				return errWeiterleitung
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errAdresseSchema
			}
			if req.URL.User != nil {
				return errAdresseZugang
			}
			return nil
		},
	}
}

// -------------------------------------------------------------- Abrufen

// kalenderHolen lädt den Kalender und prüft, dass es einer ist.
func kalenderHolen(ctx context.Context, client *http.Client, adresse string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, aboZeitlimit)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adresse, nil)
	if err != nil {
		return nil, errAdresseUngueltig
	}
	req.Header.Set("User-Agent", "FamilienDashboard (Kalender-Abo)")
	req.Header.Set("Accept", "text/calendar, text/plain;q=0.5, */*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		switch {
		case errors.Is(err, errGesperrt):
			return nil, errGesperrt
		case errors.Is(err, errWeiterleitung):
			return nil, errWeiterleitung
		case errors.Is(err, errAdresseSchema), errors.Is(err, errAdresseZugang):
			return nil, errAdresseUngueltig
		}
		log.Debug().Err(err).Msg("Calendar subscription fetch failed")
		return nil, errNichtErreichbar
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("Calendar subscription: unexpected status")
		return nil, errServerFehler
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, aboMaxBytes+1))
	if err != nil {
		log.Debug().Err(err).Msg("Calendar subscription read failed")
		return nil, errNichtErreichbar
	}
	if len(data) > aboMaxBytes {
		return nil, errZuGross
	}
	if !bytes.Contains(bytes.ToUpper(data), []byte("BEGIN:VCALENDAR")) {
		return nil, errKeinKalender
	}
	if _, err := ical.NewDecoder(bytes.NewReader(data)).Decode(); err != nil {
		log.Debug().Err(err).Msg("Calendar subscription does not parse")
		return nil, errKeinKalender
	}
	return data, nil
}

// ------------------------------------------------------------ Speicherung

// Subscription ist ein abonnierter Kalender.
type Subscription struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	Color     string     `json:"color"`
	LastOK    *time.Time `json:"last_ok"`
	LastError string     `json:"last_error"`
}

// aboDatei ist die letzte gute Fassung. Sie liegt neben dem ics-Ordner und
// nicht darin: Dort würde die Ordnerüberwachung sie als eigenen Kalender
// lesen, und ein Abo hätte seine Termine doppelt.
func (s *Service) aboDatei(id int) string {
	return filepath.Join(s.aboDir, strconv.Itoa(id)+".ics")
}

// dateiSchreiben legt die Datei atomar ab: Ein Neustart mitten im Schreiben
// darf nie eine halbe Datei hinterlassen.
func dateiSchreiben(pfad string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(pfad), 0o755); err != nil {
		return err
	}
	tmp := pfad + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, pfad); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Service) abos() ([]Subscription, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT id, name, url, color, last_ok, last_error FROM calendar_subscriptions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var a Subscription
		var ok sql.NullTime
		if err := rows.Scan(&a.ID, &a.Name, &a.URL, &a.Color, &ok, &a.LastError); err != nil {
			return nil, err
		}
		if ok.Valid {
			t := ok.Time
			a.LastOK = &t
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// aboEvents liest die gespeicherten Fassungen aller Abos. Die Termine tragen
// das Präfix abo<id>-, damit sich ihre IDs nicht mit denen der .ics-Dateien
// beißen.
func (s *Service) aboEvents() []Event {
	liste, err := s.abos()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load calendar subscriptions")
		return nil
	}
	var all []Event
	for _, a := range liste {
		events, err := s.parseFile(s.aboDatei(a.ID), a.Name, a.Color)
		if err != nil {
			// Noch nie erfolgreich geholt oder Datei weg: kein Grund zur Sorge.
			log.Debug().Err(err).Int("id", a.ID).Msg("Calendar subscription has no readable copy")
			continue
		}
		for i := range events {
			events[i].ID = fmt.Sprintf("abo%d-%s", a.ID, events[i].ID)
		}
		all = append(all, events...)
	}
	return all
}

// aboAktualisieren holt ein Abo und hält das Ergebnis fest. Schlägt der Abruf
// fehl, bleibt die letzte gute Fassung in Kraft.
func (s *Service) aboAktualisieren(ctx context.Context, a Subscription) error {
	data, err := kalenderHolen(ctx, s.aboClient(), a.URL)
	if err != nil {
		if _, dberr := s.db.Exec(`UPDATE calendar_subscriptions SET last_error = ? WHERE id = ?`, err.Error(), a.ID); dberr != nil {
			log.Error().Err(dberr).Msg("Failed to record subscription error")
		}
		return err
	}
	if err := dateiSchreiben(s.aboDatei(a.ID), data); err != nil {
		log.Error().Err(err).Int("id", a.ID).Msg("Failed to store calendar subscription")
		return errors.New("Der Kalender konnte nicht gespeichert werden")
	}
	if _, err := s.db.Exec(`UPDATE calendar_subscriptions SET last_ok = ?, last_error = '' WHERE id = ?`, time.Now(), a.ID); err != nil {
		log.Error().Err(err).Msg("Failed to record subscription success")
	}
	return nil
}

func (s *Service) aboAlleAktualisieren(ctx context.Context) {
	liste, err := s.abos()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list calendar subscriptions")
		return
	}
	for _, a := range liste {
		if ctx.Err() != nil {
			return
		}
		if err := s.aboAktualisieren(ctx, a); err != nil {
			log.Warn().Err(err).Int("id", a.ID).Str("name", a.Name).Msg("Calendar subscription not updated")
		}
	}
	if len(liste) > 0 {
		s.parseAll()
	}
}

// aboLoop hält die Abos aktuell: kurz nach dem Start, dann alle sechs Stunden.
func (s *Service) aboLoop(ctx context.Context) {
	if s.db == nil {
		return
	}
	wait := aboErsterAbruf
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		s.aboAlleAktualisieren(ctx)
		wait = aboIntervall
	}
}

func (s *Service) aboClient() *http.Client {
	if s.clientFabrik != nil {
		return s.clientFabrik()
	}
	return sichererClient()
}

// --------------------------------------------------------------- Handler

var farbmuster = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Service) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	liste, err := s.abos()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if liste == nil {
		liste = []Subscription{}
	}
	auth.WriteJSON(w, liste)
}

func (s *Service) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Termine sind nicht verfügbar")
		return
	}
	var req struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 60 {
		auth.HTTPError(w, http.StatusBadRequest, "Name fehlt oder ist zu lang")
		return
	}
	color := req.Color
	if color == "" {
		color = "#6366f1"
	}
	if !farbmuster.MatchString(color) {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Farbe")
		return
	}
	adresse, err := adresseBereinigen(req.URL)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	var anzahl int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM calendar_subscriptions`).Scan(&anzahl); err == nil && anzahl >= aboMaxAnzahl {
		auth.HTTPError(w, http.StatusBadRequest, "Es sind höchstens 10 Kalender-Abos möglich")
		return
	}

	// Erst holen, dann eintragen: Ein Abo, das nicht funktioniert, soll gar
	// nicht erst in der Liste stehen.
	data, err := kalenderHolen(r.Context(), s.aboClient(), adresse)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := s.db.Exec(
		`INSERT INTO calendar_subscriptions (name, url, color, last_ok) VALUES (?, ?, ?, ?)`,
		name, adresse, color, time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create calendar subscription")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	id64, _ := res.LastInsertId()
	id := int(id64)

	if err := dateiSchreiben(s.aboDatei(id), data); err != nil {
		log.Error().Err(err).Msg("Failed to store calendar subscription")
		_, _ = s.db.Exec(`DELETE FROM calendar_subscriptions WHERE id = ?`, id)
		auth.HTTPError(w, http.StatusInternalServerError, "Der Kalender konnte nicht gespeichert werden")
		return
	}
	s.parseAll()

	liste, _ := s.abos()
	for _, a := range liste {
		if a.ID == id {
			w.WriteHeader(http.StatusCreated)
			auth.WriteJSON(w, a)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, Subscription{ID: id, Name: name, URL: adresse, Color: color})
}

func (s *Service) RefreshSubscription(w http.ResponseWriter, r *http.Request) {
	a, ok := s.aboAusAnfrage(w, r)
	if !ok {
		return
	}
	if err := s.aboAktualisieren(r.Context(), a); err != nil {
		auth.HTTPError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.parseAll()
	liste, _ := s.abos()
	for _, neu := range liste {
		if neu.ID == a.ID {
			auth.WriteJSON(w, neu)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	a, ok := s.aboAusAnfrage(w, r)
	if !ok {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM calendar_subscriptions WHERE id = ?`, a.ID); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	_ = os.Remove(s.aboDatei(a.ID))
	s.parseAll()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) aboAusAnfrage(w http.ResponseWriter, r *http.Request) (Subscription, bool) {
	if s.db == nil {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Termine sind nicht verfügbar")
		return Subscription{}, false
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return Subscription{}, false
	}
	liste, err := s.abos()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return Subscription{}, false
	}
	for _, a := range liste {
		if a.ID == id {
			return a, true
		}
	}
	auth.HTTPError(w, http.StatusNotFound, "Abo nicht gefunden")
	return Subscription{}, false
}
