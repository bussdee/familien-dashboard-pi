package calendar

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestErlaubteAdresse(t *testing.T) {
	gesperrt := []string{
		"127.0.0.1", "127.1.2.3", "10.0.0.5", "172.16.0.1", "172.31.255.1",
		"192.168.1.1", "169.254.169.254", "0.0.0.0", "224.0.0.1", "255.255.255.255",
		"100.64.0.1", "100.100.100.100", "192.0.2.1", "198.51.100.7", "203.0.113.9",
		"198.18.0.1", "240.0.0.1",
		"::1", "::", "fc00::1", "fd12:3456::1", "fe80::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:192.168.0.1",
		"64:ff9b::7f00:1", "2001:db8::1", "2002:7f00:1::1", "2001::1",
	}
	for _, s := range gesperrt {
		if erlaubteAdresse(netip.MustParseAddr(s)) {
			t.Errorf("%s sollte gesperrt sein", s)
		}
	}
	erlaubt := []string{"93.184.216.34", "8.8.8.8", "1.1.1.1", "172.32.0.1", "100.63.255.255", "100.128.0.1",
		"2606:4700:4700::1111", "2a00:1450:4001::200e", "::ffff:8.8.8.8"}
	for _, s := range erlaubt {
		if !erlaubteAdresse(netip.MustParseAddr(s)) {
			t.Errorf("%s sollte erlaubt sein", s)
		}
	}
	if erlaubteAdresse(netip.Addr{}) {
		t.Error("die leere Adresse sollte gesperrt sein")
	}
}

func TestAdresseBereinigen(t *testing.T) {
	gut := map[string]string{
		"https://example.com/k.ics":          "https://example.com/k.ics",
		"  http://example.com/k.ics  ":       "http://example.com/k.ics",
		"webcal://example.com/k.ics":         "https://example.com/k.ics",
		"WEBCAL://example.com/k.ics":         "https://example.com/k.ics",
		"webcals://example.com/k.ics?a=1":    "https://example.com/k.ics?a=1",
		"https://example.com/k.ics#fragment": "https://example.com/k.ics",
	}
	for roh, erwartet := range gut {
		got, err := adresseBereinigen(roh)
		if err != nil || got != erwartet {
			t.Errorf("%q: erwartet %q, war %q (%v)", roh, erwartet, got, err)
		}
	}

	schlecht := map[string]error{
		"":                                  errAdresseFehlt,
		"   ":                               errAdresseFehlt,
		"file:///etc/passwd":                errAdresseSchema,
		"ftp://example.com/k.ics":           errAdresseSchema,
		"gopher://example.com":              errAdresseSchema,
		"example.com/k.ics":                 errAdresseSchema,
		"https://user:pw@example.com/k.ics": errAdresseZugang,
		"https://user@example.com/k.ics":    errAdresseZugang,
		"https:///k.ics":                    errAdresseUngueltig,
		"https://exa mple.com/":             errAdresseUngueltig,
		"https://example.com/" + strings.Repeat("a", 2100): errAdresseZuLang,
	}
	for roh, erwartet := range schlecht {
		if _, err := adresseBereinigen(roh); !errors.Is(err, erwartet) {
			t.Errorf("%.40q: erwartet %v, war %v", roh, erwartet, err)
		}
	}
}

// beispielKalender hat einen Termin in einer Woche, damit er im Zeitfenster
// des Dashboards liegt.
var beispielKalender = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//DE\r\n" +
	"BEGIN:VEVENT\r\nUID:abc@test\r\nDTSTART:" + time.Now().UTC().AddDate(0, 0, 7).Format("20060102") + "T100000Z\r\n" +
	"DTEND:" + time.Now().UTC().AddDate(0, 0, 7).Format("20060102") + "T110000Z\r\nSUMMARY:Elternabend\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

// Der echte Client darf keinen Testserver auf 127.0.0.1 erreichen.
func TestSichererClientSperrtLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(beispielKalender))
	}))
	defer srv.Close()

	_, err := kalenderHolen(context.Background(), sichererClient(), srv.URL)
	if !errors.Is(err, errGesperrt) {
		t.Fatalf("erwartet gesperrt, war %v", err)
	}
}

