package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
	"github.com/go-chi/chi/v5"
)

// Die Rechtematrix. Wer darf welchen Weg aufrufen — und zwar so, wie der
// Server wirklich verdrahtet ist (newRouter), nicht wie eine Liste im Kopf es
// behauptet.
//
// Fünf Gegenüber:
//
//	anon      niemand angemeldet
//	kind      ein Kind (Rolle member), angemeldet
//	papa      ein Elternteil (Rolle admin), angemeldet
//	wand      das Wandgerät, ohne PIN
//	wandPIN   das Wandgerät mit der PIN eines Elternteils
//
// Fünf Stufen, von offen bis streng:
//
//	öffentlich  jeder, auch ohne Anmeldung
//	alle        jeder, der angemeldet ist, auch das Wandgerät (lesen, reagieren)
//	person      nur eine angemeldete Person — am Wandgerät gesperrt
//	eltern      Eltern; am Wandgerät mit Eltern-PIN
//	admin       nur ein angemeldeter Administrator
//
// Jeder Weg, der etwas verändert, muss hier ausdrücklich stehen. Kommt ein
// neuer hinzu und niemand trägt ihn ein, schlägt der Test fehl — statt
// stillschweigend für alle offen zu sein.

type stufe string

const (
	oeffentlich stufe = "öffentlich"
	alle        stufe = "alle"
	person      stufe = "person"
	eltern      stufe = "eltern"
	admin       stufe = "admin"
)

var schreibwege = map[string]stufe{
	// öffentlich
	"POST /api/auth/login":  oeffentlich,
	"POST /api/auth/logout": oeffentlich,

	// alle: reagieren, nicht eintragen
	"DELETE /api/auth/device":          alle,
	"POST /api/chores/{id}/complete":   alle,
	"PUT /api/shopping/{id}/checked":   alle,
	"POST /api/shopping/clear-checked": alle,

	// person: gehört einer Person
	"PUT /api/auth/profile":                person,
	"POST /api/auth/pin":                   person,
	"PUT /api/preferences/{key}":           person,
	"POST /api/rewards/{id}/redeem":        person,
	"DELETE /api/rewards/redemptions/{id}": person,
	"POST /api/wishes":                     person,
	"DELETE /api/wishes/{id}":              person,
	"POST /api/wishes/{id}/redeem":         person,

	// eltern: eintragen, ändern, löschen, bestätigen
	"POST /api/auth/eltern":                        eltern,
	"POST /api/admin/points":                       eltern,
	"DELETE /api/admin/points/{id}":                eltern,
	"POST /api/chores/pending/{id}/approve":        eltern,
	"POST /api/chores/pending/{id}/reject":         eltern,
	"POST /api/calendar/events":                    eltern,
	"PUT /api/calendar/events/{id}":                eltern,
	"DELETE /api/calendar/events/{id}":             eltern,
	"POST /api/calendar/events/{id}/skip":          eltern,
	"DELETE /api/calendar/events/{id}/skip/{date}": eltern,
	"POST /api/shopping":                           eltern,
	"PUT /api/shopping/{id}":                       eltern,
	"DELETE /api/shopping/{id}":                    eltern,
	"DELETE /api/shopping/suggestions":             eltern,
	"PUT /api/meals/{day}":                         eltern,
	"DELETE /api/meals/{day}":                      eltern,
	"POST /api/meals/{day}/shopping":               eltern,
	"POST /api/notes":                              eltern,
	"PUT /api/notes/{id}":                          eltern,
	"DELETE /api/notes/{id}":                       eltern,
	"POST /api/chores":                             eltern,
	"PUT /api/chores/{id}":                         eltern,
	"DELETE /api/chores/done":                      eltern,
	"DELETE /api/chores/{id}":                      eltern,
	"POST /api/photos":                             eltern,
	"DELETE /api/photos/{name}":                    eltern,
	"POST /api/times/weekly":                       eltern,
	"DELETE /api/times/weekly/{id}":                eltern,
	"PUT /api/times/days":                          eltern,
	"POST /api/links":                              eltern,
	"PUT /api/links/{id}":                          eltern,
	"POST /api/links/{id}/pin":                     eltern,
	"DELETE /api/links/{id}":                       eltern,
	"POST /api/links/reorder":                      eltern,
	"PUT /api/admin/wishes/{id}":                   eltern,
	"DELETE /api/admin/wishes/{id}":                eltern,

	// admin
	"POST /api/auth/device":                               admin,
	"POST /api/admin/users":                               admin,
	"PUT /api/admin/users/{id}":                           admin,
	"DELETE /api/admin/users/{id}":                        admin,
	"PUT /api/admin/weather/location":                     admin,
	"POST /api/admin/points/reset":                        admin,
	"POST /api/admin/calendar/subscriptions":              admin,
	"POST /api/admin/calendar/subscriptions/{id}/refresh": admin,
	"DELETE /api/admin/calendar/subscriptions/{id}":       admin,
	"POST /api/admin/notify/test":                         admin,
	"POST /api/admin/devices":                             admin,
	"PUT /api/admin/devices/{id}":                         admin,
	"DELETE /api/admin/devices/{id}":                      admin,
	"POST /api/admin/devices/reorder":                     admin,
	"POST /api/admin/devices/test":                        admin,
	"POST /api/music/rescan":                              admin,
	"PUT /api/admin/music/dir":                            admin,
	"POST /api/files":                                     admin,
	"DELETE /api/files/{name}":                            admin,
	"POST /api/admin/goal":                                admin,
	"PUT /api/admin/goal/{id}":                            admin,
	"POST /api/admin/goal/{id}/close":                     admin,
	"POST /api/admin/rewards":                             admin,
	"PUT /api/admin/rewards/{id}":                         admin,
	"DELETE /api/admin/rewards/{id}":                      admin,
	"POST /api/admin/rewards/redemptions/{id}":            admin,
	"POST /api/admin/backup":                              admin,
}

