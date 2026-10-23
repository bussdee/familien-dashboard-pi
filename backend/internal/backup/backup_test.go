package backup

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"family-dashboard/backend/internal/store"
)

// testDienst baut einen Backup-Dienst mit echter Datenbank und einem
// Datenordner, in dem von allem etwas liegt.
func testDienst(t *testing.T) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.NewStore(filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	for pfad, inhalt := range map[string]string{
		"notes/einkauf.md":        "Milch",
		"ics/schule.ics":          "BEGIN:VCALENDAR",
		"ics-abo/1.ics":           "BEGIN:VCALENDAR abo",
		"photos/urlaub.jpg":       "jpeg",
		"files/vertrag.pdf":       "pdf",
		"music/lied.mp3":          "nicht sichern",
		"nicht-in-der-liste/x.md": "nicht sichern",
	} {
		p := filepath.Join(dir, pfad)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(inhalt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return NewService(db.DB(), filepath.Join(dir, "db.sqlite"), filepath.Join(dir, "backup"), dir, 7, "0 3 * * *"), dir
}

func TestSicherungIstEineHeileKopie(t *testing.T) {
	s, dir := testDienst(t)
	res, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}

	kopie, err := sql.Open("sqlite", filepath.Join(dir, "backup", res.Database))
	if err != nil {
		t.Fatal(err)
	}
	defer kopie.Close()

	var befund string
	if err := kopie.QueryRow(`PRAGMA integrity_check`).Scan(&befund); err != nil || befund != "ok" {
		t.Fatalf("Integritätsprüfung der Kopie: %q (%v)", befund, err)
	}
	// Und es sind die Daten der Familie drin, nicht eine leere Hülle.
	var n int
	if err := kopie.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n == 0 {
		t.Errorf("Kopie enthält keine Personen: %d (%v)", n, err)
	}
}

func TestArchivEnthaeltGenauDieDatenordner(t *testing.T) {
	s, dir := testDienst(t)
	res, err := s.Run()
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(filepath.Join(dir, "backup", res.Files))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	inhalt := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			b, _ := io.ReadAll(tr)
			inhalt[h.Name] = string(b)
		}
	}

	erwartet := map[string]string{
		"notes/einkauf.md":  "Milch",
		"ics/schule.ics":    "BEGIN:VCALENDAR",
		"ics-abo/1.ics":     "BEGIN:VCALENDAR abo", // die zuletzt gute Fassung eines Abos
		"photos/urlaub.jpg": "jpeg",
		"files/vertrag.pdf": "pdf",
	}
	for name, want := range erwartet {
		if got, ok := inhalt[name]; !ok || got != want {
			t.Errorf("%s: erwartet %q, war %q (vorhanden: %v)", name, want, got, ok)
		}
	}
	for name := range inhalt {
		if _, ok := erwartet[name]; !ok {
			t.Errorf("%s gehört nicht ins Archiv", name)
		}
	}
}

func TestAufbewahrung(t *testing.T) {
	s, dir := testDienst(t)
	bdir := filepath.Join(dir, "backup")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		t.Fatal(err)
	}
	alt := time.Now().AddDate(0, 0, -8)
	frisch := time.Now().AddDate(0, 0, -6)
	legeAn := func(name string, zeit time.Time) {
		p := filepath.Join(bdir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, zeit, zeit); err != nil {
			t.Fatal(err)
		}
	}
	legeAn("db_alt.sqlite", alt)
	legeAn("files_alt.tar.gz", alt)
	legeAn("db_frisch.sqlite", frisch)
	legeAn("files_frisch.tar.gz", frisch)
	legeAn("notiz.txt", alt) // gehört nicht zu den Sicherungen und bleibt

	if _, err := s.Run(); err != nil {
		t.Fatal(err)
	}
	for name, soll := range map[string]bool{
		"db_alt.sqlite": false, "files_alt.tar.gz": false,
		"db_frisch.sqlite": true, "files_frisch.tar.gz": true, "notiz.txt": true,
	} {
		_, err := os.Stat(filepath.Join(bdir, name))
		if (err == nil) != soll {
			t.Errorf("%s: vorhanden=%v, erwartet %v", name, err == nil, soll)
		}
	}
}

func TestStandardAufbewahrungSieben(t *testing.T) {
	if s := NewService(nil, "", "", "", 0, ""); s.retentionDays != 7 {
		t.Errorf("ohne Angabe erwartet 7 Tage, war %d", s.retentionDays)
	}
	if s := NewService(nil, "", "", "", -3, ""); s.retentionDays != 7 {
		t.Errorf("negativ erwartet 7 Tage, war %d", s.retentionDays)
	}
}

func TestHandler(t *testing.T) {
	s, _ := testDienst(t)

	// Auslösen
	rec := httptest.NewRecorder()
	s.TriggerBackup(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("Auslösen: %d %s", rec.Code, rec.Body)
	}
	var res Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || !strings.HasPrefix(res.Database, "db_") {
		t.Fatalf("Antwort: %v %s", err, rec.Body)
	}

	// Auflisten
	rec = httptest.NewRecorder()
	s.ListBackups(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var liste []struct{ Name string }
	_ = json.Unmarshal(rec.Body.Bytes(), &liste)
	if len(liste) != 2 {
		t.Errorf("erwartet 2 Dateien (Datenbank und Archiv), waren %d: %s", len(liste), rec.Body)
	}

	// Herunterladen: frischer Schnappschuss, keine halbe Datei
	rec = httptest.NewRecorder()
	s.Download(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "SQLite format 3") {
		t.Errorf("Download ist keine SQLite-Datei: %d %.20q", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition: %q", cd)
	}
}

func TestAuflistenOhneOrdner(t *testing.T) {
	s := NewService(nil, "", filepath.Join(t.TempDir(), "gibt-es-nicht"), "", 7, "")
	rec := httptest.NewRecorder()
	s.ListBackups(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("ohne Ordner erwartet leere Liste, war %d %s", rec.Code, rec.Body)
	}
}
