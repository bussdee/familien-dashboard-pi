package wishes

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"family-dashboard/backend/internal/notify"
	"family-dashboard/backend/internal/rewards"
)

// merker sammelt, was die Dienste melden wollen.
type merker struct {
	mu   sync.Mutex
	list []notify.Message
}

func (m *merker) Notify(n notify.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.list = append(m.list, n)
}

func (m *merker) anzahl() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.list)
}

// Die Eltern erfahren von einem Wunsch des Kindes — nicht von dem, den sie
// selbst eintragen.
func TestNeuerWunschMeldetSichBeiDenEltern(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	m := &merker{}
	s.SetNotifier(m)

	wunschAnlegen(t, s, kind, "member", `{"title":"Ins Kino gehen"}`)
	if m.anzahl() != 1 {
		t.Fatalf("erwartet 1 Nachricht, waren %d", m.anzahl())
	}
	if got := m.list[0]; !strings.Contains(got.Body, "Ins Kino gehen") || got.Title != "Neuer Wunsch" {
		t.Errorf("Nachricht: %+v", got)
	}

	wunschAnlegen(t, s, papa, "admin", `{"title":"Fahrrad","user_id":3}`)
	if m.anzahl() != 1 {
		t.Errorf("ein Wunsch, den ein Elternteil einträgt, braucht keine Nachricht (waren %d)", m.anzahl())
	}
}

func TestEinloesungMeldetSichBeiDenEltern(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	m := &merker{}
	s.SetNotifier(m)

	id := wunschAnlegen(t, s, kind, "member", `{"title":"Kino"}`)
	_ = aufruf(s.Decide, papa, "admin", false, `{"cost":100}`, strconv.Itoa(id))
	punkte(t, db, kind, 150)
	before := m.anzahl()

	// Zu wenig Guthaben meldet nichts; hier reicht es.
	if rec := aufruf(s.Redeem, kind, "member", false, "", strconv.Itoa(id)); rec.Code != 201 {
		t.Fatalf("Einlösen: %d %s", rec.Code, rec.Body)
	}
	if m.anzahl() != before+1 || !strings.Contains(m.list[len(m.list)-1].Body, "100 Punkte") {
		t.Errorf("Nachricht zur Einlösung fehlt: %+v", m.list)
	}
}

func TestOhneNotifierBleibtAlleswieBisher(t *testing.T) {
	db := testDB(t)
	s := NewService(db) // Voreinstellung: aus
	s.SetNotifier(nil)  // nil ändert nichts
	wunschAnlegen(t, s, kind, "member", `{"title":"Kino"}`)
	_ = rewards.NewService(db) // Voreinstellung ohne Notifier darf nicht abstürzen
}
