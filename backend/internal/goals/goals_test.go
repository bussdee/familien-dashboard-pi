package goals

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

	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
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

func anlegen(t *testing.T, s *Service, body string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Create(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("Anlegen: %d %s", rec.Code, rec.Body)
	}
	var out struct{ ID int }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func lesen(t *testing.T, s *Service) Overview {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Get(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var o Overview
	if err := json.Unmarshal(rec.Body.Bytes(), &o); err != nil {
		t.Fatalf("Antwort: %v %s", err, rec.Body)
	}
	return o
}

func punkte(t *testing.T, db *sql.DB, userID, n int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO point_events (user_id, source, points, note) VALUES (?, 'bonus', ?, '')`, userID, n); err != nil {
		t.Fatal(err)
	}
}

// Gezählt wird, was nach dem Start verdient wurde — von allen zusammen.
// Abzüge zählen nicht dagegen.
func TestFortschrittZaehltNurVerdientes(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	// Punkte von vor dem Start gehören nicht dazu.
	if _, err := db.Exec(`INSERT INTO point_events (user_id, source, points, note, created_at)
		VALUES (3, 'bonus', 500, '', datetime('now', '-1 day'))`); err != nil {
		t.Fatal(err)
	}
	anlegen(t, s, `{"title":"Pizza-Abend","emoji":"🍕","target":100}`)
	punkte(t, db, 3, 40)
	punkte(t, db, 2, 30)
	punkte(t, db, 3, -20)

	o := lesen(t, s)
	if o.Goal == nil || o.Goal.Title != "Pizza-Abend" {
		t.Fatalf("kein laufendes Ziel: %+v", o)
	}
	if o.Progress != 70 {
		t.Errorf("erwartet 70 (40+30, Abzug zählt nicht), war %d", o.Progress)
	}
	if len(o.Contributions) != 2 {
		t.Errorf("zwei haben beigetragen, gezählt: %d", len(o.Contributions))
	}
	if o.Goal.ReachedAt != nil {
		t.Error("70 von 100 ist noch nicht geschafft")
	}
}

func TestGeschafftUndAbgeschlossen(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	id := anlegen(t, s, `{"title":"Kino","target":50}`)
	punkte(t, db, 3, 60)

	o := lesen(t, s)
	if o.Goal == nil || o.Goal.ReachedAt == nil {
		t.Fatalf("60 von 50 sollte geschafft sein: %+v", o.Goal)
	}
	if o.Goal.Emoji != "🎯" {
		t.Errorf("ohne Symbol erwartet 🎯, war %q", o.Goal.Emoji)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(id))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	s.Close(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Abschliessen: %d", rec.Code)
	}

	o = lesen(t, s)
	if o.Goal != nil {
		t.Error("nach dem Abschliessen sollte kein Ziel mehr laufen")
	}
	if len(o.Reached) != 1 || o.Reached[0].Title != "Kino" {
		t.Errorf("Kino sollte unter den erreichten Zielen stehen: %+v", o.Reached)
	}
}

// Es gibt immer nur ein laufendes Ziel. Ein neues schliesst das alte ab.
func TestNeuesZielErsetztDasAlte(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	anlegen(t, s, `{"title":"Erstes","target":100}`)
	anlegen(t, s, `{"title":"Zweites","target":100}`)

	var offen int
	_ = db.QueryRow(`SELECT COUNT(*) FROM family_goals WHERE closed_at IS NULL`).Scan(&offen)
	if offen != 1 {
		t.Errorf("erwartet genau ein laufendes Ziel, waren %d", offen)
	}
	if o := lesen(t, s); o.Goal == nil || o.Goal.Title != "Zweites" {
		t.Errorf("laufen sollte das zweite: %+v", o.Goal)
	}
}

func TestUngueltigeZiele(t *testing.T) {
	s := NewService(testDB(t))
	for _, body := range []string{`{"title":"","target":10}`, `{"title":"X","target":0}`, `{"title":"X","target":-5}`} {
		rec := httptest.NewRecorder()
		s.Create(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: erwartet 400, war %d", body, rec.Code)
		}
	}
}
