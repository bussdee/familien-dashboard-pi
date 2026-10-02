package points

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"family-dashboard/backend/internal/store"
)

// testDB legt eine echte, frisch migrierte Datenbank an — mit den drei
// Standardpersonen (Papa=1, Mama=2, Kind=3).
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

func buchen(t *testing.T, s *Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/points", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Adjust(rec, req)
	return rec
}

func summe(t *testing.T, db *sql.DB, userID int) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COALESCE(SUM(points), 0) FROM point_events WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGutschreibenUndAbziehen(t *testing.T) {
	db := testDB(t)
	s := NewService(db)

	if rec := buchen(t, s, `{"user_id":3,"points":25,"note":"Toll geholfen"}`); rec.Code != http.StatusCreated {
		t.Fatalf("Gutschrift: %d %s", rec.Code, rec.Body)
	}
	if rec := buchen(t, s, `{"user_id":3,"points":-10}`); rec.Code != http.StatusCreated {
		t.Fatalf("Abzug: %d %s", rec.Code, rec.Body)
	}
	if got := summe(t, db, 3); got != 15 {
		t.Errorf("erwartet 15 Punkte, waren %d", got)
	}

	// Ohne Grund bekommt ein Abzug einen Namen, der auch einer ist.
	var note string
	if err := db.QueryRow(`SELECT note FROM point_events WHERE points < 0`).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if note != "Abzug" {
		t.Errorf("Abzug ohne Grund sollte „Abzug“ heissen, hiess %q", note)
	}
}

// „Alle Kinder +10": eine Buchung, mehrere Einträge, Kennungen zurück.
func TestMehrerePersonenAufEinmal(t *testing.T) {
	db := testDB(t)
	s := NewService(db)

	rec := buchen(t, s, `{"user_ids":[2,3,3],"points":10,"note":"Garten"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Status %d: %s", rec.Code, rec.Body)
	}
	var antwort struct {
		IDs     []int `json:"ids"`
		UserIDs []int `json:"user_ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &antwort); err != nil {
		t.Fatal(err)
	}
	if len(antwort.IDs) != 2 || len(antwort.UserIDs) != 2 {
		t.Errorf("doppelte Person sollte einmal zählen: ids=%v users=%v", antwort.IDs, antwort.UserIDs)
	}
	if summe(t, db, 2) != 10 || summe(t, db, 3) != 10 {
		t.Errorf("beide sollten 10 Punkte haben: Mama %d, Kind %d", summe(t, db, 2), summe(t, db, 3))
	}
}

// Ist eine Person unbekannt, bekommt auch keine der anderen etwas — sonst
// stünde eine halbe Buchung im Verlauf.
func TestUnbekanntePersonBuchtNichts(t *testing.T) {
	db := testDB(t)
	s := NewService(db)

	if rec := buchen(t, s, `{"user_ids":[3,99],"points":10}`); rec.Code != http.StatusNotFound {
		t.Fatalf("erwartet 404, war %d", rec.Code)
	}
	if got := summe(t, db, 3); got != 0 {
		t.Errorf("nichts hätte gebucht werden dürfen, Kind hat %d", got)
	}
}

func TestUngueltigeBuchungen(t *testing.T) {
	s := NewService(testDB(t))
	for _, body := range []string{
		`{"user_id":3,"points":0}`,
		`{"points":10}`,
		`{"user_id":3,"points":20000}`,
		`kaputt`,
	} {
		if rec := buchen(t, s, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: erwartet 400, war %d", body, rec.Code)
		}
	}
}
