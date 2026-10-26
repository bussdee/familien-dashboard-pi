// Package neuigkeiten ist der Feed-Leser der Familie: Zeitungen, Blogs, der
// Schulverteiler — alles, was einen RSS- oder Atom-Feed hat, an einem Ort.
//
// Es gibt zwei Arten von Listen:
//
//   - Familien-Feeds, die die Eltern pflegen und alle sehen (auch das
//     Wandgerät),
//   - persönliche Feeds, die jede Person für sich pflegt und nur sie selbst
//     sieht.
//
// Beides lässt sich in der Verwaltung abschalten (Schalter modul.neuigkeiten
// und modul.neuigkeiten_persoenlich, beide standardmäßig aus), und jede Person
// kann die Neuigkeiten für sich ausblenden.
//
// Das ist ein Weg, auf dem der Server fremde Adressen aufruft. Er benutzt
// deshalb das Paket abruf mit seiner Sperre fürs Heimnetz, und alles, was aus
// einem Feed kommt, wird als Klartext gespeichert.
package neuigkeiten

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/abruf"
	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

const (
	feedMaxBytes       = 2 << 20
	abrufZeitlimit     = 15 * time.Second
	intervall          = time.Hour
	ersterAbruf        = 45 * time.Second
	maxFamilienFeeds   = 12
	maxPersoenlich     = 8
	aufbewahrenTage    = 60
	aktualisierenPause = 30 * time.Second
)

// Schalter liefert die Schalter aus den Einstellungen.
type Schalter interface{ An(key string) bool }

type Service struct {
	db       *sql.DB
	schalter Schalter

	// clientFabrik ersetzt in Tests den Client mit der Sperre fürs Heimnetz.
	clientFabrik func() *http.Client

	mu         sync.Mutex
	letzteHand time.Time
	laeuft     bool
}

func NewService(db *sql.DB, schalter Schalter) *Service {
	return &Service{db: db, schalter: schalter}
}

func (s *Service) client() *http.Client {
	if s.clientFabrik != nil {
		return s.clientFabrik()
	}
	return abruf.NeuerClient(abrufZeitlimit)
}

func (s *Service) modulAn() bool { return s.schalter == nil || s.schalter.An("modul.neuigkeiten") }
func (s *Service) persoenlichAn() bool {
	return s.modulAn() && (s.schalter == nil || s.schalter.An("modul.neuigkeiten_persoenlich"))
}

// ---------------------------------------------------------------- Abrufen

// Feed ist eine Quelle.
type Feed struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Scope     string     `json:"scope"` // "familie" oder "persoenlich"
	OwnerID   *int       `json:"owner_id"`
	LastOK    *time.Time `json:"last_ok"`
	LastError string     `json:"last_error"`
	Count     int        `json:"count"`
	Editable  bool       `json:"editable"`
}

// feedHolen ruft die Adresse ab. Ist die Antwort eine Webseite, folgt ein
// Versuch mit dem Feed, auf den sie verweist.
func (s *Service) feedHolen(ctx context.Context, adresse string) (Gelesen, string, error) {
	for versuch := 0; versuch < 2; versuch++ {
		a, err := abruf.Holen(ctx, s.client(), adresse,
			"application/rss+xml, application/atom+xml, application/xml;q=0.9, text/xml;q=0.9, text/html;q=0.5, */*;q=0.1",
			"FamilienDashboard (Neuigkeiten)", feedMaxBytes, abrufZeitlimit)
		if err != nil {
			return Gelesen{}, adresse, err
		}
		if g, err := Lesen(a.Body); err == nil {
			return g, adresse, nil
		}
		if versuch == 0 && strings.Contains(a.Typ, "html") {
			if ziel := FeedInSeite(a.Body); ziel != "" {
				basis, err := url.Parse(adresse)
				rel, err2 := url.Parse(ziel)
				if err == nil && err2 == nil {
					if sauber, err := abruf.Bereinigen(basis.ResolveReference(rel).String()); err == nil {
						adresse = sauber
						continue
					}
				}
			}
		}
		return Gelesen{}, adresse, errKeinFeed
	}
	return Gelesen{}, adresse, errKeinFeed
}

