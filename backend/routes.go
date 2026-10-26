package main

import (
	"net/http"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/backup"
	"family-dashboard/backend/internal/calendar"
	"family-dashboard/backend/internal/chores"
	"family-dashboard/backend/internal/config"
	"family-dashboard/backend/internal/devices"
	"family-dashboard/backend/internal/einstellungen"
	"family-dashboard/backend/internal/files"
	"family-dashboard/backend/internal/goals"
	"family-dashboard/backend/internal/links"
	"family-dashboard/backend/internal/meals"
	"family-dashboard/backend/internal/music"
	"family-dashboard/backend/internal/neuigkeiten"
	"family-dashboard/backend/internal/notes"
	"family-dashboard/backend/internal/notify"
	"family-dashboard/backend/internal/photos"
	"family-dashboard/backend/internal/points"
	"family-dashboard/backend/internal/rewards"
	"family-dashboard/backend/internal/rezepte"
	"family-dashboard/backend/internal/shopping"
	"family-dashboard/backend/internal/times"
	"family-dashboard/backend/internal/weather"
	"family-dashboard/backend/internal/wishes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// dienste sind die Teile, an die die Wege geknüpft werden. Als eigene
// Struktur, damit die Rechte aller Wege in einem Test geprüft werden können
// (main_test.go), ohne den Server zu starten.
type dienste struct {
	authSvc     *auth.Service
	backupSvc   *backup.Service
	calendarSvc *calendar.Service
	choresSvc   *chores.Service
	devicesSvc  *devices.Service
	einstSvc    *einstellungen.Service
	filesSvc    *files.Service
	goalsSvc    *goals.Service
	linksSvc    *links.Service
	mealsSvc    *meals.Service
	musicSvc    *music.Service
	newsSvc     *neuigkeiten.Service
	notesSvc    *notes.Service
	notifySvc   *notify.Service
	photosSvc   *photos.Service
	pointsSvc   *points.Service
	rewardsSvc  *rewards.Service
	rezepteSvc  *rezepte.Service
	shoppingSvc *shopping.Service
	timesSvc    *times.Service
	weatherSvc  *weather.Service
	wishesSvc   *wishes.Service
}

// newRouter knüpft alle Wege an die Dienste. Wer was darf, steht hier in den
// Gruppen: öffentlich, angemeldet (auch das Wandgerät), Person, Eltern, Admin.
func newRouter(cfg *config.Config, d dienste) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// Die Adresse des Geräts, nicht die, die der Absender behauptet — die
	// Sperre nach Fehlversuchen hängt daran (siehe auth/quelle.go).
	r.Use(auth.RealClientIP(cfg.Server.ProxyHops))
	r.Use(middleware.Recoverer)
	r.Use(requestLogger)
	// Ein Zeitlimit von 60 Sekunden ist für Anfragen richtig und für alles
	// falsch, was einen Datenstrom offen hält: den WebSocket, ein Hörspiel von
	// 80 Minuten, eine Datei von 100 MB über schwaches WLAN.
	r.Use(skipFor(langlaeufer, middleware.Timeout(60*time.Second)))

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
		r.Get("/auth/users", d.authSvc.PublicUsers)
		r.Post("/auth/login", d.authSvc.Login)
		r.Post("/auth/logout", d.authSvc.Logout)

		r.Group(func(r chi.Router) {
			r.Use(d.authSvc.AuthMiddleware)

			r.Get("/auth/me", d.authSvc.Me)
			// Wandgerät ein- und ausschalten. Einschalten darf nur ein
			// Administrator, ausschalten das Gerät selbst.
			r.Delete("/auth/device", d.authSvc.DisableDevice)

			// Lesen darf auch das Wandgerät: es sieht nur die geteilten Links.
			r.Get("/links", d.linksSvc.List)

			// Alles, was einer Person gehört: am Wandgerät gesperrt, dort ist
			// niemand persönlich angemeldet.
			r.Group(func(r chi.Router) {
				r.Use(d.authSvc.PersonMiddleware)
				r.Put("/auth/profile", d.authSvc.UpdateOwnProfile)
				r.Post("/auth/pin", d.authSvc.ChangeOwnPIN)
				r.Get("/preferences/{key}", d.authSvc.GetPreference)
				r.Put("/preferences/{key}", d.authSvc.SetPreference)

				// Einlösen gehört einer Person: Am Wandgerät könnte sonst
				// jeder das Guthaben der Schwester ausgeben.
				r.Post("/rewards/{id}/redeem", d.rewardsSvc.Redeem)
				r.Delete("/rewards/redemptions/{id}", d.rewardsSvc.Cancel)

				// Die Wunschliste: ein Kind trägt einen Wunsch ein, die
				// Eltern legen den Preis fest, das Kind löst ihn ein.
				// Auch das gehört einer Person.
				r.Post("/wishes", d.wishesSvc.Create)
				r.Delete("/wishes/{id}", d.wishesSvc.Withdraw)
				r.Post("/wishes/{id}/redeem", d.wishesSvc.Redeem)

				// Persönliche Feeds gehören einer Person.
				r.Post("/rss/feeds", d.newsSvc.CreatePersonal)
				r.Delete("/rss/feeds/{id}", d.newsSvc.DeletePersonal)
			})

			// Die Schalter der Familie: Lesen darf jeder, auch das Wandgerät —
			// Menü und Bildschirmschoner richten sich danach.
			r.Get("/settings", d.einstSvc.Get)

			// Neuigkeiten und Rezeptbuch: Lesen darf jeder.
			r.Get("/rss/feeds", d.newsSvc.ListFeeds)
			r.Get("/rss/items", d.newsSvc.Items)
			r.Get("/recipes", d.rezepteSvc.List)
			r.Get("/recipes/{id}", d.rezepteSvc.Get)
			r.Get("/recipes/{id}/image", d.rezepteSvc.Image)

			r.Get("/weather", d.weatherSvc.GetWeather)
			r.Get("/weather/location", d.weatherSvc.GetLocation)
			r.Get("/weather/search", d.weatherSvc.SearchLocations)

			r.Get("/calendar", d.calendarSvc.GetEvents)
			r.Get("/calendar/events", d.calendarSvc.ListStored)

			// Abhaken darf jeder, auch das Wandgerät — das ist „reagieren".
			// Was auf der Liste steht, bestimmen die Eltern (siehe unten).
			r.Get("/shopping", d.shoppingSvc.List)
			r.Put("/shopping/{id}/checked", d.shoppingSvc.SetChecked)
			r.Post("/shopping/clear-checked", d.shoppingSvc.ClearChecked)
			r.Get("/shopping/reward", d.shoppingSvc.Reward)
			r.Get("/shopping/ws", d.shoppingSvc.WebSocket)
			r.Get("/shopping/suggestions", d.shoppingSvc.Suggestions)

			r.Get("/meals", d.mealsSvc.List)
			r.Get("/meals/recent", d.mealsSvc.Recent)

			// Das Angebot sehen darf jeder, auch das Wandgerät: Es spornt an.
			r.Get("/rewards", d.rewardsSvc.List)

			r.Get("/notes", d.notesSvc.List)

			r.Get("/chores", d.choresSvc.List)
			// Eine Aufgabe erledigen darf jeder: Die Punkte gehen an den, der
			// sie gemacht hat. Anlegen, ändern und löschen ist Elternsache.
			r.Post("/chores/{id}/complete", d.choresSvc.Complete)
			// Was auf ein Elternteil wartet. Lesen darf jeder, auch das
			// Wandgerät; bestätigen nur Eltern (siehe unten).
			r.Get("/chores/pending", d.choresSvc.ListPending)

			// Das Familienziel sieht jeder — es gehört allen.
			r.Get("/goal", d.goalsSvc.Get)

			// Die Wunschliste sieht jeder, auch das Wandgerät: Eltern sehen
			// alle Wünsche, ein Kind seine eigenen und das Wandgerät die der
			// ganzen Familie (es spornt an, wie das Angebot der Belohnungen).
			r.Get("/wishes", d.wishesSvc.List)

			// Eltern-Wege: für Eltern immer, am Wandgerät mit der PIN eines
			// Elternteils. So lassen sich dort Punkte vergeben und Aufgaben
			// bestätigen, ohne das Tablet als Papa angemeldet zurückzulassen.
			r.Group(func(r chi.Router) {
				r.Use(d.authSvc.ElternMiddleware)
				r.Post("/auth/eltern", d.authSvc.ElternPruefen)
				r.Post("/admin/points", d.pointsSvc.Adjust)
				r.Delete("/admin/points/{id}", d.pointsSvc.Revoke)
				r.Post("/chores/pending/{id}/approve", d.choresSvc.Approve)
				r.Post("/chores/pending/{id}/reject", d.choresSvc.Reject)

				// Neues eintragen, ändern und löschen: nur Eltern. Kinder
				// lesen und reagieren (abhaken, erledigen, einlösen, wünschen).
				// Am Wandgerät geht es mit der PIN eines Elternteils.
				r.Post("/calendar/events", d.calendarSvc.CreateEvent)
				r.Put("/calendar/events/{id}", d.calendarSvc.UpdateEvent)
				r.Delete("/calendar/events/{id}", d.calendarSvc.DeleteEvent)
				r.Post("/calendar/events/{id}/skip", d.calendarSvc.SkipOccurrence)
				r.Delete("/calendar/events/{id}/skip/{date}", d.calendarSvc.RestoreOccurrence)

				r.Post("/shopping", d.shoppingSvc.Create)
				r.Put("/shopping/{id}", d.shoppingSvc.Update)
				r.Delete("/shopping/{id}", d.shoppingSvc.Delete)
				r.Delete("/shopping/suggestions", d.shoppingSvc.ForgetSuggestion)

				r.Put("/meals/{day}", d.mealsSvc.Save)
				r.Delete("/meals/{day}", d.mealsSvc.Delete)
				r.Post("/meals/{day}/shopping", d.mealsSvc.ToShopping)

				r.Post("/notes", d.notesSvc.Create)
				r.Put("/notes/{id}", d.notesSvc.Update)
				r.Delete("/notes/{id}", d.notesSvc.Delete)

				r.Post("/chores", d.choresSvc.Create)
				r.Put("/chores/{id}", d.choresSvc.Update)
				// Steht vor /chores/{id}, damit „done" nicht als ID gelesen wird.
				r.Delete("/chores/done", d.choresSvc.DeleteDone)
				r.Delete("/chores/{id}", d.choresSvc.Delete)

				r.Post("/photos", d.photosSvc.Upload)
				r.Delete("/photos/{name}", d.photosSvc.Delete)

				r.Post("/times/weekly", d.timesSvc.CreateWeekly)
				r.Delete("/times/weekly/{id}", d.timesSvc.DeleteWeekly)
				r.Put("/times/days", d.timesSvc.SaveDays)

				r.Post("/links", d.linksSvc.Create)
				r.Put("/links/{id}", d.linksSvc.Update)
				r.Post("/links/{id}/pin", d.linksSvc.TogglePin)
				r.Delete("/links/{id}", d.linksSvc.Delete)
				r.Post("/links/reorder", d.linksSvc.Reorder)

				// Die Preise der Wunschliste legen die Eltern fest.
				// Das Rezeptbuch und die Familien-Feeds pflegen die Eltern. Der
				// Link-Import ruft eine fremde Adresse auf (Paket abruf).
				r.Post("/recipes", d.rezepteSvc.Create)
				r.Post("/recipes/import", d.rezepteSvc.Import)
				r.Put("/recipes/{id}", d.rezepteSvc.Update)
				r.Delete("/recipes/{id}", d.rezepteSvc.Delete)
				r.Post("/recipes/{id}/shopping", d.rezepteSvc.ToShopping)
				r.Post("/admin/rss/feeds", d.newsSvc.CreateFamily)
				r.Delete("/admin/rss/feeds/{id}", d.newsSvc.DeleteFamily)
				r.Post("/admin/rss/refresh", d.newsSvc.Refresh)

				r.Put("/admin/wishes/{id}", d.wishesSvc.Decide)
				r.Delete("/admin/wishes/{id}", d.wishesSvc.Remove)
			})

			// The scoreboard spans chores and shopping, so it lives outside
			// either one. /chores/stats stays as the historical path.
			r.Get("/scoreboard", d.pointsSvc.Scoreboard)
			r.Get("/scoreboard/history", d.pointsSvc.History)
			// Die Monatsranglisten. Sie bleiben stehen, wenn die einmaligen
			// Aufgaben des Monats längst gelöscht sind.
			r.Get("/scoreboard/months", d.pointsSvc.Months)
			r.Get("/chores/stats", d.pointsSvc.Scoreboard)

			r.Get("/devices", d.devicesSvc.GetStatus)

			r.Get("/photos", d.photosSvc.List)
			r.Get("/photos/{name}", d.photosSvc.Serve)

			// Herunterladen darf jeder, auch das Wandgerät. Hochladen und
			// Löschen stehen weiter unten im Adminbereich.
			r.Get("/files", d.filesSvc.List)
			r.Get("/files/{name}", d.filesSvc.Serve)

			// Musik hören darf jeder, auch das Wandgerät — es ist Familienmusik
			// und keine persönliche. Nur das Neu-Einlesen ist Adminsache: Bei
			// einer grossen Sammlung ist das minutenlange Arbeit für die Platte.
			// Wer wann weg ist. Lesen darf jeder, auch das Wandgerät — im Flur
			// ist „ab wann sind alle da" genau die nützliche Frage.
			r.Get("/times", d.timesSvc.Overview)
			r.Get("/times/weekly", d.timesSvc.ListWeekly)
			r.Get("/times/days", d.timesSvc.ListDays)

			r.Get("/music/status", d.musicSvc.Status)
			r.Get("/music/browse", d.musicSvc.Browse)
			r.Get("/music/search", d.musicSvc.Search)
			r.Get("/music/track/{id}", d.musicSvc.Stream)

			r.Group(func(r chi.Router) {
				r.Use(d.authSvc.AdminMiddleware)
				r.Post("/auth/device", d.authSvc.EnableDevice)
				r.Put("/admin/settings", d.einstSvc.Put)
				r.Get("/admin/users", d.authSvc.ListUsers)
				r.Post("/admin/users", d.authSvc.CreateUser)
				r.Put("/admin/users/{id}", d.authSvc.UpdateUser)
				r.Delete("/admin/users/{id}", d.authSvc.DeleteUser)
				r.Put("/admin/weather/location", d.weatherSvc.SetLocation)
				r.Get("/admin/points", d.pointsSvc.AdminHistory)
				r.Post("/admin/points/reset", d.pointsSvc.ResetUser)
				r.Get("/admin/calendar/subscriptions", d.calendarSvc.ListSubscriptions)
				r.Post("/admin/calendar/subscriptions", d.calendarSvc.CreateSubscription)
				r.Post("/admin/calendar/subscriptions/{id}/refresh", d.calendarSvc.RefreshSubscription)
				r.Delete("/admin/calendar/subscriptions/{id}", d.calendarSvc.DeleteSubscription)
				r.Get("/admin/client", d.authSvc.Client)
				r.Get("/admin/notify", d.notifySvc.Status)
				r.Post("/admin/notify/test", d.notifySvc.Test)
				r.Get("/admin/devices", d.devicesSvc.List)
				r.Post("/admin/devices", d.devicesSvc.Create)
				r.Put("/admin/devices/{id}", d.devicesSvc.Update)
				r.Delete("/admin/devices/{id}", d.devicesSvc.Delete)
				r.Post("/admin/devices/reorder", d.devicesSvc.Reorder)
				r.Post("/admin/devices/test", d.devicesSvc.Test)
				r.Post("/music/rescan", d.musicSvc.Rescan)
				// Welcher Ordner eingehängt wird, steht in der .env — das kann
				// keine Weboberfläche ändern. Welcher Teil davon gehört wird,
				// schon.
				r.Get("/admin/music/folders", d.musicSvc.AdminFolders)
				r.Put("/admin/music/dir", d.musicSvc.SetDir)
				r.Post("/files", d.filesSvc.Upload)
				r.Delete("/files/{name}", d.filesSvc.Delete)
				r.Post("/admin/goal", d.goalsSvc.Create)
				r.Put("/admin/goal/{id}", d.goalsSvc.Update)
				r.Post("/admin/goal/{id}/close", d.goalsSvc.Close)
				r.Post("/admin/rewards", d.rewardsSvc.Create)
				r.Put("/admin/rewards/{id}", d.rewardsSvc.Update)
				r.Delete("/admin/rewards/{id}", d.rewardsSvc.Delete)
				r.Post("/admin/rewards/redemptions/{id}", d.rewardsSvc.Decide)
				r.Get("/admin/backups", d.backupSvc.ListBackups)
				r.Post("/admin/backup", d.backupSvc.TriggerBackup)
				r.Get("/admin/backup/download", d.backupSvc.Download)
			})
		})
	})

	return r
}
