package tagesbonus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/einstellungen"
	"family-dashboard/backend/internal/points"
	"family-dashboard/backend/internal/store"
)

// Standardpersonen: Papa=1 (admin), Mama=2 (admin), Kind=3 (member).
const (
	papa = 1
	kind = 3
)

type schalter map[string]string

func (s schalter) An(k string) bool     { return s[k] == "1" }
func (s schalter) Wert(k string) string { return s[k] }

type uhr struct{ t time.Time }

func (u *uhr) jetzt() time.Time { return u.t }
func (u *uhr) tage(n int)       { u.t = u.t.AddDate(0, 0, n) }

func testService(t *testing.T, sw schalter) (*Service, *uhr) {
	t.Helper()
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	u := &uhr{t: time.Date(2026, 11, 2, 7, 30, 0, 0, time.Local)}
	s := NewService(st.DB(), sw)
	s.jetzt = u.jetzt
	return s, u
}

func abholen(t *testing.T, s *Service, wer int, rolle string, geraet bool) Stand {
	t.Helper()
	r := auth.AlsPerson(httptest.NewRequest(http.MethodPost, "/", nil), wer, rolle, geraet)
	w := httptest.NewRecorder()
	s.Claim(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("Abholen: %d %s", w.Code, w.Body)
	}
	var st Stand
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	return st
}

func stand(t *testing.T, s *Service, wer int) Stand {
	t.Helper()
	r := auth.AlsPerson(httptest.NewRequest(http.MethodGet, "/", nil), wer, "member", false)
	w := httptest.NewRecorder()
	s.Get(w, r)
	var st Stand
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	return st
}

func summe(t *testing.T, s *Service, wer int) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(points), 0) FROM point_events WHERE user_id = ? AND source = ?`, wer, Quelle).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var an = schalter{"modul.tagesbonus": "1"}

// Tag 1 → 1 Punkt, Tag 2 → 2, Tag 3 → 3; fehlt ein Tag, wieder 1.
func TestSerieWaechstUndBeginntNachLueckeNeu(t *testing.T) {
	s, u := testService(t, an)

	erwartet := []int{1, 2, 3, 4}
	for i, p := range erwartet {
		st := abholen(t, s, kind, "member", false)
		if !st.Neu || st.Punkte != p || st.Serie != i+1 || st.Morgen != p+1 {
			t.Fatalf("Tag %d: %+v", i+1, st)
		}
		u.tage(1)
	}
	if got := summe(t, s, kind); got != 1+2+3+4 {
		t.Errorf("Summe nach vier Tagen: %d", got)
	}

	// Ein Tag fehlt (u steht schon auf Tag 5, wir springen auf Tag 6).
	u.tage(1)
	st := abholen(t, s, kind, "member", false)
	if st.Serie != 1 || st.Punkte != 1 {
		t.Fatalf("nach einer Lücke: %+v", st)
	}
}

func TestNurEinmalProTag(t *testing.T) {
	s, u := testService(t, an)
	if st := abholen(t, s, kind, "member", false); !st.Neu {
		t.Fatal("erster Aufruf sollte gutschreiben")
	}
	// Später am selben Tag, zweites Gerät: nichts Neues.
	u.t = u.t.Add(10 * time.Hour)
	st := abholen(t, s, kind, "member", false)
	if st.Neu || !st.HeuteAbgeholt || st.Punkte != 1 {
		t.Fatalf("zweiter Aufruf: %+v", st)
	}
	if got := summe(t, s, kind); got != 1 {
		t.Errorf("doppelt gutgeschrieben: %d", got)
	}
}

func TestStandOhneAbholen(t *testing.T) {
	s, u := testService(t, an)
	if st := stand(t, s, kind); !st.Berechtigt || st.HeuteAbgeholt || st.Serie != 0 || st.Punkte != 1 {
		t.Fatalf("vor dem ersten Mal: %+v", st)
	}
	abholen(t, s, kind, "member", false)
	u.tage(1)
	// Gestern abgeholt, heute noch nicht: Serie 1, heute gibt es 2.
	if st := stand(t, s, kind); st.HeuteAbgeholt || st.Serie != 1 || st.Punkte != 2 || st.Morgen != 3 {
		t.Fatalf("am Morgen danach: %+v", st)
	}
	if got := summe(t, s, kind); got != 1 {
		t.Errorf("Stand darf nichts gutschreiben: %d", got)
	}
}

func TestObergrenze(t *testing.T) {
	s, u := testService(t, schalter{"modul.tagesbonus": "1", "bonus.max": "3"})
	var letzte Stand
	for i := 0; i < 5; i++ {
		letzte = abholen(t, s, kind, "member", false)
		u.tage(1)
	}
	if letzte.Serie != 5 || letzte.Punkte != 3 || letzte.Morgen != 3 {
		t.Fatalf("mit Obergrenze 3: %+v", letzte)
	}
	if got := summe(t, s, kind); got != 1+2+3+3+3 {
		t.Errorf("Summe: %d", got)
	}
}

// Eltern, das Wandgerät und ein ausgeschalteter Bereich bekommen nichts.
func TestNurKinderUndNurWennEingeschaltet(t *testing.T) {
	s, _ := testService(t, an)
	if st := abholen(t, s, papa, "admin", false); st.Berechtigt || st.Neu {
		t.Errorf("Eltern: %+v", st)
	}
	if st := abholen(t, s, 0, "device", true); st.Berechtigt || st.Neu {
		t.Errorf("Wandgerät: %+v", st)
	}
	if got := summe(t, s, papa); got != 0 {
		t.Errorf("Eltern haben Punkte bekommen: %d", got)
	}

	aus, _ := testService(t, schalter{})
	if st := abholen(t, aus, kind, "member", false); st.Berechtigt || st.Neu {
		t.Errorf("ausgeschaltet: %+v", st)
	}
	if got := summe(t, aus, kind); got != 0 {
		t.Errorf("ausgeschaltet gutgeschrieben: %d", got)
	}
}

// Der Bonus zählt wie verdiente Punkte: Guthaben und Level steigen.
func TestBonusErhoehtGuthaben(t *testing.T) {
	s, u := testService(t, an)
	for i := 0; i < 3; i++ {
		abholen(t, s, kind, "member", false)
		u.tage(1)
	}
	b, err := points.Balance(s.db, kind)
	if err != nil {
		t.Fatal(err)
	}
	if b != 6 {
		t.Errorf("Guthaben: %d, erwartet 6", b)
	}
}

// Mit den echten Einstellungen aus der Datenbank: Die Datenbank hat nur eine
// Verbindung. Fragte die Abholung innerhalb ihrer Transaktion nach der
// Obergrenze, wartete sie ewig auf sich selbst.
func TestMitEchtenEinstellungenKeinStillstand(t *testing.T) {
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting("modul.tagesbonus", "1"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting("bonus.max", "2"); err != nil {
		t.Fatal(err)
	}
	s := NewService(st.DB(), einstellungen.NewService(st))

	fertig := make(chan Stand, 1)
	go func() { fertig <- abholen(t, s, kind, "member", false) }()
	select {
	case got := <-fertig:
		if !got.Neu || got.Punkte != 1 || got.Morgen != 2 {
			t.Fatalf("Abholung: %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("die Abholung hängt (Datenbank wartet auf sich selbst)")
	}
	if got := stand(t, s, kind); !got.HeuteAbgeholt {
		t.Fatalf("Stand danach: %+v", got)
	}
}