// aktualisieren holt einen Feed und hält das Ergebnis fest. Schlägt der Abruf
// fehl, bleiben die bisherigen Meldungen stehen.
func (s *Service) aktualisieren(ctx context.Context, id int, adresse string) error {
	g, _, err := s.feedHolen(ctx, adresse)
	if err != nil {
		if _, dberr := s.db.Exec(`UPDATE rss_feeds SET last_error = ? WHERE id = ?`, err.Error(), id); dberr != nil {
			log.Error().Err(dberr).Msg("Feed-Fehler konnte nicht festgehalten werden")
		}
		return err
	}
	return s.speichern(id, g)
}

func (s *Service) speichern(id int, g Gelesen) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, e := range g.Eintraege {
		// Eine bekannte Meldung behält ihr Datum, wenn der Feed keines nennt:
		// Sonst rutschte sie bei jedem Abruf wieder nach oben.
		if _, err := tx.Exec(`
			INSERT INTO rss_items (feed_id, guid, title, link, summary, published)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(feed_id, guid) DO UPDATE SET
				title = excluded.title, link = excluded.link, summary = excluded.summary,
				published = CASE WHEN ? THEN rss_items.published ELSE excluded.published END`,
			id, e.GUID, e.Titel, e.Link, e.Text, e.Veroeff.UTC(), e.zeitFehlt); err != nil {
			return err
		}
	}
	// Ältere Meldungen räumen sich selbst weg: je Feed die neuesten 40, und
	// nichts, was älter ist als zwei Monate.
	if _, err := tx.Exec(`
		DELETE FROM rss_items WHERE feed_id = ? AND (
			published < datetime('now', ?) OR id NOT IN (
				SELECT id FROM rss_items WHERE feed_id = ? ORDER BY published DESC, id DESC LIMIT ?))`,
		id, "-"+strconv.Itoa(aufbewahrenTage)+" days", id, maxEintraegeJeFeed); err != nil {
		return err
	}
	if g.Titel != "" {
		if _, err := tx.Exec(`UPDATE rss_feeds SET title = ? WHERE id = ? AND title = ''`, g.Titel, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE rss_feeds SET last_ok = ?, last_error = '' WHERE id = ?`, time.Now().UTC(), id); err != nil {
		return err
	}
	return tx.Commit()
}

type kurzFeed struct {
	id  int
	url string
}

func (s *Service) alleFeeds() ([]kurzFeed, error) {
	rows, err := s.db.Query(`SELECT id, url FROM rss_feeds ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []kurzFeed
	for rows.Next() {
		var f kurzFeed
		if err := rows.Scan(&f.id, &f.url); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// AlleAktualisieren holt jeden Feed. Läuft schon ein Durchgang, kommt kein
// zweiter dazu.
func (s *Service) AlleAktualisieren(ctx context.Context) {
	s.mu.Lock()
	if s.laeuft {
		s.mu.Unlock()
		return
	}
	s.laeuft = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.laeuft = false
		s.mu.Unlock()
	}()

	liste, err := s.alleFeeds()
	if err != nil {
		log.Error().Err(err).Msg("Feeds konnten nicht gelesen werden")
		return
	}
	for _, f := range liste {
		if ctx.Err() != nil {
			return
		}
		if err := s.aktualisieren(ctx, f.id, f.url); err != nil {
			log.Warn().Err(err).Int("id", f.id).Msg("Feed nicht aktualisiert")
		}
	}
}

// Start hält die Feeds aktuell: kurz nach dem Start, dann jede Stunde. Ist der
// Bereich ausgeschaltet, ruft der Server nichts ab.
func (s *Service) Start(ctx context.Context) {
	wait := ersterAbruf
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if s.modulAn() {
			s.AlleAktualisieren(ctx)
		}
		wait = intervall
	}
}

// ---------------------------------------------------------------- Lesen

// sichtbar liefert die Bedingung, welche Feeds jemand sehen darf.
func (s *Service) sichtbar(r *http.Request) (bedingung string, args []any) {
	bedingung = "f.owner_id IS NULL"
	if uid, ok := auth.GetUserID(r); ok && s.persoenlichAn() {
		bedingung += " OR f.owner_id = ?"
		args = append(args, uid)
	}
	return "(" + bedingung + ")", args
}

func (s *Service) darfBearbeiten(r *http.Request, owner *int) bool {
	role, _ := auth.GetUserRole(r)
	if owner == nil {
		return role == "admin" && !auth.IsDevice(r)
	}
	uid, ok := auth.GetUserID(r)
	return ok && uid == *owner
}

func (s *Service) ListFeeds(w http.ResponseWriter, r *http.Request) {
	if !s.modulAn() {
		auth.WriteJSON(w, []Feed{})
		return
	}
	bed, args := s.sichtbar(r)
	rows, err := s.db.Query(`
		SELECT f.id, f.title, f.url, f.owner_id, f.last_ok, f.last_error,
		       (SELECT COUNT(*) FROM rss_items i WHERE i.feed_id = f.id)
		FROM rss_feeds f WHERE `+bed+` ORDER BY f.owner_id IS NOT NULL, f.title COLLATE NOCASE, f.id`, args...)
	if err != nil {
		log.Error().Err(err).Msg("Feeds konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	out := []Feed{}
	for rows.Next() {
		var f Feed
		var owner sql.NullInt64
		var ok sql.NullTime
		if err := rows.Scan(&f.ID, &f.Title, &f.URL, &owner, &ok, &f.LastError, &f.Count); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		f.Scope = "familie"
		if owner.Valid {
			o := int(owner.Int64)
			f.OwnerID = &o
			f.Scope = "persoenlich"
		}
		if ok.Valid {
			t := ok.Time
			f.LastOK = &t
		}
		f.Editable = s.darfBearbeiten(r, f.OwnerID)
		if !f.Editable {
			f.LastError = "" // Fehlersuche ist Sache dessen, der den Feed pflegt
		}
		out = append(out, f)
	}
	auth.WriteJSON(w, out)
}

// Meldung ist ein Eintrag für die Oberfläche.
type Meldung struct {
	ID        int       `json:"id"`
	FeedID    int       `json:"feed_id"`
	FeedTitle string    `json:"feed_title"`
	Scope     string    `json:"scope"`
	Title     string    `json:"title"`
	Link      string    `json:"link"`
	Summary   string    `json:"summary"`
	Published time.Time `json:"published"`
}

func (s *Service) Items(w http.ResponseWriter, r *http.Request) {
	if !s.modulAn() {
		auth.WriteJSON(w, []Meldung{})
		return
	}
	limit := 40
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = min(v, 200)
	}
	bed, args := s.sichtbar(r)
	q := `SELECT i.id, f.id, f.title, f.owner_id, i.title, i.link, i.summary, i.published
	      FROM rss_items i JOIN rss_feeds f ON f.id = i.feed_id WHERE ` + bed
	if v := r.URL.Query().Get("feed"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
			return
		}
		q += " AND f.id = ?"
		args = append(args, id)
	}
	q += ` ORDER BY i.published DESC, i.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		log.Error().Err(err).Msg("Meldungen konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()
	out := []Meldung{}
	for rows.Next() {
		var m Meldung
		var owner sql.NullInt64
		if err := rows.Scan(&m.ID, &m.FeedID, &m.FeedTitle, &owner, &m.Title, &m.Link, &m.Summary, &m.Published); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		m.Scope = "familie"
		if owner.Valid {
			m.Scope = "persoenlich"
		}
		out = append(out, m)
	}
	auth.WriteJSON(w, out)
}

// -------------------------------------------------------------- Schreiben

type neuerFeed struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (s *Service) anlegen(w http.ResponseWriter, r *http.Request, owner *int) {
	var req neuerFeed
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	titel := abruf.Klartext(req.Title, 80, false)
	roh := strings.TrimSpace(req.URL)
	if strings.HasPrefix(strings.ToLower(roh), "feed://") {
		roh = "https://" + roh[len("feed://"):]
	}
	adresse, err := abruf.Bereinigen(roh)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	grenze, wo := maxFamilienFeeds, "owner_id IS NULL"
	args := []any{}
	if owner != nil {
		grenze, wo = maxPersoenlich, "owner_id = ?"
		args = append(args, *owner)
	}
	var anzahl int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM rss_feeds WHERE `+wo, args...).Scan(&anzahl); err == nil && anzahl >= grenze {
		auth.HTTPError(w, http.StatusBadRequest, "Weitere Feeds sind nicht möglich")
		return
	}

	// Erst holen, dann eintragen: Ein Feed, der nicht funktioniert, soll gar
	// nicht erst in der Liste stehen. Weist die Adresse auf eine Webseite, wird
	// der Feed gespeichert, auf den sie verweist.
	g, endgueltig, err := s.feedHolen(r.Context(), adresse)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	if titel == "" {
		titel = g.Titel
	}
	if titel == "" {
		if u, err := url.Parse(endgueltig); err == nil {
			titel = strings.TrimPrefix(u.Hostname(), "www.")
		}
	}

	var da int
	if err := s.db.QueryRow(`SELECT id FROM rss_feeds WHERE url = ? AND `+wo, append([]any{endgueltig}, args...)...).Scan(&da); err == nil {
		auth.HTTPError(w, http.StatusConflict, "Diesen Feed gibt es schon")
		return
	}

	var besitzer any
	if owner != nil {
		besitzer = *owner
	}
	res, err := s.db.Exec(`INSERT INTO rss_feeds (title, url, owner_id) VALUES (?, ?, ?)`, titel, endgueltig, besitzer)
	if err != nil {
		log.Error().Err(err).Msg("Feed konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	id64, _ := res.LastInsertId()
	if err := s.speichern(int(id64), g); err != nil {
		log.Error().Err(err).Msg("Meldungen konnten nicht gespeichert werden")
	}

	w.WriteHeader(http.StatusCreated)
	s.ListFeeds(w, r)
}

// CreatePersonal legt einen persönlichen Feed an: für jede angemeldete Person,
// wenn die Verwaltung das erlaubt.
func (s *Service) CreatePersonal(w http.ResponseWriter, r *http.Request) {
	if !s.persoenlichAn() {
		auth.HTTPError(w, http.StatusForbidden, "Persönliche Feeds sind ausgeschaltet")
		return
	}
	uid, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}
	s.anlegen(w, r, &uid)
}

// CreateFamily legt einen Familien-Feed an: Elternsache.
func (s *Service) CreateFamily(w http.ResponseWriter, r *http.Request) {
	if !s.modulAn() {
		auth.HTTPError(w, http.StatusForbidden, "Die Neuigkeiten sind ausgeschaltet")
		return
	}
	s.anlegen(w, r, nil)
}

func (s *Service) loeschen(w http.ResponseWriter, r *http.Request, familie bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var owner sql.NullInt64
	if err := s.db.QueryRow(`SELECT owner_id FROM rss_feeds WHERE id = ?`, id).Scan(&owner); err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Feed nicht gefunden")
		return
	}
	// Der Weg bestimmt, was gelöscht werden darf: Über den Eltern-Weg nur
	// Familien-Feeds, über den persönlichen Weg nur die eigenen.
	if familie != !owner.Valid {
		auth.HTTPError(w, http.StatusNotFound, "Feed nicht gefunden")
		return
	}
	if owner.Valid {
		uid, ok := auth.GetUserID(r)
		if !ok || uid != int(owner.Int64) {
			auth.HTTPError(w, http.StatusNotFound, "Feed nicht gefunden")
			return
		}
	}
	if _, err := s.db.Exec(`DELETE FROM rss_feeds WHERE id = ?`, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) DeletePersonal(w http.ResponseWriter, r *http.Request) { s.loeschen(w, r, false) }
func (s *Service) DeleteFamily(w http.ResponseWriter, r *http.Request)   { s.loeschen(w, r, true) }

// Refresh holt alle Feeds jetzt. Eine Pause schützt die fremden Seiten vor
// einem Finger, der immer wieder tippt.
func (s *Service) Refresh(w http.ResponseWriter, r *http.Request) {
	if !s.modulAn() {
		auth.HTTPError(w, http.StatusForbidden, "Die Neuigkeiten sind ausgeschaltet")
		return
	}
	s.mu.Lock()
	if time.Since(s.letzteHand) < aktualisierenPause {
		s.mu.Unlock()
		auth.HTTPError(w, http.StatusTooManyRequests, "Bitte einen Moment warten")
		return
	}
	s.letzteHand = time.Now()
	s.mu.Unlock()

	s.AlleAktualisieren(r.Context())
	s.ListFeeds(w, r)
}
