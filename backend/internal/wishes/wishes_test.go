package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/rewards"
	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

// Standardpersonen: Papa=1 und Mama=2 (Eltern), Kind=3.
const (
	papa = 1
	kind = 3
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	return st.DB()
}

func aufruf(h http.HandlerFunc, wer int, rolle string, geraet bool, body, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if wer > 0 || geraet {
		r = auth.AlsPerson(r, wer, rolle, geraet)
	}
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	rec := httptest.NewRecorder()
	h(rec, r)
	return rec
}

func wunschAnlegen(t *testing.T, s *Service, wer int, rolle, body string) int {
	t.Helper()
	rec := aufruf(s.Create, wer, rolle, false, body, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("Anlegen: %d %s", rec.Code, rec.Body)
	}
	var out struct{ ID int }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func liste(t *testing.T, s *Service, wer int, rolle string, geraet bool) []Wish {
	t.Helper()
	rec := aufruf(s.List, wer, rolle, geraet, "", "")
	var out []Wish
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort: %v %s", err, rec.Body)
	}
	return out
}

func punkte(t *testing.T, db *sql.DB, userID, n int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO point_events (user_id, source, points, note) VALUES (?, 'bonus', ?, '')`, userID, n); err != nil {
		t.Fatal(err)
	}
}

func stand(t *testing.T, db *sql.DB, id int) string {
	t.Helper()
	var st string
	if err := db.QueryRow(`SELECT status FROM wishes WHERE id = ?`, id).Scan(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestKindTraegtWunschEinUndElternLegenPreisFest(t *testing.T) {
	db := testDB(t)
	s := NewService(db)

	id := wunschAnlegen(t, s, kind, "member", `{"title":"  Ins Kino gehen "}`)
	if stand(t, db, id) != StatusOffen {
		t.Fatal("ein neuer Wunsch hat noch keinen Preis")
	}

	l := liste(t, s, kind, "member", false)
	if len(l) != 1 || l[0].Title != "Ins Kino gehen" || l[0].Cost != nil || l[0].Emoji != vorgabeSym {
		t.Fatalf("Liste: %+v", l)
	}

	// Ohne Preis lässt er sich nicht einlösen, auch mit Guthaben nicht.
	punkte(t, db, kind, 500)
	if rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusConflict {
		t.Fatalf("Einlösen ohne Preis: %d", rec.Code)
	}

	if rec := aufruf(s.Decide, papa, "admin", false, `{"cost":120,"note":"am Wochenende"}`, strconv.Itoa(id)); rec.Code != http.StatusNoContent {
		t.Fatalf("Preis setzen: %d %s", rec.Code, rec.Body)
	}
	l = liste(t, s, kind, "member", false)
	if l[0].Status != StatusBepreist || l[0].Cost == nil || *l[0].Cost != 120 || l[0].Note != "am Wochenende" {
		t.Fatalf("nach dem Preis: %+v", l[0])
	}
}

func TestPreisMussPositivSein(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	id := wunschAnlegen(t, s, kind, "member", `{"title":"Roller"}`)

	for _, body := range []string{`{"cost":0}`, `{"cost":-5}`, `{"cost":100001}`, `{}`} {
		if rec := aufruf(s.Decide, papa, "admin", false, body, strconv.Itoa(id)); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: erwartet 400, bekam %d", body, rec.Code)
		}
	}
	if stand(t, db, id) != StatusOffen {
		t.Error("ein abgelehnter Preis darf nichts ändern")
	}
}

func TestKindSiehtNurEigeneWuenscheElternAlleWandgeraetKeine(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	// Ein zweites Kind.
	if _, err := db.Exec(`INSERT INTO users (id, name, color, pin_hash, role, avatar_emoji) VALUES (4, 'Zweit', '#fff', 'x', 'member', '🙂')`); err != nil {
		t.Fatal(err)
	}
	wunschAnlegen(t, s, kind, "member", `{"title":"A"}`)
	wunschAnlegen(t, s, 4, "member", `{"title":"B"}`)

	if n := len(liste(t, s, kind, "member", false)); n != 1 {
		t.Errorf("Kind sieht %d Wünsche, erwartet 1", n)
	}
	if n := len(liste(t, s, papa, "admin", false)); n != 2 {
		t.Errorf("Eltern sehen %d Wünsche, erwartet 2", n)
	}
	if n := len(liste(t, s, 0, "", true)); n != 0 {
		t.Errorf("Wandgerät sieht %d Wünsche, erwartet 0", n)
	}
}

// Ein Kind darf nur für sich wünschen — und ein Wunsch ist nicht
// weiterzureichen: Einlösen, Zurücknehmen nur für die eigenen.
func TestFremdeWuenscheBleibenUnberuehrt(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	if _, err := db.Exec(`INSERT INTO users (id, name, color, pin_hash, role, avatar_emoji) VALUES (4, 'Zweit', '#fff', 'x', 'member', '🙂')`); err != nil {
		t.Fatal(err)
	}

	if rec := aufruf(s.Create, kind, "member", false, `{"title":"Für die Schwester","user_id":4}`, ""); rec.Code != http.StatusForbidden {
		t.Errorf("fremder Wunsch von einem Kind: %d", rec.Code)
	}
	// Ein Elternteil darf für ein Kind eintragen.
	id := wunschAnlegen(t, s, papa, "admin", `{"title":"Fahrrad","user_id":4}`)
	var owner int
	_ = db.QueryRow(`SELECT user_id FROM wishes WHERE id = ?`, id).Scan(&owner)
	if owner != 4 {
		t.Errorf("Wunsch gehört %d, erwartet 4", owner)
	}

	if rec := aufruf(s.Withdraw, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusConflict {
		t.Errorf("fremden Wunsch zurückgenommen: %d", rec.Code)
	}
	if rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusNotFound {
		t.Errorf("fremden Wunsch eingelöst: %d", rec.Code)
	}
	if rec := aufruf(s.Create, 0, "", true, `{"title":"Am Tablet"}`, ""); rec.Code != http.StatusForbidden {
		t.Errorf("das Wandgerät hat gewünscht: %d", rec.Code)
	}
}

// Der ganze Weg bis zum erfüllten Wunsch, über die Belohnungs-Einlösung.
func TestEinloesenAblehnenUndErfuellen(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	rw := rewards.NewService(db)

	id := wunschAnlegen(t, s, kind, "member", `{"title":"Kino"}`)
	_ = aufruf(s.Decide, papa, "admin", false, `{"cost":100}`, strconv.Itoa(id))

	punkte(t, db, kind, 60)
	rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "fehlen noch 40") {
		t.Fatalf("zu wenig Guthaben: %d %s", rec.Code, rec.Body)
	}

	punkte(t, db, kind, 90) // 150
	rec = aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id))
	if rec.Code != http.StatusCreated {
		t.Fatalf("Einlösen: %d %s", rec.Code, rec.Body)
	}
	var res struct {
		ID      int
		Balance int
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Balance != 50 {
		t.Errorf("Guthaben danach %d, erwartet 50", res.Balance)
	}
	if stand(t, db, id) != StatusAngefragt {
		t.Fatal("der Wunsch muss angefragt sein")
	}
	// Zweimal einlösen geht nicht — die Punkte wären doppelt ausgegeben.
	if rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusConflict {
		t.Errorf("zweites Einlösen: %d", rec.Code)
	}
	// Ein angefragter Wunsch lässt sich nicht mehr neu bepreisen oder
	// zurücknehmen.
	if rec := aufruf(s.Decide, papa, "admin", false, `{"cost":10}`, strconv.Itoa(id)); rec.Code != http.StatusConflict {
		t.Errorf("Preis nach der Anfrage geändert: %d", rec.Code)
	}
	if rec := aufruf(s.Withdraw, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusConflict {
		t.Errorf("angefragten Wunsch zurückgenommen: %d", rec.Code)
	}

	// Eltern lehnen die Einlösung ab: Punkte zurück, Wunsch wieder da.
	entscheide := func(status string) int {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"status":"`+status+`"}`))
		r = auth.AlsPerson(r, papa, "admin", false)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.Itoa(res.ID))
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		rw.Decide(w, r)
		return w.Code
	}
	if c := entscheide("abgelehnt"); c != http.StatusNoContent {
		t.Fatalf("Ablehnen: %d", c)
	}
	if stand(t, db, id) != StatusBepreist {
		t.Fatalf("nach dem Ablehnen: %s", stand(t, db, id))
	}

	// Noch einmal, diesmal eingelöst.
	rec = aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id))
	if rec.Code != http.StatusCreated {
		t.Fatalf("zweiter Versuch: %d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if c := entscheide("eingeloest"); c != http.StatusNoContent {
		t.Fatalf("Einlösen bestätigen: %d", c)
	}
	if stand(t, db, id) != StatusErfuellt {
		t.Fatalf("nach der Bestätigung: %s", stand(t, db, id))
	}
}

