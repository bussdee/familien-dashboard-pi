package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// anmelden schickt eine Anmeldung von einem bestimmten Gerät.
func anmelden(s *Service, geraet string, userID int, pin string) int {
	body := `{"user_id":` + itoa(userID) + `,"pin":"` + pin + `"}`
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", stringsReader(body))
	r.RemoteAddr = geraet + ":4000"
	w := httptest.NewRecorder()
	s.Login(w, r)
	return w.Code
}

// Ein Kind, das Papas PIN rät, sperrt nur sein eigenes Gerät.
func TestSperreGiltNurFuerDasGeraet(t *testing.T) {
	s := elternTestService(t)
	for i := 0; i < 5; i++ {
		if code := anmelden(s, "192.168.1.50", 1, "0000"); code != http.StatusUnauthorized {
			t.Fatalf("Versuch %d: erwartet 401, war %d", i+1, code)
		}
	}
	if code := anmelden(s, "192.168.1.50", 1, "1234"); code != http.StatusTooManyRequests {
		t.Errorf("das Gerät des Kindes sollte gesperrt sein, war %d", code)
	}
	// Papa am eigenen Handy kommt mit der richtigen PIN herein.
	if code := anmelden(s, "192.168.1.20", 1, "1234"); code != http.StatusOK {
		t.Errorf("anderes Gerät sollte durchkommen, war %d", code)
	}
	// Und das Kind kann sich selbst weiter anmelden: gesperrt ist Papa auf
	// diesem Gerät, nicht das Gerät für alle.
	if code := anmelden(s, "192.168.1.50", 3, "1234"); code != http.StatusOK {
		t.Errorf("das Kind sollte sich selbst anmelden können, war %d", code)
	}
}

