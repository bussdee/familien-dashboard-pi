package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/backup"
	"family-dashboard/backend/internal/calendar"
	"family-dashboard/backend/internal/chores"
	"family-dashboard/backend/internal/config"
	"family-dashboard/backend/internal/devices"
	"family-dashboard/backend/internal/files"
	"family-dashboard/backend/internal/goals"
	"family-dashboard/backend/internal/links"
	"family-dashboard/backend/internal/meals"
	"family-dashboard/backend/internal/music"
	"family-dashboard/backend/internal/notes"
	"family-dashboard/backend/internal/notify"
	"family-dashboard/backend/internal/photos"
	"family-dashboard/backend/internal/points"
	"family-dashboard/backend/internal/rewards"
	"family-dashboard/backend/internal/shopping"
	"family-dashboard/backend/internal/store"
	"family-dashboard/backend/internal/times"
	"family-dashboard/backend/internal/weather"
	"family-dashboard/backend/internal/wishes"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	configPath := flag.String("config", envOr("CONFIG_PATH", "/app/config.yaml"), "Path to config file")
	flag.Parse()

	setupLogging()
	_ = godotenv.Load()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Konfiguration konnte nicht geladen werden")
	}

	db, err := store.NewStore(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("Datenbank konnte nicht geöffnet werden")
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatal().Err(err).Msg("Migration fehlgeschlagen")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sql := db.DB()
	authSvc := auth.NewService(sql, db, cfg.Auth.JWTSecret, cfg.Auth.TokenTTL, cfg.Server.SecureCookies)
	weatherSvc := weather.NewService(sql, db, cfg.Weather.Latitude, cfg.Weather.Longitude, cfg.Weather.Timezone, cfg.Weather.CacheTTL)
	weatherSvc.SetSchwellen(weather.Schwellen{
		NassAbMillimeter:     cfg.Weather.DryWindow.WetFromMM,
		NassAbWahrscheinlich: cfg.Weather.DryWindow.WetFromProbability,
		MindestStunden:       cfg.Weather.DryWindow.MinHours,
		SonnigBisBewoelkung:  cfg.Weather.DryWindow.SunnyBelowCloud,
	})
	calendarSvc := calendar.NewService(cfg.Calendar.ICSDir, cfg.Calendar.LookaheadDays, cfg.Calendar.Watch, cfg.Weather.Timezone)
	calendarSvc.SetDB(sql)
	shoppingSvc := shopping.NewService(sql, cfg.Server.AllowedOrigins)
	notesSvc := notes.NewService(sql, cfg.Notes.Dir, cfg.Notes.Watch)
	choresSvc := chores.NewService(sql)
	pointsSvc := points.NewService(sql)
	devicesSvc := devices.NewService(sql, cfg.Devices.CheckInterval, cfg.Devices.Timeout)
	if err := devicesSvc.Seed(cfg.Devices.Targets); err != nil {
		log.Error().Err(err).Msg("Geräte aus config.yaml konnten nicht übernommen werden")
	}
	linksSvc := links.NewService(sql)
	photosSvc := photos.NewService(cfg.Photos.Dir)
	filesSvc := files.NewService(cfg.Files.Dir)
	musicSvc := music.NewService(sql, cfg.Music.Dir, db)
	timesSvc := times.NewService(sql)
	mealsSvc := meals.NewService(sql, shoppingSvc)
	rewardsSvc := rewards.NewService(sql)
	goalsSvc := goals.NewService(sql)
	wishesSvc := wishes.NewService(sql)
	backupSvc := backup.NewService(sql, cfg.Database.Path, cfg.Database.BackupDir,
		dataDir(cfg.Database.Path), cfg.Database.BackupRetention, cfg.Database.BackupCron)

	// Benachrichtigungen sind aus, solange keine ntfy-Adresse eingetragen ist.
	// Eine kaputte Adresse hält den Start nicht auf.
	notifySvc, err := notify.Neu(cfg.Notify.NtfyURL, cfg.Notify.NtfyToken)
	if err != nil {
		log.Error().Err(err).Msg("Benachrichtigungen bleiben aus")
		notifySvc, _ = notify.Neu("", "")
	}
	choresSvc.SetNotifier(notifySvc)
	wishesSvc.SetNotifier(notifySvc)
	rewardsSvc.SetNotifier(notifySvc)

	go notifySvc.Start(ctx)
	go weatherSvc.Start(ctx)
	go calendarSvc.Start(ctx)
	go shoppingSvc.Start(ctx)
	go notesSvc.Start(ctx)
	go choresSvc.Start(ctx)
	go pointsSvc.Start(ctx)
	go devicesSvc.Start(ctx)
	go backupSvc.Start(ctx)
	go musicSvc.Start(ctx)

	r := newRouter(cfg, dienste{
		authSvc:     authSvc,
		backupSvc:   backupSvc,
		calendarSvc: calendarSvc,
		choresSvc:   choresSvc,
		devicesSvc:  devicesSvc,
		filesSvc:    filesSvc,
		goalsSvc:    goalsSvc,
		linksSvc:    linksSvc,
		mealsSvc:    mealsSvc,
		musicSvc:    musicSvc,
		notesSvc:    notesSvc,
		notifySvc:   notifySvc,
		photosSvc:   photosSvc,
		pointsSvc:   pointsSvc,
		rewardsSvc:  rewardsSvc,
		shoppingSvc: shoppingSvc,
		timesSvc:    timesSvc,
		weatherSvc:  weatherSvc,
		wishesSvc:   wishesSvc,
	})

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}
	// The WebSocket route must outlive WriteTimeout, so it is exempted by
	// keeping WriteTimeout generous and relying on per-connection deadlines.

	go func() {
		log.Info().Str("addr", server.Addr).Msg("Server läuft")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("Server abgestürzt")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Fahre herunter...")
	cancel()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Shutdown erzwungen")
	}
	log.Info().Msg("Server beendet")
}

// langlaeufer sagt, welche Wege kein Zeitlimit vertragen.
//
// Der Schrägstrich am Ende ist wichtig: "/api/files/" trifft nur das
// Herunterladen einer einzelnen Datei, nicht die Liste unter "/api/files" —
// die soll ihr Zeitlimit behalten.
func langlaeufer(pfad string) bool {
	switch {
	case pfad == "/api/shopping/ws":
		return true
	case strings.HasPrefix(pfad, "/api/music/track/"):
		return true
	case strings.HasPrefix(pfad, "/api/files/"):
		return true
	}
	return false
}

// skipFor applies a middleware to every request except those the predicate
// picks out.
func skipFor(ausnahme func(string) bool, mw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := mw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ausnahme(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}

// requestLogger keeps health checks out of the log; they fire every 30s.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)

		ev := log.Info()
		if ww.Status() >= 500 {
			ev = log.Error()
		} else if ww.Status() >= 400 {
			ev = log.Warn()
		}
		ev.Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("took", time.Since(start)).
			Msg("request")
	})
}

func dataDir(dbPath string) string {
	if i := strings.LastIndex(dbPath, "/"); i > 0 {
		return dbPath[:i]
	}
	return "."
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func setupLogging() {
	zerolog.TimeFieldFormat = time.RFC3339
	level := zerolog.InfoLevel
	if lvl, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil && os.Getenv("LOG_LEVEL") != "" {
		level = lvl
	}
	zerolog.SetGlobalLevel(level)

	if os.Getenv("LOG_FORMAT") == "json" {
		log.Logger = log.With().Timestamp().Logger()
		return
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
		With().Timestamp().Logger()
}
