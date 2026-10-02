package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"family-dashboard/backend/internal/store"
)

// Standardpersonen nach der Migration: Papa=1 und Mama=2 (Eltern), Kind=3,
// alle mit der PIN 1234.
func elternTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	return NewService(st.DB(), st, "test-secret-test-secret-test-secret", time.Hour, false)
}

func amWandgeraet(id, pin string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/points", nil)
	if id != "" {
		r.Header.Set(ElternIDHeader, id)
	}
	if pin != "" {
		r.Header.Set(ElternPINHeader, pin)
	}
	return AlsPerson(r, 0, RoleDevice, true)
}

// durch ruft die Middleware auf und meldet, als wer der Handler dahinter die
// Anfrage gesehen hat.
func durch(s *Service, r *http.Request) (status int, alsID int, alsGeraet bool) {
	rec := httptest.NewRecorder()
	s.ElternMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alsID, _ = GetUserID(r)
		alsGeraet = IsDevice(r)
		w.WriteHeader(http.StatusTeapot)
	})).ServeHTTP(rec, r)
	return rec.Code, alsID, alsGeraet
}

func TestWandgeraetMitElternPIN(t *testing.T) {
	s := elternTestService(t)
	status, id, geraet := durch(s, amWandgeraet("2", "1234"))
	if status != http.StatusTeapot {
		t.Fatalf("mit richtiger PIN erwartet durchgelassen, Status %d", status)
	}
	if id != 2 || geraet {
		t.Errorf("Handler sollte Mama sehen, nicht das Gerät: id=%d gerät=%v", id, geraet)
	}
}

func TestWandgeraetOhneOderMitFalscherPIN(t *testing.T) {
	s := elternTestService(t)
	for name, r := range map[string]*http.Request{
		"ohne PIN":    amWandgeraet("", ""),
		"falsche PIN": amWandgeraet("1", "9999"),
		"Kinder-PIN":  amWandgeraet("3", "1234"),
		"Unbekannt":   amWandgeraet("99", "1234"),
	} {
		if status, _, _ := durch(s, r); status != http.StatusForbidden {
			t.Errorf("%s: erwartet 403, war %d", name, status)
		}
	}
}

// Fünf Fehlversuche sperren — wie bei der Anmeldung. Sonst liesse sich die
// PIN am Tablet im Flur in Ruhe durchprobieren.
func TestWandgeraetSperreNachFehlversuchen(t *testing.T) {
	s := elternTestService(t)
	for i := 0; i < 5; i++ {
		durch(s, amWandgeraet("1", "0000"))
	}
	if status, _, _ := durch(s, amWandgeraet("1", "1234")); status != http.StatusTooManyRequests {
		t.Errorf("nach fünf Fehlversuchen erwartet 429, war %d", status)
	}
}

func TestElternUndKinderOhneGeraet(t *testing.T) {
	s := elternTestService(t)
	eltern := AlsPerson(httptest.NewRequest(http.MethodPost, "/", nil), 1, "admin", false)
	if status, id, _ := durch(s, eltern); status != http.StatusTeapot || id != 1 {
		t.Errorf("angemeldetes Elternteil sollte durchkommen: %d, id %d", status, id)
	}
	kind := AlsPerson(httptest.NewRequest(http.MethodPost, "/", nil), 3, "member", false)
	if status, _, _ := durch(s, kind); status != http.StatusForbidden {
		t.Errorf("angemeldetes Kind erwartet 403, war %d", status)
	}
}
