package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAdresseTeilen(t *testing.T) {
	gut := map[string][2]string{
		"https://ntfy.example.com/familie":      {"https://ntfy.example.com", "familie"},
		"https://ntfy.example.com/familie/":     {"https://ntfy.example.com", "familie"},
		"http://192.168.1.5:8081/dashboard":     {"http://192.168.1.5:8081", "dashboard"},
		"https://example.com/ntfy/familie_2024": {"https://example.com/ntfy", "familie_2024"},
	}
	for in, want := range gut {
		basis, thema, err := adresseTeilen(in)
		if err != nil || basis != want[0] || thema != want[1] {
			t.Errorf("%s: erwartet %v, war %s %s (%v)", in, want, basis, thema, err)
		}
	}
	for _, in := range []string{
		"ftp://example.com/familie", "https://example.com", "https://example.com/", "example.com/familie",
		"https://user:pw@example.com/familie", "https://example.com/fami lie", "https:///familie",
	} {
		if _, _, err := adresseTeilen(in); err == nil {
			t.Errorf("%q sollte abgelehnt werden", in)
		}
	}
}

func TestAusIstStandard(t *testing.T) {
	s, err := Neu("", "geheim")
	if err != nil {
		t.Fatal(err)
	}
	if s.Aktiv() || s.Host() != "" {
		t.Error("ohne Adresse muss alles aus sein")
	}
	s.Notify(Message{Body: "x"}) // darf weder blockieren noch abstürzen
	if len(s.queue) != 0 {
		t.Error("ausgeschaltet darf nichts im Puffer landen")
	}
	if err := s.Senden(context.Background(), Message{}); err == nil {
		t.Error("Senden ohne Einrichtung sollte scheitern")
	}
	var n Notifier = Aus{}
	n.Notify(Message{})
}

// Die Nachricht kommt als JSON an, mit Umlauten, Thema und Token.
func TestSendenSchicktJSON(t *testing.T) {
	var mu sync.Mutex
	var got ntfyBody
	var auth, ctype, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		auth, ctype, path = r.Header.Get("Authorization"), r.Header.Get("Content-Type"), r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, err := Neu(srv.URL+"/familie", "tk_abc")
	if err != nil {
		t.Fatal(err)
	}
	err = s.Senden(context.Background(), Message{Title: "Wünsche für Käthe", Body: "Zimmer aufräumen ✅", Tags: []string{"gift"}, Priority: 4})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if path != "/" || ctype != "application/json" || auth != "Bearer tk_abc" {
		t.Errorf("Kopf falsch: path=%q type=%q auth=%q", path, ctype, auth)
	}
	if got.Topic != "familie" || got.Title != "Wünsche für Käthe" || got.Message != "Zimmer aufräumen ✅" ||
		got.Priority != 4 || len(got.Tags) != 1 {
		t.Errorf("Inhalt falsch: %+v", got)
	}
}

func TestSendenOhneTokenSchicktKeinenKopf(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
	}))
	defer srv.Close()
	s, _ := Neu(srv.URL+"/familie", "")
	if err := s.Senden(context.Background(), Message{Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		t.Errorf("ohne Token kein Authorization-Kopf, war %q", auth)
	}
}

func TestFehlerbehandlung(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	s, _ := Neu(srv.URL+"/familie", "")
	if err := s.Senden(context.Background(), Message{Body: "x"}); err != errAbgelehnt {
		t.Errorf("403 sollte abgelehnt heißen, war %v", err)
	}
	srv.Close()
	if err := s.Senden(context.Background(), Message{Body: "x"}); err != errNichtErreichbar {
		t.Errorf("geschlossener Server sollte nicht erreichbar heißen, war %v", err)
	}
}

// Ein hängender Server darf den Aufrufer nie aufhalten, und ein voller
// Puffer verwirft, statt zu warten.
func TestNotifyBlockiertNie(t *testing.T) {
	frei := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-frei }))
	defer srv.Close()
	defer close(frei)

	s, _ := Neu(srv.URL+"/familie", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)

	fertig := make(chan struct{})
	go func() {
		for i := 0; i < puffer*3; i++ {
			s.Notify(Message{Body: "x"})
		}
		close(fertig)
	}()
	select {
	case <-fertig:
	case <-time.After(2 * time.Second):
		t.Fatal("Notify hat gewartet")
	}
}

func TestStartVerschicktDenPuffer(t *testing.T) {
	empfangen := make(chan ntfyBody, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b ntfyBody
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &b)
		empfangen <- b
	}))
	defer srv.Close()
	s, _ := Neu(srv.URL+"/familie", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)

	s.Notify(Message{Title: "A", Body: "eins"})
	select {
	case b := <-empfangen:
		if b.Message != "eins" || b.Title != "A" {
			t.Errorf("falsche Nachricht: %+v", b)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Nachricht kam nicht an")
	}
}

func TestStatusUndTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	// Aus: Status sagt es, Test weist ab.
	aus, _ := Neu("", "")
	rec := httptest.NewRecorder()
	aus.Status(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Errorf("Status aus: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	aus.Test(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("Test ohne Einrichtung: erwartet 409, war %d", rec.Code)
	}

	// An: Status nennt den Server, aber weder Thema noch Token.
	an, _ := Neu(srv.URL+"/geheimes-thema", "tk_geheim")
	rec = httptest.NewRecorder()
	an.Status(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `"enabled":true`) || !strings.Contains(body, "127.0.0.1") ||
		strings.Contains(body, "geheim") {
		t.Errorf("Status an verrät zu viel oder zu wenig: %s", body)
	}
	rec = httptest.NewRecorder()
	an.Test(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("Test: erwartet 204, war %d", rec.Code)
	}
}