// Lesewege, die nicht für alle Angemeldeten offen sind.
var lesewege = map[string]stufe{
	"GET /health":                           oeffentlich,
	"GET /api/health":                       oeffentlich,
	"GET /api/auth/users":                   oeffentlich,
	"GET /api/preferences/{key}":            person,
	"GET /api/admin/users":                  admin,
	"GET /api/admin/points":                 admin,
	"GET /api/admin/calendar/subscriptions": admin,
	"GET /api/admin/notify":                 admin,
	"GET /api/admin/client":                 admin,
	"GET /api/admin/devices":                admin,
	"GET /api/admin/music/folders":          admin,
	"GET /api/admin/backups":                admin,
	"GET /api/admin/backup/download":        admin,
}

// platzhalter macht aus einem Muster einen aufrufbaren Weg. Die Nummern gibt
// es nicht: Die Wege laufen ins Leere, statt Testdaten zu zerstören.
var platzhalter = strings.NewReplacer(
	"{id}", "999999", "{name}", "x.txt", "{day}", "2026-01-01",
	"{key}", "dashboard.layout", "{date}", "2026-01-01",
)

type gegenueber struct {
	name    string
	cookies []*http.Cookie
	kopf    map[string]string
}

func (g gegenueber) erlaubt(s stufe) bool {
	switch s {
	case oeffentlich:
		return true
	case alle:
		return g.name != "anon"
	case person:
		return g.name == "kind" || g.name == "papa"
	case eltern:
		return g.name == "papa" || g.name == "wandPIN"
	case admin:
		return g.name == "papa"
	}
	return false
}

// Meldungen, mit denen die Schutzschichten ablehnen. Ein 403 aus einem
// Handler (etwa „nicht deine Aufgabe") zählt nicht als Ablehnung der Rechte.
var sperrmeldungen = []string{
	"Nur für Admins", "Nur für Eltern", "Dafür bitte anmelden",
	"Dafür braucht es die PIN", "Das geht nur mit der PIN eines Elternteils", "Die PIN stimmt nicht",
}

