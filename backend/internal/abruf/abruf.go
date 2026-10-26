// Package abruf ist der einzige Weg, auf dem der Server auf Wunsch einer
// Person eine fremde Adresse aufruft: Kalender-Abos, Rezepte, Neuigkeiten.
//
// Ohne Vorkehrungen wäre er damit ein Werkzeug, das das Heimnetz abklopft
// (Router, Plex, der Pi selbst). Deshalb gilt für jeden Abruf:
//
//   - nur http und https, keine Zugangsdaten in der Adresse,
//   - gesperrt wird die IP, die tatsächlich angewählt wird — nicht der Name,
//     den jemand so wählen kann, dass er erst beim Abruf ins Heimnetz zeigt,
//   - Weiterleitungen, Zeit und Größe sind begrenzt,
//   - eine Umgebungsvariable wie HTTP_PROXY wird nicht beachtet.
//
// Was mit der Antwort geschieht (als Kalender lesen, als Rezept, als Feed),
// entscheidet der Aufrufer.
package abruf

import (
	"context"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
)

// Meldungen, die bei der Person ankommen. Bewusst feste Texte: Sie lassen sich
// übersetzen, und die Einzelheiten stehen im Log, nicht in der Oberfläche.
var (
	ErrFehlt           = errors.New("Adresse fehlt")
	ErrUngueltig       = errors.New("Die Adresse ist ungültig")
	ErrZuLang          = errors.New("Die Adresse ist zu lang")
	ErrSchema          = errors.New("Die Adresse muss mit http:// oder https:// beginnen")
	ErrZugang          = errors.New("Zugangsdaten gehören nicht in die Adresse")
	ErrGesperrt        = errors.New("Die Adresse zeigt ins Heimnetz oder ist nicht öffentlich")
	ErrWeiterleitung   = errors.New("Die Adresse leitet zu oft weiter")
	ErrZuGross         = errors.New("Die Antwort ist zu groß")
	ErrServerFehler    = errors.New("Die Seite antwortet mit einem Fehler")
	ErrNichtErreichbar = errors.New("Die Seite ist nicht erreichbar")
)

// MaxWeiterleitungen ist die Zahl der Weiterleitungen, denen gefolgt wird.
const MaxWeiterleitungen = 3

// Bereinigen macht aus der Eingabe eine Adresse, die der Server aufrufen darf.
func Bereinigen(roh string) (string, error) {
	roh = strings.TrimSpace(roh)
	if roh == "" {
		return "", ErrFehlt
	}
	if len(roh) > 2000 {
		return "", ErrZuLang
	}
	u, err := url.Parse(roh)
	if err != nil {
		return "", ErrUngueltig
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrSchema
	}
	if u.User != nil {
		return "", ErrZugang
	}
	if u.Hostname() == "" {
		return "", ErrUngueltig
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

// ErlaubteAdresse sagt, ob der Server diese IP anwählen darf: nur öffentliche
// Adressen. Eine als IPv6 verpackte IPv4-Adresse (::ffff:127.0.0.1) wird als
// das behandelt, was sie ist.
func ErlaubteAdresse(ip netip.Addr) bool {
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

// NeuerClient liefert einen HTTP-Client, der nur öffentliche Adressen
// anwählt. Die Prüfung sitzt im Dialer, nach der Namensauflösung und vor dem
// Verbindungsaufbau: Sie sieht die IP, die wirklich benutzt wird, und gilt
// damit auch für jede Weiterleitung.
func NeuerClient(zeitlimit time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || !ErlaubteAdresse(ap.Addr()) {
				return ErrGesperrt
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: zeitlimit,
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            dialer.DialContext,
			TLSHandshakeTimeout:    10 * time.Second,
			ResponseHeaderTimeout:  zeitlimit,
			DisableKeepAlives:      true,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxWeiterleitungen {
				return ErrWeiterleitung
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrSchema
			}
			if req.URL.User != nil {
				return ErrZugang
			}
			return nil
		},
	}
}

// Antwort ist, was ein Abruf geliefert hat.
type Antwort struct {
	Body []byte
	// Typ ist der Content-Type ohne Zusätze („text/html").
	Typ string
}

// Holen ruft die Adresse ab und liest höchstens maxBytes. Fehler sind die
// Err…-Werte oben; was schiefging, steht im Log.
func Holen(ctx context.Context, client *http.Client, adresse, accept, agent string, maxBytes int64, zeitlimit time.Duration) (Antwort, error) {
	ctx, cancel := context.WithTimeout(ctx, zeitlimit)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adresse, nil)
	if err != nil {
		return Antwort{}, ErrUngueltig
	}
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Accept", accept)

	resp, err := client.Do(req)
	if err != nil {
		return Antwort{}, Uebersetzen(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("Abruf: unerwarteter Status")
		return Antwort{}, ErrServerFehler
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		log.Debug().Err(err).Msg("Abruf: Lesen fehlgeschlagen")
		return Antwort{}, ErrNichtErreichbar
	}
	if int64(len(data)) > maxBytes {
		return Antwort{}, ErrZuGross
	}
	typ := resp.Header.Get("Content-Type")
	if i := strings.IndexByte(typ, ';'); i >= 0 {
		typ = typ[:i]
	}
	return Antwort{Body: data, Typ: strings.ToLower(strings.TrimSpace(typ))}, nil
}

// Uebersetzen macht aus einem Fehler des HTTP-Clients eine der festen
// Meldungen.
func Uebersetzen(err error) error {
	switch {
	case errors.Is(err, ErrGesperrt):
		return ErrGesperrt
	case errors.Is(err, ErrWeiterleitung):
		return ErrWeiterleitung
	case errors.Is(err, ErrSchema), errors.Is(err, ErrZugang):
		return ErrUngueltig
	}
	log.Debug().Err(err).Msg("Abruf fehlgeschlagen")
	return ErrNichtErreichbar
}

// ---------------------------------------------------------- Fremder Text

var (
	htmlTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlUmbruch = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/h[1-6])\s*/?>`)
	mehrLeer    = regexp.MustCompile(`[ \t\x{00a0}]+`)
	mehrZeilen  = regexp.MustCompile(`\n{3,}`)
)

// Klartext macht aus fremdem Text (Feed, Webseite) Text, der sich gefahrlos
// anzeigen lässt: Tags fallen weg, Sonderzeichen werden gelesen, Steuerzeichen
// verschwinden, die Länge ist begrenzt. Die Oberfläche zeigt das Ergebnis nie
// als HTML. mehrzeilig behält Absätze, sonst wird eine Zeile daraus.
func Klartext(s string, max int, mehrzeilig bool) string {
	s = htmlUmbruch.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			if mehrzeilig {
				return '\n'
			}
			return ' '
		case r == '\r' || r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		}
		return r
	}, s)
	s = mehrLeer.ReplaceAllString(s, " ")
	if mehrzeilig {
		s = mehrZeilen.ReplaceAllString(s, "\n\n")
	}
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		s = strings.TrimSpace(string([]rune(s)[:max]))
	}
	return s
}
