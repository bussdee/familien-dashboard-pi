package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

type Service struct {
	db            *sql.DB
	dbPath        string
	backupDir     string
	dataDir       string
	retentionDays int
	schedule      string

	mu sync.Mutex
	cr *cron.Cron
}

func NewService(db *sql.DB, dbPath, backupDir, dataDir string, retentionDays int, schedule string) *Service {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	return &Service{
		db:            db,
		dbPath:        dbPath,
		backupDir:     backupDir,
		dataDir:       dataDir,
		retentionDays: retentionDays,
		schedule:      schedule,
	}
}

// Start registers the nightly job. The previous setup documented a cron inside
// the container but never installed one, so backups never actually ran.
func (s *Service) Start(ctx context.Context) {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		log.Error().Err(err).Msg("Cannot create backup dir")
		return
	}

	c := cron.New()
	if _, err := c.AddFunc(s.schedule, func() {
		if _, err := s.Run(); err != nil {
			log.Error().Err(err).Msg("Scheduled backup failed")
		}
	}); err != nil {
		log.Error().Err(err).Str("cron", s.schedule).Msg("Invalid backup schedule, backups disabled")
		return
	}

	c.Start()
	s.cr = c
	log.Info().Str("cron", s.schedule).Int("retention_days", s.retentionDays).Msg("Backup scheduler started")

	<-ctx.Done()
	<-c.Stop().Done()
}

type Result struct {
	Database string    `json:"database"`
	Files    string    `json:"files"`
	Created  time.Time `json:"created"`
}

// Run takes a consistent snapshot. VACUUM INTO is SQLite's own hot-backup path,
// so no external sqlite3 binary is needed inside the container.
func (s *Service) Run() (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create backup dir: %w", err)
	}

	stamp := time.Now().Format("2006-01-02_15-04-05")
	dbTarget := filepath.Join(s.backupDir, "db_"+stamp+".sqlite")

	if _, err := s.db.Exec("VACUUM INTO ?", dbTarget); err != nil {
		return Result{}, fmt.Errorf("vacuum into: %w", err)
	}

	filesTarget := filepath.Join(s.backupDir, "files_"+stamp+".tar.gz")
	if err := s.archiveData(filesTarget); err != nil {
		return Result{}, fmt.Errorf("archive data: %w", err)
	}

	if err := s.prune(); err != nil {
		log.Error().Err(err).Msg("Backup pruning failed")
	}

	log.Info().Str("db", dbTarget).Str("files", filesTarget).Msg("Backup created")
	return Result{Database: filepath.Base(dbTarget), Files: filepath.Base(filesTarget), Created: time.Now()}, nil
}

func (s *Service) archiveData(target string) error {
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// Wird hier ein Ordner vergessen, fehlt er im Backup, ohne dass irgendwo
	// etwas rot wird. "files" ist beim Anlegen des Moduls genau deshalb sofort
	// mit eingetragen worden.
	for _, sub := range []string{"notes", "ics", "photos", "files"} {
		root := filepath.Join(s.dataDir, sub)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // an unreadable file must not abort the whole backup
			}
			rel, err := filepath.Rel(s.dataDir, path)
			if err != nil {
				return nil
			}

			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return nil
			}
			hdr.Name = filepath.ToSlash(rel)
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if info.IsDir() || !info.Mode().IsRegular() {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) prune() error {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "db_") && !strings.HasPrefix(name, "files_") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(s.backupDir, name)); err != nil {
			log.Error().Err(err).Str("file", name).Msg("Failed to prune backup")
		}
	}
	return nil
}

// ------------------------------------------------------------------ Handlers

func (s *Service) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	res, err := s.Run()
	if err != nil {
		log.Error().Err(err).Msg("Manual backup failed")
		auth.HTTPError(w, http.StatusInternalServerError, "Backup fehlgeschlagen: "+err.Error())
		return
	}
	auth.WriteJSON(w, res)
}

func (s *Service) ListBackups(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		auth.WriteJSON(w, []any{})
		return
	}

	type item struct {
		Name     string    `json:"name"`
		Size     int64     `json:"size"`
		Modified time.Time `json:"modified"`
	}
	items := []item{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, item{Name: e.Name(), Size: info.Size(), Modified: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Modified.After(items[j].Modified) })
	auth.WriteJSON(w, items)
}

// Download streams a fresh snapshot rather than the live database file, which
// would be torn mid-write under WAL.
func (s *Service) Download(w http.ResponseWriter, r *http.Request) {
	tmp, err := os.CreateTemp("", "family-dashboard-*.sqlite")
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	path := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(path) // VACUUM INTO requires the target not to exist
	defer os.Remove(path)

	if _, err := s.db.Exec("VACUUM INTO ?", path); err != nil {
		log.Error().Err(err).Msg("Snapshot for download failed")
		auth.HTTPError(w, http.StatusInternalServerError, "Snapshot fehlgeschlagen")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer f.Close()

	name := fmt.Sprintf("family-dashboard_%s.sqlite", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if _, err := io.Copy(w, f); err != nil {
		log.Error().Err(err).Msg("Backup download interrupted")
	}
}