// Eine Weiterleitung ins Heimnetz scheitert ebenfalls, auch wenn die erste
// Adresse harmlos aussieht. Hier trifft schon die erste Verbindung die Sperre;
// die Weiterleitung selbst prüft CheckRedirect.
func TestSichererClientPruftWeiterleitungen(t *testing.T) {
	c := sichererClient()

	mk := func(u string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, u, nil)
		return r
	}
	via := func(n int) []*http.Request {
		out := make([]*http.Request, n)
		for i := range out {
			out[i] = mk("https://example.com/")
		}
		return out
	}

	if err := c.CheckRedirect(mk("https://example.com/b"), via(3)); err != nil {
		t.Errorf("3 Weiterleitungen sind erlaubt: %v", err)
	}
	if err := c.CheckRedirect(mk("https://example.com/b"), via(4)); !errors.Is(err, errWeiterleitung) {
		t.Errorf("die 4. Weiterleitung sollte scheitern, war %v", err)
	}
	if err := c.CheckRedirect(mk("file:///etc/passwd"), via(1)); !errors.Is(err, errAdresseSchema) {
		t.Errorf("file:// als Ziel sollte scheitern, war %v", err)
	}
	if err := c.CheckRedirect(mk("https://u:p@example.com/"), via(1)); !errors.Is(err, errAdresseZugang) {
		t.Errorf("Zugangsdaten im Ziel sollten scheitern, war %v", err)
	}
}

// Ende zu Ende gegen einen lokalen Testserver; dafür ersetzt der Test den
// Client durch einen ohne Sperre.
func TestKalenderHolenPrueftDenInhalt(t *testing.T) {
	antwort := beispielKalender
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/weiter" {
			http.Redirect(w, r, "/kalender.ics", http.StatusFound)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(antwort))
	}))
	defer srv.Close()
	client := &http.Client{Timeout: 5 * time.Second}

	if data, err := kalenderHolen(context.Background(), client, srv.URL+"/kalender.ics"); err != nil || len(data) == 0 {
		t.Fatalf("gültiger Kalender: %v", err)
	}
	if _, err := kalenderHolen(context.Background(), client, srv.URL+"/weiter"); err != nil {
		t.Errorf("Weiterleitung auf einen Kalender: %v", err)
	}

	antwort = "<html>Anmeldung</html>"
	if _, err := kalenderHolen(context.Background(), client, srv.URL); !errors.Is(err, errKeinKalender) {
		t.Errorf("HTML ist kein Kalender, war %v", err)
	}
	antwort = "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n"
	if _, err := kalenderHolen(context.Background(), client, srv.URL); !errors.Is(err, errKeinKalender) {
		t.Errorf("abgeschnittener Kalender, war %v", err)
	}
	antwort = beispielKalender + strings.Repeat("X-FILLER:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r\n", 150_000)
	if _, err := kalenderHolen(context.Background(), client, srv.URL); !errors.Is(err, errZuGross) {
		t.Errorf("über 5 MB, war %v", err)
	}
	status = http.StatusInternalServerError
	antwort = beispielKalender
	if _, err := kalenderHolen(context.Background(), client, srv.URL); !errors.Is(err, errServerFehler) {
		t.Errorf("Status 500, war %v", err)
	}
}

