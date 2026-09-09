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
	"family-dashboard/backend/internal/links"
	"family-dashboard/backend/internal/notes"
	"family-dashboard/backend/internal/photos"
	"family-dashboard/backend/internal/points"
	"family-dashboard/backend/internal/shopping"
	"family-dashboard/backend/internal/store"
	"family-dashboard/backend/internal/weather"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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
	backupSvc := backup.NewService(sql, cfg.Database.Path, cfg.Database.BackupDir,
		dataDir(cfg.Database.Path), cfg.Database.BackupRetention, cfg.Database.BackupCron)

	go weatherSvc.Start(ctx)
	go calendarSvc.Start(ctx)
	go shoppingSvc.Start(ctx)
	go notesSvc.Start(ctx)
	go choresSvc.Start(ctx)
	go devicesSvc.Start(ctx)
	go backupSvc.Start(ctx)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger)
	// The WebSocket handshake must not inherit a 60s request deadline.
	r.Use(skipFor("/api/shopping/ws", middleware.Timeout(60*time.Second)))

	if len(cfg.Server.AllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.Server.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OK"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			auth.WriteJSON(w, map[string]string{"status": "ok"})
		})

		// Public: the login screen needs the roster before anyone is signed in.
		r.Get("/auth/users", authSvc.PublicUsers)
		r.Post("/auth/login", authSvc.Login)
		r.Post("/auth/logout", authSvc.Logout)

		r.Group(func(r chi.Router) {
			r.Use(authSvc.AuthMiddleware)

			r.Get("/auth/me", authSvc.Me)
			// Wandgerät ein- und ausschalten. Einschalten darf nur ein
			// Administrator, ausschalten das Gerät selbst.
			r.Delete("/auth/device", authSvc.DisableDevice)

			// Lesen darf auch das Wandgerät: es sieht nur die geteilten Links.
			r.Get("/links", linksSvc.List)

			// Alles, was einer Person gehört: am Wandgerät gesperrt, dort ist
			// niemand persönlich angemeldet.
			r.Group(func(r chi.Router) {
				r.Use(authSvc.PersonMiddleware)
				r.Put("/auth/profile", authSvc.UpdateOwnProfile)
				r.Post("/auth/pin", authSvc.ChangeOwnPIN)
				r.Get("/preferences/{key}", authSvc.GetPreference)
				r.Put("/preferences/{key}", authSvc.SetPreference)

				r.Post("/links", linksSvc.Create)
				r.Put("/links/{id}", linksSvc.Update)
				r.Post("/links/{id}/pin", linksSvc.TogglePin)
				r.Delete("/links/{id}", linksSvc.Delete)
				r.Post("/links/reorder", linksSvc.Reorder)
			})

			r.Get("/weather", weatherSvc.GetWeather)
			r.Get("/weather/location", weatherSvc.GetLocation)
			r.Get("/weather/search", weatherSvc.SearchLocations)

			r.Get("/calendar", calendarSvc.GetEvents)
			r.Get("/calendar/events", calendarSvc.ListStored)
			r.Post("/calendar/events", calendarSvc.CreateEvent)
			r.Put("/calendar/events/{id}", calendarSvc.UpdateEvent)
			r.Delete("/calendar/events/{id}", calendarSvc.DeleteEvent)

			r.Get("/shopping", shoppingSvc.List)
			r.Post("/shopping", shoppingSvc.Create)
			r.Put("/shopping/{id}", shoppingSvc.Update)
			r.Delete("/shopping/{id}", shoppingSvc.Delete)
			r.Post("/shopping/clear-checked", shoppingSvc.ClearChecked)
			r.Get("/shopping/reward", shoppingSvc.Reward)
			r.Get("/shopping/ws", shoppingSvc.WebSocket)

			r.Get("/notes", notesSvc.List)
			r.Post("/notes", notesSvc.Create)
			r.Put("/notes/{id}", notesSvc.Update)
			r.Delete("/notes/{id}", notesSvc.Delete)

			r.Get("/chores", choresSvc.List)
			r.Post("/chores", choresSvc.Create)
			r.Put("/chores/{id}", choresSvc.Update)
			r.Delete("/chores/{id}", choresSvc.Delete)
			r.Post("/chores/{id}/complete", choresSvc.Complete)

			// The scoreboard spans chores and shopping, so it lives outside
			// either one. /chores/stats stays as the historical path.
			r.Get("/scoreboard", pointsSvc.Scoreboard)
			r.Get("/scoreboard/history", pointsSvc.History)
			r.Get("/chores/stats", pointsSvc.Scoreboard)

			r.Get("/devices", devicesSvc.GetStatus)

			r.Get("/photos", photosSvc.List)
			r.Post("/photos", photosSvc.Upload)
			r.Get("/photos/{name}", photosSvc.Serve)
			r.Delete("/photos/{name}", photosSvc.Delete)

			r.Group(func(r chi.Router) {
				r.Use(authSvc.AdminMiddleware)
				r.Post("/auth/device", authSvc.EnableDevice)
				r.Get("/admin/users", authSvc.ListUsers)
				r.Post("/admin/users", authSvc.CreateUser)
				r.Put("/admin/users/{id}", authSvc.UpdateUser)
				r.Delete("/admin/users/{id}", authSvc.DeleteUser)
				r.Put("/admin/weather/location", weatherSvc.SetLocation)
				r.Get("/admin/points", pointsSvc.AdminHistory)
				r.Post("/admin/points", pointsSvc.Adjust)
				r.Delete("/admin/points/{id}", pointsSvc.Revoke)
				r.Post("/admin/points/reset", pointsSvc.ResetUser)
				r.Get("/admin/devices", devicesSvc.List)
				r.Post("/admin/devices", devicesSvc.Create)
				r.Put("/admin/devices/{id}", devicesSvc.Update)
				r.Delete("/admin/devices/{id}", devicesSvc.Delete)
				r.Post("/admin/devices/reorder", devicesSvc.Reorder)
				r.Post("/admin/devices/test", devicesSvc.Test)
				r.Get("/admin/backups", backupSvc.ListBackups)
				r.Post("/admin/backup", backupSvc.TriggerBackup)
				r.Get("/admin/backup/download", backupSvc.Download)
			})
		})
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

// skipFor applies a middleware to every request except one exact path.
func skipFor(path string, mw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := mw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == path {
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
