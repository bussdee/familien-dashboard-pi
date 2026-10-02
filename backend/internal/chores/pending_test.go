package chores

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

// Standardpersonen: Papa=1 und Mama=2 (Eltern), Kind=3.
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

// mitID ruft einen Handler auf, der {id} aus der Route liest.
func mitID(h http.HandlerFunc, id string, r *http.Request) *httptest.ResponseRecorder {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h(rec, r)
	return rec
}

func itoa(n int) string { return strconv.Itoa(n) }

func neueAufgabe(t *testing.T, db *sql.DB, pruefen bool) int {
	t.Helper()
	res, err := db.Exec(`INSERT INTO chores (title, interval_days, points, assignment, next_due_at, needs_check)
		VALUES ('Zimmer aufräumen', 7, 15, 'nobody', datetime('now', '-1 hour'), ?)`, pruefen)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

func abhaken(t *testing.T, s *Service, choreID int, userID int, role string) map[string]any {
	t.Helper()
	r := auth.AlsPerson(httptest.NewRequest(http.MethodPost, "/", nil), userID, role, false)
	rec := mitID(s.Complete, itoa(choreID), r)
	if rec.Code != http.StatusOK {
		t.Fatalf("Abhaken: %d %s", rec.Code, rec.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func punkteVon(t *testing.T, db *sql.DB, userID int) int {
	t.Helper()
	var n int
	_ = db.QueryRow(`SELECT COALESCE(SUM(points),0) FROM point_events WHERE user_id = ?`, userID).Scan(&n)
	return n
}

func offeneErledigung(t *testing.T, db *sql.DB) int {
	t.Helper()
	var id int
	if err := db.QueryRow(`SELECT id FROM chore_completions WHERE pending = 1`).Scan(&id); err != nil {
		t.Fatalf("keine ausstehende Erledigung: %v", err)
	}
	return id
}

// Ein Kind hakt ab: Die Aufgabe ist erledigt, die Punkte warten. Erst die
// Bestätigung bringt sie — und zwar genau einmal.
func TestKindWartetAufBestaetigung(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	chore := neueAufgabe(t, db, true)

	out := abhaken(t, s, chore, 3, "member")
	if out["pending"] != true || out["points_awarded"].(float64) != 0 {
		t.Fatalf("erwartet wartend ohne Punkte, war %v", out)
	}
	if punkteVon(t, db, 3) != 0 {
		t.Fatal("vor der Bestätigung darf es keine Punkte geben")
	}
	c, _ := s.byID(chore)
	if !c.PendingCheck || c.IsDue {
		t.Errorf("Aufgabe sollte als wartend und nicht mehr fällig gelten: %+v", c)
	}

	cc := offeneErledigung(t, db)
	if rec := mitID(s.Approve, itoa(cc), httptest.NewRequest(http.MethodPost, "/", nil)); rec.Code != http.StatusOK {
		t.Fatalf("Bestätigen: %d %s", rec.Code, rec.Body)
	}
	if got := punkteVon(t, db, 3); got != 15 {
		t.Errorf("nach der Bestätigung erwartet 15 Punkte, waren %d", got)
	}
	if rec := mitID(s.Approve, itoa(cc), httptest.NewRequest(http.MethodPost, "/", nil)); rec.Code != http.StatusConflict {
		t.Errorf("zweites Bestätigen erwartet 409, war %d", rec.Code)
	}
	if got := punkteVon(t, db, 3); got != 15 {
		t.Errorf("zweites Bestätigen darf nichts doppelt buchen: %d", got)
	}
}

// Abgelehnt heisst: noch nicht fertig. Die Aufgabe ist wieder fällig, Punkte
// gab es keine.
func TestAblehnenMachtWiederFaellig(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	chore := neueAufgabe(t, db, true)
	abhaken(t, s, chore, 3, "member")

	cc := offeneErledigung(t, db)
	if rec := mitID(s.Reject, itoa(cc), httptest.NewRequest(http.MethodPost, "/", nil)); rec.Code != http.StatusNoContent {
		t.Fatalf("Ablehnen: %d %s", rec.Code, rec.Body)
	}
	c, _ := s.byID(chore)
	if !c.IsDue || c.PendingCheck {
		t.Errorf("nach dem Ablehnen sollte die Aufgabe wieder fällig sein: %+v", c)
	}
	if punkteVon(t, db, 3) != 0 {
		t.Error("abgelehnt darf keine Punkte bringen")
	}
}

// Eltern brauchen sich nicht selbst zu bestätigen, und Aufgaben ohne die
// Markierung verhalten sich wie immer.
func TestOhneBestaetigung(t *testing.T) {
	db := testDB(t)
	s := NewService(db)

	out := abhaken(t, s, neueAufgabe(t, db, true), 2, "admin")
	if out["pending"] == true || punkteVon(t, db, 2) != 15 {
		t.Errorf("Elternteil sollte die Punkte sofort bekommen: %v, %d", out, punkteVon(t, db, 2))
	}
	out = abhaken(t, s, neueAufgabe(t, db, false), 3, "member")
	if out["pending"] == true || punkteVon(t, db, 3) != 15 {
		t.Errorf("ohne Markierung sollte das Kind die Punkte sofort bekommen: %v", out)
	}
}