// Nach Ablauf der Sperre zählt es von vorn: ein einzelner Fehlversuch sperrt
// nicht sofort wieder (früher blieb der Zähler bei 5 stehen).
func TestSperreLaeuftAbUndZaehltNeu(t *testing.T) {
	s := elternTestService(t)
	for i := 0; i < 5; i++ {
		anmelden(s, "192.168.1.50", 1, "0000")
	}
	// Die Sperre in die Vergangenheit verlegen, als wären 5 Minuten um.
	if _, err := s.db.Exec(`UPDATE login_attempts_source SET locked_until = ?`, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if code := anmelden(s, "192.168.1.50", 1, "0000"); code != http.StatusUnauthorized {
		t.Fatalf("erster Fehlversuch nach der Sperre: erwartet 401, war %d", code)
	}
	if code := anmelden(s, "192.168.1.50", 1, "1234"); code != http.StatusOK {
		t.Errorf("ein einzelner Fehlversuch darf nicht sofort wieder sperren, war %d", code)
	}
}

// Fehlversuche verfallen: vier heute und einer nächste Woche sind keine fünf.
func TestFehlversucheVerfallenNachDemFenster(t *testing.T) {
	s := elternTestService(t)
	for i := 0; i < 4; i++ {
		anmelden(s, "192.168.1.50", 1, "0000")
	}
	if _, err := s.db.Exec(`UPDATE login_attempts_source SET first_at = ?`, time.Now().Add(-failureWindow-time.Minute)); err != nil {
		t.Fatal(err)
	}
	anmelden(s, "192.168.1.50", 1, "0000") // der fünfte, aber im neuen Fenster der erste
	if code := anmelden(s, "192.168.1.50", 1, "1234"); code != http.StatusOK {
		t.Errorf("alte Fehlversuche sollten nicht mitzählen, war %d", code)
	}
}

// Wer von vielen Geräten aus probiert, stößt an die Obergrenze der Person.
func TestObergrenzeJePerson(t *testing.T) {
	s := elternTestService(t)
	for i := 0; i < maxPersonFailures; i++ {
		geraet := "10.1.0." + itoa(i+1) // jedes Mal ein anderes Gerät
		if code := anmelden(s, geraet, 1, "0000"); code != http.StatusUnauthorized {
			t.Fatalf("Versuch %d: erwartet 401, war %d", i+1, code)
		}
	}
	// Ein ganz neues Gerät mit der richtigen PIN wird trotzdem gesperrt.
	if code := anmelden(s, "10.9.9.9", 1, "1234"); code != http.StatusTooManyRequests {
		t.Errorf("Obergrenze je Person sollte greifen, war %d", code)
	}
	// Andere Personen sind davon nicht betroffen.
	if code := anmelden(s, "10.9.9.9", 2, "1234"); code != http.StatusOK {
		t.Errorf("andere Person sollte durchkommen, war %d", code)
	}
}

// Eine richtige PIN hebt die Sperre dieses Geräts nicht auf, aber zurücksetzen
// durch einen Administrator räumt alles ab.
func TestSperreAufheben(t *testing.T) {
	s := elternTestService(t)
	for i := 0; i < 5; i++ {
		anmelden(s, "192.168.1.50", 1, "0000")
	}
	s.clearAllFailures(1)
	if code := anmelden(s, "192.168.1.50", 1, "1234"); code != http.StatusOK {
		t.Errorf("nach dem Zurücksetzen erwartet 200, war %d", code)
	}
}

// Das Wandgerät mit Eltern-PIN sperrt ebenfalls je Gerät.
func TestWandgeraetSperreJeGeraet(t *testing.T) {
	s := elternTestService(t)
	flur := func() *http.Request {
		r := amWandgeraet("1", "0000")
		r.RemoteAddr = "192.168.1.60:1"
		return r
	}
	for i := 0; i < 5; i++ {
		durch(s, flur())
	}
	if status, _, _ := durch(s, flur()); status != http.StatusTooManyRequests {
		t.Fatalf("Wandgerät sollte gesperrt sein, war %d", status)
	}
	// Papa am Handy meldet sich weiter an.
	if code := anmelden(s, "192.168.1.20", 1, "1234"); code != http.StatusOK {
		t.Errorf("Anmeldung vom Handy erwartet 200, war %d", code)
	}
}

func TestClientAdresse(t *testing.T) {
	faelle := []struct {
		name   string
		remote string
		xff    []string
		stufen int
		want   string
	}{
		{"direkt", "192.168.1.9:5555", nil, 2, "192.168.1.9"},
		// Traefik hängt das Gerät an, nginx hängt Traefik an, RemoteAddr ist nginx.
		{"Traefik und nginx", "172.18.0.4:4000", []string{"192.168.1.9, 172.18.0.2"}, 2, "192.168.1.9"},
		// Das Gerät behauptet etwas: steht links und wird nicht gelesen.
		{"erfundener Eintrag", "172.18.0.4:4000", []string{"6.6.6.6, 192.168.1.9, 172.18.0.2"}, 2, "192.168.1.9"},
		{"mehrere Köpfe", "172.18.0.4:4000", []string{"6.6.6.6", "192.168.1.9, 172.18.0.2"}, 2, "192.168.1.9"},
		// Weniger Stationen als erwartet: die direkte Verbindung zählt.
		{"zu kurze Kette", "172.18.0.4:4000", []string{"192.168.1.9"}, 2, "172.18.0.4"},
		{"eine Station", "172.18.0.2:4000", []string{"192.168.1.9"}, 1, "192.168.1.9"},
		{"keine Station", "192.168.1.9:1", []string{"6.6.6.6"}, 0, "192.168.1.9"},
		{"Müll im Kopf", "172.18.0.4:4000", []string{"hallo, welt, nein"}, 2, "172.18.0.4"},
		{"IPv4 in IPv6", "172.18.0.4:4000", []string{"::ffff:192.168.1.9, 172.18.0.2"}, 2, "192.168.1.9"},
		{"IPv6 im /64", "172.18.0.4:4000", []string{"2001:db8:1:2:aaaa:bbbb:cccc:dddd, 172.18.0.2"}, 2, "2001:db8:1:2::/64"},
	}
	for _, f := range faelle {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = f.remote
		for _, x := range f.xff {
			r.Header.Add("X-Forwarded-For", x)
		}
		r.Header.Set("X-Real-IP", "7.7.7.7")
		r.Header.Set("True-Client-IP", "8.8.8.8")
		if got := ClientAdresse(r, f.stufen); got != f.want {
			t.Errorf("%s: erwartet %s, war %s", f.name, f.want, got)
		}
	}
}

func TestRealClientIPSetztRemoteAddr(t *testing.T) {
	var gesehen string
	h := RealClientIP(2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gesehen = quelle(r)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "172.18.0.4:4000"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 192.168.1.9, 172.18.0.2")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if gesehen != "192.168.1.9" {
		t.Errorf("erwartet 192.168.1.9, war %s", gesehen)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
