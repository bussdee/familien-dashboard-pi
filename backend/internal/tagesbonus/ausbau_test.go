package tagesbonus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"family-dashboard/backend/internal/auth"
)

func summeQuelle(t *testing.T, s *Service, wer int, quelle string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(points), 0) FROM point_events WHERE user_id = ? AND source = ?`, wer, quelle).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMeilensteinBeiSiebenTagen(t *testing.T) {
	s, u := testService(t, schalter{"modul.tagesbonus": "1", "bonus.meilenstein": "10"})
	var st Stand
	for i := 1; i <= 7; i++ {
		st = abholen(t, s, kind, "member", false)
		if i == 5 && (st.NaechsterMeilenstein != 7 || st.TageBisMeilenstein != 2) {
			t.Errorf("Tag 5: nächster Meilenstein %d in %d Tagen", st.NaechsterMeilenstein, st.TageBisMeilenstein)
		}
		if i < 7 && st.Meilenstein != 0 {
			t.Errorf("Tag %d meldet Meilenstein %d", i, st.Meilenstein)
		}
		u.tage(1)
	}
	if st.Meilenstein != 7 || st.MeilensteinPunkte != 10 || st.NaechsterMeilenstein != 14 {
		t.Fatalf("Tag 7: %+v", st)
	}
	if got := summeQuelle(t, s, kind, QuelleMeilenstein); got != 10 {
		t.Errorf("Meilenstein-Punkte: %d", got)
	}
	// Ohne Meilenstein-Bonus (0) gibt es keine Extra-Punkte.
	s0, u0 := testService(t, schalter{"modul.tagesbonus": "1", "bonus.meilenstein": "0"})
	for i := 0; i < 7; i++ {
		abholen(t, s0, kind, "member", false)
		u0.tage(1)
	}
	if got := summeQuelle(t, s0, kind, QuelleMeilenstein); got != 0 {
		t.Errorf("Meilenstein mit 0: %d", got)
	}
}

func TestSerienschutzRettetEinenTagProWoche(t *testing.T) {
	s, u := testService(t, schalter{"modul.tagesbonus": "1", "bonus.joker": "1"})
	abholen(t, s, kind, "member", false) // Tag 1
	u.tage(1)
	abholen(t, s, kind, "member", false) // Tag 2
	u.tage(2)                            // einen Tag verpasst
	st := abholen(t, s, kind, "member", false)
	if !st.JokerGenutzt || st.Serie != 4 || st.Punkte != 4 {
		t.Fatalf("mit Joker: %+v", st)
	}
	if st.JokerFrei {
		t.Error("der Joker sollte jetzt verbraucht sein")
	}
	// Der gerettete Tag bringt keine Punkte.
	if got := summe(t, s, kind); got != 1+2+4 {
		t.Errorf("Summe: %d", got)
	}
	// Innerhalb einer Woche noch eine Lücke: kein zweiter Joker.
	u.tage(2)
	st = abholen(t, s, kind, "member", false)
	if st.JokerGenutzt || st.Serie != 1 {
		t.Fatalf("zweite Lücke in derselben Woche: %+v", st)
	}
	// Zwei verpasste Tage rettet auch ein Joker nicht.
	s2, u2 := testService(t, schalter{"modul.tagesbonus": "1", "bonus.joker": "1"})
	abholen(t, s2, kind, "member", false)
	u2.tage(3)
	if st := abholen(t, s2, kind, "member", false); st.JokerGenutzt || st.Serie != 1 {
		t.Fatalf("zwei Tage Lücke: %+v", st)
	}
	// Ohne Schalter kein Joker.
	s3, u3 := testService(t, an)
	abholen(t, s3, kind, "member", false)
	u3.tage(2)
	if st := abholen(t, s3, kind, "member", false); st.JokerGenutzt || st.Serie != 1 {
		t.Fatalf("ohne Serienschutz: %+v", st)
	}
}

func TestKalender(t *testing.T) {
	s, u := testService(t, schalter{"modul.tagesbonus": "1", "bonus.joker": "1"})
	abholen(t, s, kind, "member", false) // 2.11.
	u.tage(2)                            // 3.11. verpasst, Joker
	abholen(t, s, kind, "member", false) // 4.11.

	r := auth.AlsPerson(httptest.NewRequest(http.MethodGet, "/?monat=2026-11", nil), kind, "member", false)
	w := httptest.NewRecorder()
	s.Kalender(w, r)
	var k struct {
		Days []KalenderTag `json:"days"`
		Best int           `json:"best"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &k)
	if len(k.Days) != 3 || !k.Days[1].Joker || k.Days[1].Tag != "2026-11-03" || k.Best != 3 {
		t.Fatalf("Kalender: %+v", k)
	}
	r = auth.AlsPerson(httptest.NewRequest(http.MethodGet, "/?monat=kaputt", nil), kind, "member", false)
	w = httptest.NewRecorder()
	s.Kalender(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("ungültiger Monat: %d", w.Code)
	}
}

// ----------------------------------------------------------- Tagesaufgabe

func aufgabeHolen(t *testing.T, s *Service, wer int, rolle string) AufgabeStand {
	t.Helper()
	r := auth.AlsPerson(httptest.NewRequest(http.MethodGet, "/", nil), wer, rolle, false)
	w := httptest.NewRecorder()
	s.Aufgabe(w, r)
	var st AufgabeStand
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	return st
}