func TestAblehnenMitHinweis(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	id := wunschAnlegen(t, s, kind, "member", `{"title":"Smartphone"}`)

	if rec := aufruf(s.Decide, papa, "admin", false, `{"reject":true,"note":"Erst ab 12"}`, strconv.Itoa(id)); rec.Code != http.StatusNoContent {
		t.Fatalf("Ablehnen: %d %s", rec.Code, rec.Body)
	}
	l := liste(t, s, kind, "member", false)
	if l[0].Status != StatusAbgelehnt || l[0].Note != "Erst ab 12" || l[0].Cost != nil {
		t.Fatalf("abgelehnter Wunsch: %+v", l[0])
	}
	if rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusConflict {
		t.Errorf("abgelehnten Wunsch eingelöst: %d", rec.Code)
	}
	// Das Kind kann ihn wegräumen.
	if rec := aufruf(s.Withdraw, kind, "member", false, "", strconv.Itoa(id)); rec.Code != http.StatusNoContent {
		t.Errorf("Wunsch wegräumen: %d", rec.Code)
	}
}

func TestZurueckziehenDerAnfrageBringtWunschZurueck(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	rw := rewards.NewService(db)

	id := wunschAnlegen(t, s, kind, "member", `{"title":"Eis"}`)
	_ = aufruf(s.Decide, papa, "admin", false, `{"cost":20}`, strconv.Itoa(id))
	punkte(t, db, kind, 50)
	rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id))
	var res struct{ ID int }
	_ = json.Unmarshal(rec.Body.Bytes(), &res)

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = auth.AlsPerson(r, kind, "member", false)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(res.ID))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	rw.Cancel(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("Zurückziehen: %d %s", w.Code, w.Body)
	}
	if stand(t, db, id) != StatusBepreist {
		t.Errorf("Wunsch nach dem Zurückziehen: %s", stand(t, db, id))
	}
}

func TestTitelGrenzen(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	for _, body := range []string{`{"title":""}`, `{"title":"   "}`, `{"title":"` + strings.Repeat("x", 81) + `"}`, `{"title":"a","emoji":"123456789"}`} {
		if rec := aufruf(s.Create, kind, "member", false, body, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%.30s…: erwartet 400, bekam %d", body, rec.Code)
		}
	}
}

func TestWunschlisteHatEineGrenze(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	for i := 0; i < maxOffene; i++ {
		wunschAnlegen(t, s, kind, "member", `{"title":"Wunsch `+strconv.Itoa(i)+`"}`)
	}
	if rec := aufruf(s.Create, kind, "member", false, `{"title":"Einer zu viel"}`, ""); rec.Code != http.StatusConflict {
		t.Errorf("Wunsch über der Grenze: %d", rec.Code)
	}
}
