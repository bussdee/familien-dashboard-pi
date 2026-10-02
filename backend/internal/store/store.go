package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexedwards/argon2id"
	"github.com/rs/zerolog/log"

	_ "modernc.org/sqlite"
)

// DefaultPIN is assigned to the seeded family members on a fresh database.
// The admin area forces a change on first login (see users.pin_is_default).
const DefaultPIN = "1234"

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// SQLite tolerates exactly one writer; serialising here avoids SQLITE_BUSY
	// under the WebSocket fan-out.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			color TEXT NOT NULL,
			pin_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			avatar_emoji TEXT DEFAULT '👤',
			pin_is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS shopping_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			quantity TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT '',
			checked BOOLEAN NOT NULL DEFAULT 0,
			user_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			pinned BOOLEAN NOT NULL DEFAULT 0,
			owner_id INTEGER,
			source_file TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chores (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			interval_days INTEGER NOT NULL DEFAULT 7,
			points INTEGER NOT NULL DEFAULT 10,
			rotate BOOLEAN NOT NULL DEFAULT 1,
			-- rotate | person | everyone | nobody
			assignment TEXT NOT NULL DEFAULT 'rotate',
			assignee_id INTEGER,
			last_done_at DATETIME,
			next_due_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chore_completions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chore_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			completed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			verified BOOLEAN NOT NULL DEFAULT 0,
			points_awarded INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (chore_id) REFERENCES chores(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// One row per device, updated in place. The previous schema appended a
		// row per health check, which grew by ~11k rows/day.
		// Every scoring event lands here, whatever earned it. Keeping one table
		// means the leaderboard does not have to union a growing list of
		// sources as new ways to earn points appear.
		`CREATE TABLE IF NOT EXISTS point_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			source TEXT NOT NULL,
			reference_id INTEGER,
			points INTEGER NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// Der Monatsabschluss. Die laufende Rangliste rechnet aus
		// point_events — und die verschiebt sich, sobald eine einmalige
		// Aufgabe gelöscht wird. Wer im September gewonnen hat, soll aber im
		// Dezember noch dastehen. Also wird das Ergebnis zum Monatswechsel
		// einmal festgeschrieben und danach nicht mehr angefasst.
		//
		// Name, Farbe und Emoji stehen mit drin: Sie gehören zur Platzierung
		// von damals und dürfen sich nicht mehr ändern, wenn jemand später
		// sein Profil umbenennt oder die Familie verlässt.
		`CREATE TABLE IF NOT EXISTS month_scores (
			-- JJJJ-MM des abgeschlossenen Monats.
			month TEXT NOT NULL,
			-- Bewusst ohne Fremdschlüssel: Wer die Familie verlässt, soll
			-- aus der Rangliste vom März nicht verschwinden.
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '',
			avatar_emoji TEXT NOT NULL DEFAULT '',
			points INTEGER NOT NULL DEFAULT 0,
			activities INTEGER NOT NULL DEFAULT 0,
			rank INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (month, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS device_status (
			name TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			last_check DATETIME DEFAULT CURRENT_TIMESTAMP,
			error TEXT NOT NULL DEFAULT ''
		)`,
		// Devices used to live in config.yaml, which nobody can edit from a
		// phone. The file now only seeds this table on first run.
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'http',
			url TEXT NOT NULL DEFAULT '',
			link TEXT NOT NULL DEFAULT '',
			host TEXT NOT NULL DEFAULT '',
			port INTEGER NOT NULL DEFAULT 0,
			expect_status INTEGER NOT NULL DEFAULT 0,
			icon TEXT NOT NULL DEFAULT '',
			position INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Bookmarks. Everyone keeps their own; a shared one shows up for all.
		`CREATE TABLE IF NOT EXISTS links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_id INTEGER,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT '',
			emoji TEXT NOT NULL DEFAULT '🔗',
			pinned BOOLEAN NOT NULL DEFAULT 0,
			shared BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// Per-person preferences, e.g. which widgets are shown and in what order.
		`CREATE TABLE IF NOT EXISTS user_settings (
			user_id INTEGER NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, key),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS weather_cache (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			payload TEXT NOT NULL,
			fetched_at DATETIME NOT NULL
		)`,
		// Family-wide settings that belong in the UI rather than in config.yaml
		// (the weather location, for one).
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Appointments entered in the dashboard. These live alongside the
		// read-only events parsed from .ics files.
		`CREATE TABLE IF NOT EXISTS calendar_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			start_at DATETIME NOT NULL,
			end_at DATETIME NOT NULL,
			all_day BOOLEAN NOT NULL DEFAULT 0,
			repeat TEXT NOT NULL DEFAULT 'none',
			color TEXT NOT NULL DEFAULT '#0d9488',
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
		)`,
		// Der Musik-Index. Er gehört in die Datenbank und nicht in den
		// Arbeitsspeicher: 20 000 Dateien von einer USB-Platte zu lesen dauert
		// beim ersten Mal Minuten, und das darf nach jedem Neustart nicht
		// wieder von vorn losgehen.
		//
		// path ist der Pfad relativ zum Musikordner und eindeutig — daran
		// erkennt der Durchlauf wieder, was er schon kennt. size und mod_time
		// entscheiden, ob eine Datei überhaupt neu gelesen werden muss.
		`CREATE TABLE IF NOT EXISTS music_tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			folder TEXT NOT NULL DEFAULT '',
			filename TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			artist TEXT NOT NULL DEFAULT '',
			album TEXT NOT NULL DEFAULT '',
			track_no INTEGER NOT NULL DEFAULT 0,
			duration_sec INTEGER NOT NULL DEFAULT 0,
			size INTEGER NOT NULL DEFAULT 0,
			mod_time DATETIME NOT NULL,
			indexed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Zeiten, zu denen jemand nicht da ist. Zwei Tabellen, weil zwei
		// verschiedene Leben abgebildet werden müssen:
		//
		// Ein Kind hat einen Stundenplan, der sich ein- bis zweimal im Jahr
		// ändert. Das ist ein Wochenmuster: montags 8 bis 13, jede Woche.
		//
		// Eltern arbeiten jede Woche anders und tragen einmal im Monat die
		// nächsten vier Wochen ein. Das sind konkrete Tage mit Datum.
		//
		// Ein Wochenmuster in konkrete Tage aufzulösen hiesse, für das Kind
		// jedes Jahr 250 Zeilen zu schreiben. Konkrete Tage als Wochenmuster
		// zu zwingen ginge gar nicht. Also beides.
		`CREATE TABLE IF NOT EXISTS weekly_times (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			-- 0 = Montag ... 6 = Sonntag. Nicht die Zählung von SQLite oder
			-- JavaScript, sondern die, die man im Kalender liest.
			weekday INTEGER NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			-- arbeit | schule | sonstiges
			kind TEXT NOT NULL DEFAULT 'schule',
			note TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// Konkrete Tage. Sie gehen dem Wochenmuster vor: Wer für einen Tag
		// hier etwas stehen hat, für den gilt an diesem Tag nur das.
		`CREATE TABLE IF NOT EXISTS day_times (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			-- JJJJ-MM-TT. Als Text, damit ein Tag ein Tag bleibt und nicht in
			-- einer Zeitzone verrutscht.
			day TEXT NOT NULL,
			start_time TEXT NOT NULL DEFAULT '',
			end_time TEXT NOT NULL DEFAULT '',
			-- arbeit | schule | frei | urlaub | krank | sonstiges
			kind TEXT NOT NULL DEFAULT 'arbeit',
			note TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// Der Essensplan: ein Gericht je Tag. Mehr braucht die Frage „Was
		// gibt's heute?" nicht — und mehr pflegt auch niemand. Der Tag ist
		// der Schlüssel, als Text, aus demselben Grund wie bei day_times.
		//
		// ingredients sind Zeilen, keine eigene Tabelle: Sie wandern mit
		// einem Tipp auf die Einkaufsliste und werden dort zu Einträgen.
		`CREATE TABLE IF NOT EXISTS meals (
			day TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			ingredients TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Belohnungen: wofür die Punkte da sind. Die Punkte selbst bleiben
		// unangetastet — Level und Rangliste sollen nicht fallen, nur weil
		// jemand sich ein Eis geholt hat. Ausgegeben wird ein Guthaben, das
		// sich aus Punkten minus Einlösungen ergibt.
		`CREATE TABLE IF NOT EXISTS rewards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			emoji TEXT NOT NULL DEFAULT '🎁',
			cost INTEGER NOT NULL,
			active BOOLEAN NOT NULL DEFAULT 1,
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Eine Einlösung merkt sich Titel und Preis von damals. Wird die
		// Belohnung später teurer oder gelöscht, stimmt das Guthaben weiter.
		`CREATE TABLE IF NOT EXISTS reward_redemptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			reward_id INTEGER,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			emoji TEXT NOT NULL DEFAULT '🎁',
			cost INTEGER NOT NULL,
			-- offen | eingeloest | abgelehnt
			status TEXT NOT NULL DEFAULT 'offen',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			decided_at DATETIME,
			FOREIGN KEY (reward_id) REFERENCES rewards(id) ON DELETE SET NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// Was die Familie schon einmal eingekauft hat. Daraus werden die
		// Vorschläge unter dem Eingabefeld: Milch tippt man nicht jede Woche
		// neu. name_key ist der kleingeschriebene Name, damit „Milch" und
		// „milch" ein Eintrag bleiben.
		`CREATE TABLE IF NOT EXISTS shopping_history (
			name_key TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT '',
			uses INTEGER NOT NULL DEFAULT 1,
			last_used DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// Das Familienziel: ein gemeinsames Ziel statt eines Wettrennens.
		// Gezählt wird alles, was die Familie seit started_at zusammen
		// verdient hat. Ein Ziel ist aktiv, bis ein Elternteil es abschliesst;
		// reached_at merkt sich, wann es geschafft war.
		`CREATE TABLE IF NOT EXISTS family_goals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			emoji TEXT NOT NULL DEFAULT '🎯',
			target INTEGER NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reached_at DATETIME,
			closed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS login_attempts (
			user_id INTEGER PRIMARY KEY,
			failures INTEGER NOT NULL DEFAULT 0,
			locked_until DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_shopping_checked ON shopping_items(checked, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_owner ON notes(owner_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notes_source_file ON notes(source_file) WHERE source_file IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_chores_assignee ON chores(assignee_id)`,
		`CREATE INDEX IF NOT EXISTS idx_completions_user ON chore_completions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_start ON calendar_events(start_at)`,
		`CREATE INDEX IF NOT EXISTS idx_points_user ON point_events(user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_links_owner ON links(owner_id, position)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_position ON devices(position)`,
		// Geblättert wird nach Ordner, sortiert nach Dateiname — ein Hörspiel
		// läuft von Teil 1 bis Teil 12.
		`CREATE INDEX IF NOT EXISTS idx_music_folder ON music_tracks(folder, filename)`,
		`CREATE INDEX IF NOT EXISTS idx_weekly_times_user ON weekly_times(user_id, weekday)`,
		`CREATE INDEX IF NOT EXISTS idx_day_times_day ON day_times(day, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_month_scores ON month_scores(month, rank)`,
		`CREATE INDEX IF NOT EXISTS idx_redemptions_user ON reward_redemptions(user_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_rewards_position ON rewards(position)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed (%.60s...): %w", m, err)
		}
	}

	// Nachträglich ergänzte Spalten. CREATE TABLE IF NOT EXISTS greift bei
	// bestehenden Datenbanken nicht, ALTER TABLE bricht ab, wenn die Spalte
	// schon da ist — deshalb erst nachsehen.
	if err := s.addColumn("chores", "assignment", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	// Wer arbeitet, steht durch die Reihum-Verteilung trotzdem überall im
	// Plan. Der Schalter nimmt eine Person aus der Rotation, ohne ihr sonst
	// etwas wegzunehmen. Voreinstellung 1: bestehende Familien merken nichts.
	if err := s.addColumn("users", "in_rotation", "BOOLEAN NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	// Einmalige Aufgaben: der Zahnarzttermin unter den Hausarbeiten. Sie
	// kommen nicht wieder, wenn sie erledigt sind — ein Intervall wäre für
	// sie eine Lüge. Voreinstellung 0, damit alles Bestehende bleibt, wie es
	// ist.
	if err := s.addColumn("chores", "one_off", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// Eltern bestätigen: Eine so markierte Aufgabe bringt einem Kind erst
	// Punkte, wenn ein Elternteil nachgesehen hat. Voreinstellung 0 — alles
	// Bestehende verhält sich wie bisher.
	if err := s.addColumn("chores", "needs_check", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// Eine Erledigung, die auf diese Bestätigung wartet. Eigene Spalte statt
	// "verified": Die stand bei allen bisherigen Erledigungen auf 0 und
	// unterscheidet deshalb nichts.
	if err := s.addColumn("chore_completions", "pending", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	// Termine für eine einzelne Person. Ohne die Spalte gehört jeder Termin
	// der ganzen Familie — und „Mama: Zahnarzt 14 Uhr" liess sich gar nicht
	// eintragen.
	if err := s.addColumn("calendar_events", "user_id", "INTEGER"); err != nil {
		return err
	}

	// Bestehende Aufgaben in die neue Schreibweise überführen. Läuft genau
	// einmal, weil danach kein leerer Wert mehr übrig ist.
	if _, err := s.db.Exec(`
		UPDATE chores SET assignment = CASE
			WHEN rotate = 1            THEN 'rotate'
			WHEN assignee_id IS NOT NULL THEN 'person'
			ELSE 'nobody'
		END WHERE assignment = ''`); err != nil {
		return fmt.Errorf("chores.assignment backfill: %w", err)
	}

	if err := s.seedUsers(); err != nil {
		return err
	}
	if err := s.seedChores(); err != nil {
		return err
	}
	if err := s.seedRewards(); err != nil {
		return err
	}
	return s.backfillPoints()
}

func (s *Store) seedUsers() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := argon2id.CreateHash(DefaultPIN, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("hash default pin: %w", err)
	}

	defaults := []struct {
		name, color, role, emoji string
	}{
		{"Papa", "#3b82f6", "admin", "👨"},
		{"Mama", "#ec4899", "admin", "👩"},
		{"Kind", "#f59e0b", "member", "🧒"},
	}

	for _, u := range defaults {
		if _, err := s.db.Exec(
			`INSERT INTO users (name, color, pin_hash, role, avatar_emoji, pin_is_default)
			 VALUES (?, ?, ?, ?, ?, 1)`,
			u.name, u.color, hash, u.role, u.emoji,
		); err != nil {
			return err
		}
	}

	log.Warn().Str("pin", DefaultPIN).Msg("Seeded default users — change the PINs in the admin area")
	return nil
}

func (s *Store) seedChores() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM chores").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaults := []struct {
		title, description string
		interval, points   int
	}{
		{"Müll rausbringen", "Restmüll und Altpapier", 7, 10},
		{"Geschirrspüler ausräumen", "", 1, 5},
		{"Staubsaugen", "Wohnzimmer und Flur", 7, 15},
		{"Bad putzen", "", 14, 20},
	}

	for _, c := range defaults {
		if _, err := s.db.Exec(
			`INSERT INTO chores (title, description, interval_days, points, next_due_at)
			 VALUES (?, ?, ?, ?, datetime('now', '+' || ? || ' days'))`,
			c.title, c.description, c.interval, c.points, c.interval,
		); err != nil {
			return err
		}
	}
	return nil
}

// seedRewards legt einmalig ein paar Beispiel-Belohnungen an — auch bei
// bestehenden Installationen, die mit 2.0 zum ersten Mal Belohnungen
// kennen. Eine leere Seite erklärt die Idee schlechter als drei Beispiele.
//
// Ein Merker in settings sorgt dafür, dass das genau einmal passiert: Wer die
// Beispiele löscht, bekommt sie beim nächsten Start nicht wieder.
func (s *Store) seedRewards() error {
	if _, done, err := s.Setting("seed.rewards"); err != nil || done {
		return err
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM rewards").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		defaults := []struct {
			title, emoji string
			cost         int
		}{
			{"30 Minuten Bildschirmzeit", "📱", 50},
			{"Ein Eis", "🍦", 80},
			{"Film aussuchen am Familienabend", "🎬", 120},
			{"Später ins Bett (30 Min.)", "🌙", 150},
		}
		for i, r := range defaults {
			if _, err := s.db.Exec(
				`INSERT INTO rewards (title, emoji, cost, position) VALUES (?, ?, ?, ?)`,
				r.title, r.emoji, r.cost, i,
			); err != nil {
				return err
			}
		}
	}
	return s.SetSetting("seed.rewards", "1")
}

// Setting reads a family-wide setting. A missing key is not an error; the
// caller decides what the fallback is.
func (s *Store) Setting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value)
	return err
}

// backfillPoints moves scores from databases created before point_events
// existed. Chore completions were the only source back then.
func (s *Store) backfillPoints() error {
	var existing int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM point_events").Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	res, err := s.db.Exec(`
		INSERT INTO point_events (user_id, source, reference_id, points, note, created_at)
		SELECT user_id, 'chore', chore_id, points_awarded, '', completed_at
		FROM chore_completions`)
	if err != nil {
		return fmt.Errorf("backfill points: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Info().Int64("events", n).Msg("Migrated existing chore points")
	}
	return nil
}

// UserSetting reads one per-person preference.
func (s *Store) UserSetting(userID int, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(
		"SELECT value FROM user_settings WHERE user_id = ? AND key = ?", userID, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) SetUserSetting(userID int, key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_settings (user_id, key, value) VALUES (?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		userID, key, value)
	return err
}

// addColumn ergänzt eine Spalte, falls sie noch fehlt. SQLite kennt kein
// "ADD COLUMN IF NOT EXISTS", also wird vorher in der Tabellenbeschreibung
// nachgesehen.
func (s *Store) addColumn(table, column, ddl string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return fmt.Errorf("read columns of %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + ddl); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}