// Anlegen, Termine erscheinen, Aktualisierung bei Fehler behält die letzte gute
// Fassung, Löschen räumt auf.
func TestAboLebenslauf(t *testing.T) {
	s := testDienst(t)
	s.clientFabrik = func() *http.Client { return &http.Client{Timeout: 5 * time.Second} }

	antwort := beispielKalender
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(antwort))
	}))
	defer srv.Close()

	// Ein kaputter Kalender kommt gar nicht erst in die Liste.
	antwort = "kein Kalender"
	w := anfrage(s.CreateSubscription, http.MethodPost, "0", "", map[string]string{"name": "Schule", "url": srv.URL})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("erwartet 400, war %d", w.Code)
	}
	if liste, _ := s.abos(); len(liste) != 0 {
		t.Fatalf("kein Abo erwartet, waren %d", len(liste))
	}

	antwort = beispielKalender
	w = anfrage(s.CreateSubscription, http.MethodPost, "0", "", map[string]string{"name": "Schule", "url": srv.URL, "color": "#112233"})
	if w.Code != http.StatusCreated {
		t.Fatalf("Anlegen: Status %d, %s", w.Code, w.Body.String())
	}
	liste, _ := s.abos()
	if len(liste) != 1 || liste[0].LastOK == nil || liste[0].LastError != "" {
		t.Fatalf("Abo falsch gespeichert: %+v", liste)
	}
	id := liste[0].ID
	if _, err := os.Stat(s.aboDatei(id)); err != nil {
		t.Fatalf("Datei fehlt: %v", err)
	}
	// Die Datei liegt NICHT im ics-Ordner (sonst gäbe es die Termine doppelt).
	if filepath.Dir(s.aboDatei(id)) == s.icsDir {
		t.Error("Abo-Datei darf nicht im ics-Ordner liegen")
	}

	events := s.aboEvents()
	if len(events) != 1 || events[0].Title != "Elternabend" || events[0].Calendar != "Schule" ||
		events[0].Color != "#112233" || !strings.HasPrefix(events[0].ID, "abo") || events[0].Editable {
		t.Fatalf("Termine falsch: %+v", events)
	}

	// Der Server fällt aus: Die letzte gute Fassung bleibt, der Fehler steht da.
	status = http.StatusBadGateway
	w = anfrage(s.RefreshSubscription, http.MethodPost, itoa(id), "", nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("erwartet 502, war %d", w.Code)
	}
	liste, _ = s.abos()
	if liste[0].LastError != errServerFehler.Error() || liste[0].LastOK == nil {
		t.Errorf("Fehlerstand falsch: %+v", liste[0])
	}
	if len(s.aboEvents()) != 1 {
		t.Error("die letzte gute Fassung sollte bleiben")
	}

	// Wieder da: der Fehler verschwindet.
	status = http.StatusOK
	if w = anfrage(s.RefreshSubscription, http.MethodPost, itoa(id), "", nil); w.Code != http.StatusOK {
		t.Fatalf("Aktualisieren: Status %d", w.Code)
	}
	liste, _ = s.abos()
	if liste[0].LastError != "" {
		t.Errorf("Fehler sollte gelöscht sein: %q", liste[0].LastError)
	}

	// Und er erscheint im Kalender, den die Oberfläche abruft.
	s.parseAll()
	rec := httptest.NewRecorder()
	s.GetEvents(rec, httptest.NewRequest(http.MethodGet, "/calendar?days=30", nil))
	if !strings.Contains(rec.Body.String(), "Elternabend") {
		t.Errorf("Abo-Termin fehlt im Kalender: %s", rec.Body.String())
	}

	if w = anfrage(s.DeleteSubscription, http.MethodDelete, itoa(id), "", nil); w.Code != http.StatusNoContent {
		t.Fatalf("Löschen: Status %d", w.Code)
	}
	if _, err := os.Stat(s.aboDatei(id)); !os.IsNotExist(err) {
		t.Error("Datei sollte weg sein")
	}
	if liste, _ := s.abos(); len(liste) != 0 {
		t.Error("Abo sollte weg sein")
	}
}

func TestAboAnlegenPrueftEingaben(t *testing.T) {
	s := testDienst(t)
	s.clientFabrik = func() *http.Client { return &http.Client{Timeout: time.Second} }
	faelle := map[string]map[string]string{
		"ohne Namen":    {"name": " ", "url": "https://example.com/k.ics"},
		"ohne Adresse":  {"name": "x", "url": ""},
		"file-Adresse":  {"name": "x", "url": "file:///etc/passwd"},
		"mit Zugang":    {"name": "x", "url": "https://a:b@example.com/k.ics"},
		"falsche Farbe": {"name": "x", "url": "https://example.com/k.ics", "color": "rot"},
	}
	for name, body := range faelle {
		if w := anfrage(s.CreateSubscription, http.MethodPost, "0", "", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: erwartet 400, war %d", name, w.Code)
		}
	}
}

// Der echte Weg: Ein Abo auf eine Heimnetz-Adresse wird abgelehnt.
func TestAboAufHeimnetzAbgelehnt(t *testing.T) {
	s := testDienst(t) // echter Client
	for _, u := range []string{"http://127.0.0.1:1/k.ics", "http://192.168.1.1/k.ics", "http://169.254.169.254/latest/meta-data/"} {
		w := anfrage(s.CreateSubscription, http.MethodPost, "0", "", map[string]string{"name": "x", "url": u})
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), errGesperrt.Error()) {
			t.Errorf("%s: erwartet gesperrt, war %d %s", u, w.Code, w.Body.String())
		}
	}
}
