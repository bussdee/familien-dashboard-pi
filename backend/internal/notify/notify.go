// Package notify schickt der Familie eine Nachricht aufs Handy, wenn etwas
// auf ein Elternteil wartet: eine Aufgabe zum Bestätigen, ein neuer Wunsch,
// eine Einlösung.
//
// Gesendet wird an einen ntfy-Server (https://ntfy.sh, selbst betrieben).
// Standardmäßig ist das aus: Ohne notify.ntfy_url in der config.yaml (oder
// NTFY_URL in der .env) geht nichts nach draußen.
//
// Zwei Regeln halten das harmlos:
//   - Nie blockieren. Eine Nachricht wandert in einen Puffer und wird von
//     einem eigenen Prozess verschickt. Ist ntfy nicht erreichbar, merkt
//     das nur das Log.
//   - Nie scheitern lassen. Eine Aufgabe wird auch dann abgehakt, eine
//     Einlösung auch dann gebucht, wenn die Nachricht nicht ankommt.
package notify

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/rs/zerolog/log"
)

// Message ist eine Nachricht. Priority 1 (leise) bis 5 (dringend), 0 = Standard.
type Message struct {
	Title    string
	Body     string
	Tags     []string
	Priority int
}

// Notifier ist, was die Dienste kennen. Sie wissen nichts von ntfy.
type Notifier interface {
	Notify(Message)
}

// Aus tut nichts. Es ist die Voreinstellung in jedem Dienst, damit niemand
// auf nil prüfen muss.
type Aus struct{}

func (Aus) Notify(Message) {}

const (
	puffer      = 32
	zeitlimit   = 5 * time.Second
	maxBodyRead = 4 << 10
)

var (
	errNichtEingerichtet = errors.New("Benachrichtigungen sind nicht eingerichtet")
	errNichtErreichbar   = errors.New("Der ntfy-Server ist nicht erreichbar")
	errAbgelehnt         = errors.New("Der ntfy-Server hat die Nachricht abgelehnt")
)

var themaMuster = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Service verschickt Nachrichten an ntfy.
type Service struct {
	basis  string // https://ntfy.example.com
	thema  string // familie
	token  string
	client *http.Client
	queue  chan Message
}

// Neu liest die Adresse (https://ntfy.example.com/familie) und das Token.
// Eine leere Adresse ergibt einen ausgeschalteten Dienst; eine kaputte einen
// Fehler, den main beim Start meldet — und dann trotzdem ohne Nachrichten
// weiterläuft.
func Neu(adresse, token string) (*Service, error) {
	s := &Service{
		token:  strings.TrimSpace(token),
		client: &http.Client{Timeout: zeitlimit, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}},
		queue:  make(chan Message, puffer),
	}
	adresse = strings.TrimSpace(adresse)
	if adresse == "" {
		return s, nil
	}
	basis, thema, err := adresseTeilen(adresse)
	if err != nil {
		return s, err
	}
	s.basis, s.thema = basis, thema
	return s, nil
}

// adresseTeilen trennt https://host/pfad/thema in Basis (https://host/pfad)
// und Thema. Die JSON-Schnittstelle von ntfy erwartet das Thema im Body, so
// bleiben auch Umlaute in Titeln heil.
func adresseTeilen(adresse string) (basis, thema string, err error) {
	u, err := url.Parse(adresse)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", "", fmt.Errorf("notify.ntfy_url muss die Form https://server/thema haben")
	}
	if u.User != nil {
		return "", "", fmt.Errorf("notify.ntfy_url darf keine Zugangsdaten enthalten (dafür gibt es NTFY_TOKEN)")
	}
	pfad := strings.Trim(u.Path, "/")
	i := strings.LastIndex(pfad, "/")
	thema = pfad[i+1:]
	vorn := ""
	if i >= 0 {
		vorn = "/" + pfad[:i]
	}
	if !themaMuster.MatchString(thema) {
		return "", "", fmt.Errorf("notify.ntfy_url braucht am Ende ein Thema (https://server/thema)")
	}
	return u.Scheme + "://" + u.Host + vorn, thema, nil
}

// Aktiv sagt, ob Nachrichten verschickt werden.
func (s *Service) Aktiv() bool { return s != nil && s.basis != "" }

// Host ist der Server ohne Thema und ohne Token — fürs Anzeigen.
func (s *Service) Host() string {
	if !s.Aktiv() {
		return ""
	}
	u, err := url.Parse(s.basis)
	if err != nil {
		return ""
	}
	return u.Host
}

// Notify stellt die Nachricht in den Puffer. Es wartet nie.
func (s *Service) Notify(m Message) {
	if !s.Aktiv() {
		return
	}
	select {
	case s.queue <- m:
	default:
		log.Warn().Msg("Benachrichtigung verworfen: Puffer voll")
	}
}

// Start verschickt, was im Puffer liegt, bis ctx endet.
func (s *Service) Start(ctx context.Context) {
	if !s.Aktiv() {
		return
	}
	log.Info().Str("host", s.Host()).Msg("Benachrichtigungen über ntfy sind eingeschaltet")
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-s.queue:
			sendCtx, cancel := context.WithTimeout(ctx, zeitlimit)
			if err := s.Senden(sendCtx, m); err != nil {
				log.Warn().Err(err).Msg("Benachrichtigung nicht zugestellt")
			}
			cancel()
		}
	}
}

type ntfyBody struct {
	Topic    string   `json:"topic"`
	Title    string   `json:"title,omitempty"`
	Message  string   `json:"message"`
	Tags     []string `json:"tags,omitempty"`
	Priority int      `json:"priority,omitempty"`
}

// Senden schickt eine Nachricht sofort und meldet, ob es geklappt hat.
func (s *Service) Senden(ctx context.Context, m Message) error {
	if !s.Aktiv() {
		return errNichtEingerichtet
	}
	body, err := json.Marshal(ntfyBody{Topic: s.thema, Title: m.Title, Message: m.Body, Tags: m.Tags, Priority: m.Priority})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.basis, bytes.NewReader(body))
	if err != nil {
		return errNichtErreichbar
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("ntfy not reachable")
		return errNichtErreichbar
	}
	defer resp.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyRead))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Debug().Int("status", resp.StatusCode).Str("body", string(detail)).Msg("ntfy rejected the message")
		return errAbgelehnt
	}
	return nil
}

// ---------------------------------------------------------------- Handler

// Status zeigt in der Verwaltung, ob die Benachrichtigung an ist — und wohin
// sie geht, ohne Thema und ohne Token.
func (s *Service) Status(w http.ResponseWriter, r *http.Request) {
	auth.WriteJSON(w, map[string]any{"enabled": s.Aktiv(), "host": s.Host()})
}

// Test schickt eine Probenachricht.
func (s *Service) Test(w http.ResponseWriter, r *http.Request) {
	if !s.Aktiv() {
		auth.HTTPError(w, http.StatusConflict, errNichtEingerichtet.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), zeitlimit)
	defer cancel()
	err := s.Senden(ctx, Message{
		Title: "Familien-Dashboard",
		Body:  "Probenachricht — so melden sich Aufgaben, Wünsche und Einlösungen.",
		Tags:  []string{"bell"},
	})
	if err != nil {
		auth.HTTPError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Name liest den Namen einer Person für den Text einer Nachricht.
func Name(db *sql.DB, userID int) string {
	var name string
	if err := db.QueryRow(`SELECT name FROM users WHERE id = ?`, userID).Scan(&name); err != nil {
		return "Jemand"
	}
	return name
}
