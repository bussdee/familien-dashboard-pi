package rewards

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/notify"
	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

type merker struct{ list []notify.Message }

func (m *merker) Notify(n notify.Message) { m.list = append(m.list, n) }

func TestEinloesungMeldetSichBeiDenEltern(t *testing.T) {
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	db := st.DB()
	s := NewService(db)
	m := &merker{}
	s.SetNotifier(m)

	res, err := db.Exec(`INSERT INTO rewards (title, emoji, cost, active) VALUES ('Eis', '🍦', 40, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	rid, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO point_events (user_id, source, points, note) VALUES (3, 'bonus', 100, '')`); err != nil {
		t.Fatal(err)
	}

	redeem := func() int {
		r := auth.AlsPerson(httptest.NewRequest(http.MethodPost, "/", nil), 3, "member", false)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", strconv.FormatInt(rid, 10))
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
		rec := httptest.NewRecorder()
		s.Redeem(rec, r)
		return rec.Code
	}
	if code := redeem(); code != http.StatusCreated {
		t.Fatalf("Einlösen: %d", code)
	}
	if len(m.list) != 1 || !strings.Contains(m.list[0].Body, "Eis") || !strings.Contains(m.list[0].Body, "40 Punkte") {
		t.Errorf("Nachricht fehlt oder ist falsch: %+v", m.list)
	}
}
