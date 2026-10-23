package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

// testDienst baut einen Kalender mit echter Datenbank im Temp-Ordner.
func testDienst(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewStore(filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	s := NewService(filepath.Join(dir, "ics"), 400, false, "Europe/Vienna")
	s.SetDB(st.DB())
	return s
}

// anfrage ruft einen Handler mit {id} und optional {date} in der Route auf.
func anfrage(h http.HandlerFunc, methode, id, datum string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(methode, "/x", &buf)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", id)
	if datum != "" {
		rc.URLParams.Add("date", datum)
	}
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

// woechentlich legt einen Termin an, der jeden Tag um 18 Uhr (Wiener Zeit)
// stattfindet, und gibt seine ID zurück.
func legeAn(t *testing.T, s *Service, wiederholung Repeat, start time.Time) int {
	t.Helper()
	res, err := s.db.Exec(`INSERT INTO calendar_events (title, start_at, end_at, repeat) VALUES (?, ?, ?, ?)`,
		"Klavier", start, start.Add(time.Hour), string(wiederholung))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

func tage(events []Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, e.Start.In(time.UTC).Format("2006-01-02"))
	}
	return out
}

func TestAbsageLaesstGenauEinenTagAus(t *testing.T) {
	s := testDienst(t)
	vienna := s.loc
	start := time.Date(2026, time.October, 1, 18, 0, 0, 0, vienna)
	id := legeAn(t, s, RepeatWeekly, start) // donnerstags

	von := time.Date(2026, time.October, 1, 0, 0, 0, 0, vienna)
	bis := von.AddDate(0, 0, 30)

	if got := len(s.storedEvents(von, bis)); got != 5 {
		t.Fatalf("vor der Absage erwartet 5 Donnerstage, waren %d", got)
	}

	w := anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "2026-10-15"})
	if w.Code != http.StatusOK {
		t.Fatalf("Absage: Status %d, %s", w.Code, w.Body.String())
	}

	got := s.storedEvents(von, bis)
	if len(got) != 4 {
		t.Fatalf("nach der Absage erwartet 4 Termine, waren %d: %v", len(got), tage(got))
	}
	for _, e := range got {
		if e.Day == "2026-10-15" {
			t.Errorf("der abgesagte Tag ist noch da: %v", tage(got))
		}
	}
	// Die Nachbarn bleiben.
	for _, erwartet := range []string{"2026-10-01", "2026-10-08", "2026-10-22", "2026-10-29"} {
		if !strings.Contains(strings.Join(tage(got), ","), erwartet) {
			t.Errorf("%s fehlt: %v", erwartet, tage(got))
		}
	}

	// Zurücknehmen bringt ihn wieder.
	w = anfrage(s.RestoreOccurrence, http.MethodDelete, itoa(id), "2026-10-15", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Zurücknehmen: Status %d, %s", w.Code, w.Body.String())
	}
	if got := len(s.storedEvents(von, bis)); got != 5 {
		t.Errorf("nach dem Zurücknehmen erwartet 5, waren %d", got)
	}
}

// Der Tag zählt in der Ortszeit des Dashboards, nicht in UTC: Ein Termin um
// 00:30 Wiener Zeit liegt in UTC noch am Vortag.
func TestAbsageZaehltInOrtszeit(t *testing.T) {
	s := testDienst(t)
	start := time.Date(2026, time.October, 5, 0, 30, 0, 0, s.loc)
	id := legeAn(t, s, RepeatDaily, start)

	von := time.Date(2026, time.October, 5, 0, 0, 0, 0, s.loc)
	bis := von.AddDate(0, 0, 4)

	// 06.10. 00:30 Wien = 05.10. 22:30 UTC. Abgesagt wird der 6., nicht der 5.
	if w := anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "2026-10-06"}); w.Code != http.StatusOK {
		t.Fatalf("Status %d", w.Code)
	}
	var tageWien []string
	for _, e := range s.storedEvents(von, bis) {
		tageWien = append(tageWien, e.Day)
	}
	if strings.Join(tageWien, ",") != "2026-10-05,2026-10-07,2026-10-08" {
		t.Errorf("falsche Tage: %v", tageWien)
	}
}

func TestEinmaligerTerminLaesstSichNichtAbsagen(t *testing.T) {
	s := testDienst(t)
	start := time.Date(2026, time.October, 1, 18, 0, 0, 0, s.loc)
	id := legeAn(t, s, RepeatNone, start)

	w := anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "2026-10-01"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("erwartet 400, war %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Nur Wiederholungen lassen sich einzeln absagen") {
		t.Errorf("Meldung fehlt: %s", w.Body.String())
	}
}

func TestAbsageUngueltigeEingaben(t *testing.T) {
	s := testDienst(t)
	id := legeAn(t, s, RepeatDaily, time.Date(2026, time.October, 1, 18, 0, 0, 0, s.loc))

	if w := anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "morgen"}); w.Code != http.StatusBadRequest {
		t.Errorf("kaputtes Datum: erwartet 400, war %d", w.Code)
	}
	if w := anfrage(s.SkipOccurrence, http.MethodPost, "999", "", map[string]string{"date": "2026-10-02"}); w.Code != http.StatusNotFound {
		t.Errorf("unbekannter Termin: erwartet 404, war %d", w.Code)
	}
	if w := anfrage(s.SkipOccurrence, http.MethodPost, "abc", "", map[string]string{"date": "2026-10-02"}); w.Code != http.StatusBadRequest {
		t.Errorf("kaputte ID: erwartet 400, war %d", w.Code)
	}
}

// Zweimal absagen schadet nicht, und das Löschen des Termins räumt die
// Absagen mit ab (Fremdschlüssel).
func TestAbsageIstWiederholbarUndRaeumtAuf(t *testing.T) {
	s := testDienst(t)
	id := legeAn(t, s, RepeatDaily, time.Date(2026, time.October, 1, 18, 0, 0, 0, s.loc))
	for i := 0; i < 2; i++ {
		if w := anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "2026-10-02"}); w.Code != http.StatusOK {
			t.Fatalf("Durchgang %d: Status %d", i, w.Code)
		}
	}
	if w := anfrage(s.DeleteEvent, http.MethodDelete, itoa(id), "", nil); w.Code != http.StatusNoContent {
		t.Fatalf("Löschen: Status %d", w.Code)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM calendar_skips`).Scan(&n); err != nil || n != 0 {
		t.Errorf("Absagen sollten mit dem Termin verschwinden, übrig: %d (%v)", n, err)
	}
}

// ListStored liefert die abgesagten Tage mit, damit die Oberfläche
// „wieder aufnehmen" anbieten kann.
func TestListeNenntAbgesagteTage(t *testing.T) {
	s := testDienst(t)
	id := legeAn(t, s, RepeatDaily, time.Date(2026, time.October, 1, 18, 0, 0, 0, s.loc))
	anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "2026-10-09"})
	anfrage(s.SkipOccurrence, http.MethodPost, itoa(id), "", map[string]string{"date": "2026-10-03"})

	w := httptest.NewRecorder()
	s.ListStored(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	var liste []StoredEvent
	if err := json.Unmarshal(w.Body.Bytes(), &liste); err != nil {
		t.Fatal(err)
	}
	if len(liste) != 1 || strings.Join(liste[0].Skipped, ",") != "2026-10-03,2026-10-09" {
		t.Errorf("abgesagte Tage falsch: %+v", liste)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