// tagMit sucht einen Tag, an dem die gewünschte Aufgabe dran ist.
func tagMit(t *testing.T, id string) time.Time {
	t.Helper()
	d := time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)
	for i := 0; i < 400; i++ {
		if aufgabeFuer(d.Format(tagFormat)).ID == id {
			return d
		}
		d = d.AddDate(0, 0, 1)
	}
	t.Fatalf("kein Tag mit Aufgabe %s", id)
	return d
}

func erledigt(t *testing.T, s *Service, wer int, wann time.Time, punkte int, wartet bool) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO chore_completions (chore_id, user_id, completed_at, verified, points_awarded, pending)
		VALUES ((SELECT MIN(id) FROM chores), ?, ?, 1, ?, ?)`, wer, wann, punkte, wartet); err != nil {
		t.Fatal(err)
	}
}

func TestTagesaufgabeZwei(t *testing.T) {
	s, u := testService(t, schalter{"modul.tagesaufgabe": "1", "bonus.aufgabe": "5"})
	u.t = tagMit(t, "zwei")

	if st := aufgabeHolen(t, s, kind, "member"); !st.Berechtigt || st.ID != "zwei" || st.Ziel != 2 || st.Fortschritt != 0 || st.Geschafft {
		t.Fatalf("zu Beginn: %+v", st)
	}
	erledigt(t, s, kind, u.t.Add(-time.Hour), 10, false)
	erledigt(t, s, kind, u.t.Add(-30*time.Minute), 5, true) // wartet noch auf die Eltern: zählt nicht
	erledigt(t, s, kind, u.t.AddDate(0, 0, -1), 5, false)   // gestern: zählt nicht
	if st := aufgabeHolen(t, s, kind, "member"); st.Fortschritt != 1 || st.Geschafft {
		t.Fatalf("nach einer: %+v", st)
	}
	erledigt(t, s, kind, u.t.Add(-10*time.Minute), 5, false)
	st := aufgabeHolen(t, s, kind, "member")
	if !st.Neu || !st.Geschafft || st.Fortschritt != 2 || st.Punkte != 5 {
		t.Fatalf("geschafft: %+v", st)
	}
	if got := summeQuelle(t, s, kind, QuelleAufgabe); got != 5 {
		t.Errorf("Punkte: %d", got)
	}
	// Nur einmal.
	if st := aufgabeHolen(t, s, kind, "member"); st.Neu || !st.Geschafft {
		t.Fatalf("zweite Abfrage: %+v", st)
	}
	if got := summeQuelle(t, s, kind, QuelleAufgabe); got != 5 {
		t.Errorf("doppelt: %d", got)
	}
}

func TestTagesaufgabeVormittagUndPunkte(t *testing.T) {
	s, u := testService(t, schalter{"modul.tagesaufgabe": "1", "bonus.aufgabe": "5"})
	tag := tagMit(t, "vormittag")
	u.t = time.Date(tag.Year(), tag.Month(), tag.Day(), 15, 0, 0, 0, time.Local)
	erledigt(t, s, kind, u.t.Add(-time.Hour), 10, false) // 14 Uhr: zu spät
	if st := aufgabeHolen(t, s, kind, "member"); st.Geschafft {
		t.Fatalf("nachmittags erledigt zählt nicht: %+v", st)
	}
	erledigt(t, s, kind, time.Date(tag.Year(), tag.Month(), tag.Day(), 10, 0, 0, 0, time.Local), 5, false)
	if st := aufgabeHolen(t, s, kind, "member"); !st.Neu {
		t.Fatalf("vormittags erledigt: %+v", st)
	}

	s2, u2 := testService(t, schalter{"modul.tagesaufgabe": "1", "bonus.aufgabe": "5"})
	u2.t = tagMit(t, "punkte15")
	erledigt(t, s2, kind, u2.t.Add(-time.Hour), 10, false)
	if st := aufgabeHolen(t, s2, kind, "member"); st.Fortschritt != 10 || st.Geschafft {
		t.Fatalf("10 von 15: %+v", st)
	}
	erledigt(t, s2, kind, u2.t.Add(-time.Minute), 5, false)
	if st := aufgabeHolen(t, s2, kind, "member"); !st.Neu || st.Fortschritt != 15 {
		t.Fatalf("15 von 15: %+v", st)
	}
}

func TestTagesaufgabeNurFuerKinderUndWennAn(t *testing.T) {
	s, _ := testService(t, schalter{"modul.tagesaufgabe": "1"})
	if st := aufgabeHolen(t, s, papa, "admin"); st.Berechtigt {
		t.Errorf("Eltern: %+v", st)
	}
	aus, _ := testService(t, schalter{})
	if st := aufgabeHolen(t, aus, kind, "member"); st.Berechtigt {
		t.Errorf("ausgeschaltet: %+v", st)
	}
}

// Jede Aufgabe im Vorrat kommt vor, und die Auswahl ist für einen Tag fest.
func TestAufgabenVorrat(t *testing.T) {
	gesehen := map[string]bool{}
	d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 120; i++ {
		gesehen[aufgabeFuer(d.Format(tagFormat)).ID] = true
		d = d.AddDate(0, 0, 1)
	}
	if len(gesehen) != len(Aufgaben) {
		t.Errorf("in 120 Tagen nur %d von %d Aufgaben", len(gesehen), len(Aufgaben))
	}
	if aufgabeFuer("2026-11-12") != aufgabeFuer("2026-11-12") {
		t.Error("nicht fest")
	}
}
