package store

import (
	"path/filepath"
	"testing"
)

func neu(t *testing.T) *Store {
	t.Helper()
	st, err := NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func tabellen(t *testing.T, st *Store) map[string]bool {
	t.Helper()
	rows, err := st.DB().Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		out[n] = true
	}
	return out
}

// Die Migration darf beliebig oft laufen: Jeder Neustart des Servers tut es.
func TestMigrationIstWiederholbar(t *testing.T) {
	st := neu(t)
	for i := 0; i < 3; i++ {
		if err := st.Migrate(); err != nil {
			t.Fatalf("Durchgang %d: %v", i+1, err)
		}
	}
	var personen int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&personen); err != nil {
		t.Fatal(err)
	}
	if personen != 3 {
		t.Errorf("die Standardpersonen dürfen sich nicht vermehren: %d", personen)
	}
	for _, name := range []string{
		"calendar_events", "calendar_skips", "calendar_subscriptions", "login_attempts_source", "wishes",
	} {
		if !tabellen(t, st)[name] {
			t.Errorf("Tabelle %s fehlt", name)
		}
	}
}

// Eine Datenbank im Stand von 2.3.0 kennt die neuen Tabellen nicht. Der Weg
// nach oben darf nichts von dem berühren, was die Familie eingetragen hat.
func TestAufstiegVon23(t *testing.T) {
	st := neu(t)
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	db := st.DB()

	// Familiendaten, wie sie nach Monaten im Alltag aussehen.
	if _, err := db.Exec(`INSERT INTO calendar_events (title, start_at, end_at, repeat) VALUES ('Klavier', '2026-10-01 18:00:00', '2026-10-01 19:00:00', 'weekly')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO point_events (user_id, source, points, note) VALUES (3, 'bonus', 42, 'Mut')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO wishes (user_id, title) VALUES (3, 'Kino')`); err != nil {
		t.Fatal(err)
	}

	// Zurück in den Stand von 2.3.0: die in 2.4.0 neuen Tabellen gibt es nicht.
	for _, tbl := range []string{"calendar_skips", "calendar_subscriptions", "login_attempts_source"} {
		if _, err := db.Exec(`DROP TABLE ` + tbl); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := st.Migrate(); err != nil {
			t.Fatalf("Aufstieg, Durchgang %d: %v", i+1, err)
		}
	}

	var titel string
	if err := db.QueryRow(`SELECT title FROM calendar_events`).Scan(&titel); err != nil || titel != "Klavier" {
		t.Errorf("Termin verändert: %q (%v)", titel, err)
	}
	var punkte int
	if err := db.QueryRow(`SELECT SUM(points) FROM point_events WHERE user_id = 3`).Scan(&punkte); err != nil || punkte != 42 {
		t.Errorf("Punkte verändert: %d (%v)", punkte, err)
	}
	var wuensche int
	if err := db.QueryRow(`SELECT COUNT(*) FROM wishes`).Scan(&wuensche); err != nil || wuensche != 1 {
		t.Errorf("Wünsche verändert: %d (%v)", wuensche, err)
	}
	for _, name := range []string{"calendar_skips", "calendar_subscriptions", "login_attempts_source"} {
		if !tabellen(t, st)[name] {
			t.Errorf("Tabelle %s wurde nicht angelegt", name)
		}
	}
}

// Wird ein Termin gelöscht, verschwinden seine Absagen mit (Fremdschlüssel).
func TestAbsagenGehenMitDemTerminUnter(t *testing.T) {
	st := neu(t)
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	db := st.DB()
	res, err := db.Exec(`INSERT INTO calendar_events (title, start_at, end_at, repeat) VALUES ('Tabletten', '2026-10-01 08:00:00', '2026-10-01 08:15:00', 'daily')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO calendar_skips (event_id, day) VALUES (?, '2026-10-05')`, id); err != nil {
		t.Fatal(err)
	}
	// Ein Tag lässt sich nur einmal absagen.
	if _, err := db.Exec(`INSERT INTO calendar_skips (event_id, day) VALUES (?, '2026-10-05')`, id); err == nil {
		t.Error("derselbe Tag zweimal sollte scheitern")
	}
	// Und nicht für einen Termin, den es nicht gibt.
	if _, err := db.Exec(`INSERT INTO calendar_skips (event_id, day) VALUES (9999, '2026-10-05')`); err == nil {
		t.Error("Absage ohne Termin sollte scheitern")
	}
	if _, err := db.Exec(`DELETE FROM calendar_events WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM calendar_skips`).Scan(&n)
	if n != 0 {
		t.Errorf("Absagen sollten mit dem Termin verschwinden, übrig: %d", n)
	}
}

func TestEinstellungenJePerson(t *testing.T) {
	st := neu(t)
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, found, err := st.UserSetting(1, "dashboard.layout"); err != nil || found {
		t.Fatalf("unerwartet gefunden: %v %v", found, err)
	}
	if err := st.SetUserSetting(1, "dashboard.layout", `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserSetting(1, "dashboard.layout", `{"a":2}`); err != nil {
		t.Fatal(err)
	}
	v, found, err := st.UserSetting(1, "dashboard.layout")
	if err != nil || !found || v != `{"a":2}` {
		t.Errorf("Wert: %q %v %v", v, found, err)
	}
	if _, found, _ := st.UserSetting(2, "dashboard.layout"); found {
		t.Error("Einstellungen dürfen nicht zwischen Personen durchsickern")
	}
}