func abgelehnt(rec *httptest.ResponseRecorder) bool {
	switch rec.Code {
	case http.StatusUnauthorized:
		return true
	case http.StatusForbidden:
		body := rec.Body.String()
		for _, m := range sperrmeldungen {
			if strings.Contains(body, m) {
				return true
			}
		}
	}
	return false
}

func testRouter(t *testing.T) http.Handler {
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
	sql := db.DB()
	shop := shopping.NewService(sql, nil)
	ns, _ := notify.Neu("", "")
	return newRouter(&config.Config{Server: config.Server{ProxyHops: 2}}, dienste{
		authSvc:     auth.NewService(sql, db, "0123456789012345678901234567890123456789", time.Hour, false),
		weatherSvc:  weather.NewService(sql, db, 48, 16, "UTC", time.Minute),
		calendarSvc: calendar.NewService(filepath.Join(dir, "ics"), 400, false, "UTC"),
		shoppingSvc: shop,
		notesSvc:    notes.NewService(sql, filepath.Join(dir, "notes"), false),
		choresSvc:   chores.NewService(sql),
		pointsSvc:   points.NewService(sql),
		devicesSvc:  devices.NewService(sql, time.Minute, time.Second),
		linksSvc:    links.NewService(sql),
		photosSvc:   photos.NewService(filepath.Join(dir, "photos")),
		filesSvc:    files.NewService(filepath.Join(dir, "files")),
		musicSvc:    music.NewService(sql, filepath.Join(dir, "music"), db),
		timesSvc:    times.NewService(sql),
		mealsSvc:    meals.NewService(sql, shop),
		rewardsSvc:  rewards.NewService(sql),
		goalsSvc:    goals.NewService(sql),
		wishesSvc:   wishes.NewService(sql),
		backupSvc: backup.NewService(sql, filepath.Join(dir, "db.sqlite"), filepath.Join(dir, "backup"),
			dir, 7, "0 3 * * *"),
		notifySvc: ns,
	})
}

