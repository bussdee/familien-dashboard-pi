package familienpost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

const (
	papa = 1
	mama = 2
	kind = 3
)

func testService(t *testing.T) *Service {
	t.Helper()
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	return NewService(st.DB())
}

func aufruf(h http.HandlerFunc, wer int, rolle, body, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r = auth.AlsPerson(r, wer, rolle, false)
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func punkte(t *testing.T, s *Service, wer int) int {
	t.Helper()
	var n int
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(points), 0) FROM point_events WHERE user_id = ? AND source = ?`, wer, Quelle).Scan(&n)
	return n
}

type liste struct {
	Inbox []Brief `json:"inbox"`
	Sent  []Brief `json:"sent"`
}

func lesenAls(t *testing.T, s *Service, wer int, rolle string) liste {
	t.Helper()
	w := aufruf(s.List, wer, rolle, "", "")
	var l liste
	_ = json.Unmarshal(w.Body.Bytes(), &l)
	return l
}

func TestPostLebenslauf(t *testing.T) {
	s := testService(t)
	w := aufruf(s.Create, papa, "admin", `{"to_user":3,"text":"Super gemacht mit dem Zimmer!","bonus":5}`, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("schreiben: %d %s", w.Code, w.Body)
	}

	// Ungeöffnet: Das Kind sieht, dass Post da ist, aber nicht, was drinsteht.
	l := lesenAls(t, s, kind, "member")
	if len(l.Inbox) != 1 || l.Inbox[0].Text != "" || l.Inbox[0].VonName != "Papa" || l.Inbox[0].Geoeffnet != nil {
		t.Fatalf("Eingang vor dem Öffnen: %+v", l.Inbox)
	}
	if l.Sent != nil {
		t.Error("ein Kind hat keinen Postausgang")
	}

	// Nur das Kind öffnet.
	if w := aufruf(s.Open, mama, "admin", "", "1"); w.Code != http.StatusNotFound {
		t.Errorf("fremde Post öffnen: %d", w.Code)
	}
	w = aufruf(s.Open, kind, "member", "", "1")
	var auf struct {
		Mail      Brief `json:"mail"`
		FirstOpen bool  `json:"first_open"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &auf)
	if w.Code != http.StatusOK || !auf.FirstOpen || auf.Mail.Text != "Super gemacht mit dem Zimmer!" {
		t.Fatalf("öffnen: %d %+v", w.Code, auf)
	}
	if got := punkte(t, s, kind); got != 5 {
		t.Errorf("Bonus: %d", got)
	}
	// Zweites Öffnen: kein zweiter Bonus.
	w = aufruf(s.Open, kind, "member", "", "1")
	_ = json.Unmarshal(w.Body.Bytes(), &auf)
	if auf.FirstOpen || punkte(t, s, kind) != 5 {
		t.Errorf("zweites Öffnen: %+v, Punkte %d", auf, punkte(t, s, kind))
	}

	// Papa sieht im Ausgang, dass sie geöffnet ist; zurücknehmen geht nicht mehr.
	l = lesenAls(t, s, papa, "admin")
	if len(l.Sent) != 1 || l.Sent[0].Geoeffnet == nil {
		t.Fatalf("Ausgang: %+v", l.Sent)
	}
	if w := aufruf(s.Delete, papa, "admin", "", "1"); w.Code != http.StatusConflict {
		t.Errorf("geöffnete zurücknehmen: %d", w.Code)
	}
}

func TestPostRegeln(t *testing.T) {
	s := testService(t)
	for name, c := range map[string]struct {
		wer   int
		rolle string
		body  string
		code  int
	}{
		"Kind schreibt":  {kind, "member", `{"to_user":1,"text":"Hallo"}`, http.StatusForbidden},
		"leer":           {papa, "admin", `{"to_user":3,"text":"  "}`, http.StatusBadRequest},
		"zu lang":        {papa, "admin", `{"to_user":3,"text":"` + strings.Repeat("a", 300) + `"}`, http.StatusBadRequest},
		"Bonus zu hoch":  {papa, "admin", `{"to_user":3,"text":"Hi","bonus":500}`, http.StatusBadRequest},
		"negativ":        {papa, "admin", `{"to_user":3,"text":"Hi","bonus":-1}`, http.StatusBadRequest},
		"an sich selbst": {papa, "admin", `{"to_user":1,"text":"Hi"}`, http.StatusBadRequest},
		"unbekannt":      {papa, "admin", `{"to_user":99,"text":"Hi"}`, http.StatusBadRequest},
		"kein JSON":      {papa, "admin", `nein`, http.StatusBadRequest},
	} {
		if w := aufruf(s.Create, c.wer, c.rolle, c.body, ""); w.Code != c.code {
			t.Errorf("%s: erwartet %d, war %d (%s)", name, c.code, w.Code, w.Body)
		}
	}

	// Zurücknehmen: nur der Absender, nur ungeöffnet.
	aufruf(s.Create, papa, "admin", `{"to_user":3,"text":"Gute Nacht"}`, "")
	if w := aufruf(s.Delete, mama, "admin", "", "1"); w.Code != http.StatusConflict {
		t.Errorf("fremde Post zurücknehmen: %d", w.Code)
	}
	if w := aufruf(s.Delete, papa, "admin", "", "1"); w.Code != http.StatusNoContent {
		t.Errorf("eigene zurücknehmen: %d", w.Code)
	}

	// Höchstens zehn ungeöffnete auf einmal.
	for i := 0; i < maxUngelesen; i++ {
		aufruf(s.Create, papa, "admin", `{"to_user":3,"text":"Nr"}`, "")
	}
	if w := aufruf(s.Create, papa, "admin", `{"to_user":3,"text":"zu viel"}`, ""); w.Code != http.StatusConflict {
		t.Errorf("elfte ungeöffnete: %d", w.Code)
	}
}
