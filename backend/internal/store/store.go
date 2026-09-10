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