func aufruf(h http.Handler, methode, pfad, body string, cookies []*http.Cookie, kopf map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(methode, pfad, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	for k, v := range kopf {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func anmelden(t *testing.T, h http.Handler, userID string) []*http.Cookie {
	t.Helper()
	rec := aufruf(h, http.MethodPost, "/api/auth/login", `{"user_id":`+userID+`,"pin":"1234"}`, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Anmeldung %s: %d %s", userID, rec.Code, rec.Body)
	}
	return rec.Result().Cookies()
}

func TestRechtematrix(t *testing.T) {
	h := testRouter(t)

	kind := anmelden(t, h, "3") // Standardperson Kind
	papa := anmelden(t, h, "1") // Standardperson Papa (Administrator)
	rec := aufruf(h, http.MethodPost, "/api/auth/device", "", papa, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("Wandgerät einrichten: %d %s", rec.Code, rec.Body)
	}
	var wandCookie []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.DeviceCookieName {
			wandCookie = append(wandCookie, c)
		}
	}
	if len(wandCookie) == 0 {
		t.Fatal("kein Geräte-Cookie erhalten")
	}

	leute := []gegenueber{
		{name: "anon"},
		{name: "kind", cookies: kind},
		{name: "papa", cookies: papa},
		{name: "wand", cookies: wandCookie},
		{name: "wandPIN", cookies: wandCookie, kopf: map[string]string{auth.ElternIDHeader: "1", auth.ElternPINHeader: "1234"}},
	}

	var routen []string
	stufen := map[string]stufe{}
	_ = chi.Walk(h.(chi.Routes), func(methode, muster string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		schluessel := methode + " " + muster
		routen = append(routen, schluessel)
		if s, ok := schreibwege[schluessel]; ok {
			stufen[schluessel] = s
		} else if s, ok := lesewege[schluessel]; ok {
			stufen[schluessel] = s
		} else if methode == http.MethodGet {
			stufen[schluessel] = alle // Lesen darf jeder Angemeldete, auch das Wandgerät
		}
		return nil
	})
	sort.Strings(routen)

	// Jeder verändernde Weg ist eingetragen, und jeder Eintrag gibt es noch.
	for _, r := range routen {
		if _, ok := stufen[r]; !ok {
			t.Errorf("%s steht nicht in der Rechtematrix — bitte einer Stufe zuordnen", r)
		}
	}
	for k := range schreibwege {
		if !contains(routen, k) {
			t.Errorf("%s steht in der Rechtematrix, aber nicht mehr im Router", k)
		}
	}
	for k := range lesewege {
		if !contains(routen, k) {
			t.Errorf("%s steht in der Rechtematrix, aber nicht mehr im Router", k)
		}
	}
	if len(routen) < 100 {
		t.Fatalf("auffällig wenige Wege: %d", len(routen))
	}

	for _, schluessel := range routen {
		s, ok := stufen[schluessel]
		if !ok {
			continue
		}
		methode, muster, _ := strings.Cut(schluessel, " ")
		pfad := platzhalter.Replace(muster)
		for _, g := range leute {
			rec := aufruf(h, methode, pfad, "", g.cookies, g.kopf)
			if got, want := !abgelehnt(rec), g.erlaubt(s); got != want {
				t.Errorf("%-8s %-45s Stufe %-10s: erwartet %s, war Status %d %.60q",
					g.name, schluessel, s, wort(want), rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		}
	}
}

func wort(erlaubt bool) string {
	if erlaubt {
		return "erlaubt"
	}
	return "abgelehnt"
}

func contains(liste []string, s string) bool {
	for _, x := range liste {
		if x == s {
			return true
		}
	}
	return false
}

// Der Wandgerät-Weg mit PIN: Eine falsche PIN öffnet nichts, und die Sperre
// greift je Gerät, nicht für Papa insgesamt.
func TestWandgeraetPINSperreBetrifftNurDasGeraet(t *testing.T) {
	h := testRouter(t)
	papa := anmelden(t, h, "1")
	rec := aufruf(h, http.MethodPost, "/api/auth/device", "", papa, nil)
	var wand []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.DeviceCookieName {
			wand = append(wand, c)
		}
	}
	falsch := map[string]string{auth.ElternIDHeader: "1", auth.ElternPINHeader: "0000"}
	for i := 0; i < 5; i++ {
		aufruf(h, http.MethodPost, "/api/admin/points", "", wand, falsch)
	}
	richtig := map[string]string{auth.ElternIDHeader: "1", auth.ElternPINHeader: "1234"}
	if rec := aufruf(h, http.MethodPost, "/api/admin/points", "", wand, richtig); rec.Code != http.StatusTooManyRequests {
		t.Errorf("das Wandgerät sollte gesperrt sein, war %d", rec.Code)
	}

	// Papa am Handy: eine andere Adresse. Weiter weg im Netz, aber kein Aussperren.
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"user_id":1,"pin":"1234"}`))
	r.RemoteAddr = "192.168.1.77:5000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("Papa am eigenen Handy sollte sich anmelden können, war %d %s", w.Code, w.Body)
	}
}

// Ein erfundener X-Forwarded-For-Kopf hebt die Sperre nicht auf.
func TestErfundeneAdresseUmgehtDieSperreNicht(t *testing.T) {
	h := testRouter(t)
	login := func(pin, xff string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"user_id":1,"pin":"`+pin+`"}`))
		r.RemoteAddr = "172.18.0.4:4000" // nginx
		// So kommt es von Traefik und nginx an: hinten die echten Stationen.
		r.Header.Set("X-Forwarded-For", xff+", 192.168.1.50, 172.18.0.2")
		r.Header.Set("X-Real-IP", "7.7.7.7")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 5; i++ {
		login("0000", "10.0.0."+string(rune('1'+i))) // jedes Mal eine andere „Absenderadresse"
	}
	if code := login("1234", "10.9.9.9"); code != http.StatusTooManyRequests {
		t.Errorf("trotz wechselnder Angaben im Kopf sollte das Gerät gesperrt sein, war %d", code)
	}
}
