// Package rezepte ist das Rezeptbuch der Familie. Ein Rezept entsteht auf zwei
// Wegen: von Hand — oder aus einem Link. Wer eine Adresse einträgt, bekommt das
// Rezept samt Zutaten, Schritten und Bild ins Buch, ohne etwas abzutippen.
//
// Lesen darf jeder, auch das Wandgerät (in der Küche gehört es dorthin).
// Eintragen, ändern und löschen ist Elternsache, wie bei allem anderen.
//
// Der Link-Import ist einer der Wege, auf denen der Server eine fremde Adresse
// aufruft. Er benutzt deshalb das Paket abruf mit seiner Sperre fürs Heimnetz.
package rezepte

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"family-dashboard/backend/internal/abruf"
	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

const (
	seiteMaxBytes  = 3 << 20
	bildMaxBytes   = 5 << 20
	abrufZeitlimit = 15 * time.Second
	maxRezepte     = 2000
)

var (
	errAus        = errors.New("Das Rezeptbuch ist ausgeschaltet")
	errSchonDa    = errors.New("Dieses Rezept ist schon im Rezeptbuch")
	errKeinText   = errors.New("Die Seite ist kein Text")
	errTitelFehlt = errors.New("Das Rezept braucht einen Titel")
)

// Schalter sagt, ob das Rezeptbuch eingeschaltet ist (Verwaltung → Bereiche).
type Schalter interface{ An(key string) bool }

// Einkauf ist der Teil der Einkaufsliste, den das Rezeptbuch braucht.
type Einkauf interface {
	AddItems(names []string, userID *int) (int, error)
}

type Service struct {
	db       *sql.DB
	dir      string
	einkauf  Einkauf
	schalter Schalter
	// clientFabrik ersetzt in Tests den Client mit der Sperre fürs Heimnetz.
	clientFabrik func() *http.Client
}

func NewService(db *sql.DB, dir string, einkauf Einkauf, schalter Schalter) *Service {
	return &Service{db: db, dir: dir, einkauf: einkauf, schalter: schalter}
}

func (s *Service) client() *http.Client {
	if s.clientFabrik != nil {
		return s.clientFabrik()
	}
	return abruf.NeuerClient(abrufZeitlimit)
}

func (s *Service) an(w http.ResponseWriter) bool {
	if s.schalter != nil && !s.schalter.An("modul.rezepte") {
		auth.HTTPError(w, http.StatusNotFound, errAus.Error())
		return false
	}
	return true
}

// Rezept ist ein Eintrag im Rezeptbuch.
type Rezept struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	SourceURL string    `json:"source_url"`
	HasImage  bool      `json:"has_image"`
	Servings  string    `json:"servings"`
	PrepMin   int       `json:"prep_min"`
	CookMin   int       `json:"cook_min"`
	TotalMin  int       `json:"total_min"`
	Category  string    `json:"category"`
	Note      string    `json:"note"`
	Zutaten   []string  `json:"ingredients"`
	Schritte  []string  `json:"steps"`
	CreatedAt time.Time `json:"created_at"`
}

const spalten = `id, title, source_url, image, servings, prep_min, cook_min, total_min,
	category, note, ingredients, steps, created_at`

type zeile interface{ Scan(dest ...any) error }

func lesen(z zeile) (Rezept, string, error) {
	var r Rezept
	var bild, zutaten, schritte string
	var angelegt sql.NullTime
	if err := z.Scan(&r.ID, &r.Title, &r.SourceURL, &bild, &r.Servings, &r.PrepMin, &r.CookMin,
		&r.TotalMin, &r.Category, &r.Note, &zutaten, &schritte, &angelegt); err != nil {
		return r, "", err
	}
	r.HasImage = bild != ""
	r.Zutaten = zeilen(zutaten)
	r.Schritte = zeilen(schritte)
	if angelegt.Valid {
		r.CreatedAt = angelegt.Time
	}
	return r, bild, nil
}

func zeilen(raw string) []string {
	out := []string{}
	for _, l := range strings.Split(raw, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func zusammen(l []string) string { return strings.Join(l, "\n") }

// ------------------------------------------------------------------ Lesen

// List liefert alle Rezepte, neueste zuerst. Die Suche (?q=) schaut in Titel,
// Kategorie und Zutaten.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	rows, err := s.db.Query(`SELECT `+spalten+` FROM recipes ORDER BY created_at DESC, id DESC LIMIT ?`, maxRezepte)
	if err != nil {
		log.Error().Err(err).Msg("Rezepte konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	out := []Rezept{}
	for rows.Next() {
		rez, _, err := lesen(rows)
		if err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		if q != "" && !trifft(rez, q) {
			continue
		}
		// Die Liste braucht die Schritte nicht; sie machen sie nur schwer.
		rez.Schritte = []string{}
		out = append(out, rez)
	}
	auth.WriteJSON(w, out)
}

func trifft(r Rezept, q string) bool {
	if strings.Contains(strings.ToLower(r.Title), q) || strings.Contains(strings.ToLower(r.Category), q) {
		return true
	}
	for _, z := range r.Zutaten {
		if strings.Contains(strings.ToLower(z), q) {
			return true
		}
	}
	return false
}

func (s *Service) holen(id int) (Rezept, string, error) {
	return lesen(s.db.QueryRow(`SELECT `+spalten+` FROM recipes WHERE id = ?`, id))
}

func (s *Service) ausAnfrage(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return 0, false
	}
	return id, true
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	id, ok := s.ausAnfrage(w, r)
	if !ok {
		return
	}
	rez, _, err := s.holen(id)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Rezept nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, rez)
}

// Image liefert das gespeicherte Bild. Der Dateiname kommt aus der Datenbank,
// nie aus der Anfrage.
func (s *Service) Image(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	id, ok := s.ausAnfrage(w, r)
	if !ok {
		return
	}
	_, bild, err := s.holen(id)
	if err != nil || bild == "" {
		auth.HTTPError(w, http.StatusNotFound, "Kein Bild")
		return
	}
	data, err := os.ReadFile(filepath.Join(s.dir, filepath.Base(bild)))
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Kein Bild")
		return
	}
	w.Header().Set("Content-Type", bildTyp(bild))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data)
}

func bildTyp(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}
	return "image/jpeg"
}

// ---------------------------------------------------------------- Schreiben

type eingabe struct {
	Title     string   `json:"title"`
	SourceURL string   `json:"source_url"`
	Servings  string   `json:"servings"`
	PrepMin   int      `json:"prep_min"`
	CookMin   int      `json:"cook_min"`
	TotalMin  int      `json:"total_min"`
	Category  string   `json:"category"`
	Note      string   `json:"note"`
	Zutaten   []string `json:"ingredients"`
	Schritte  []string `json:"steps"`
}

func (e *eingabe) pruefen() error {
	e.Title = bereinige(e.Title, maxTitel)
	if e.Title == "" {
		return errTitelFehlt
	}
	e.Servings = bereinige(e.Servings, 60)
	e.Category = bereinige(e.Category, 60)
	e.Note = strings.TrimSpace(e.Note)
	if utf8.RuneCountInString(e.Note) > maxNotiz {
		e.Note = string([]rune(e.Note)[:maxNotiz])
	}
	for _, m := range []*int{&e.PrepMin, &e.CookMin, &e.TotalMin} {
		if *m < 0 || *m > 60*24*14 {
			*m = 0
		}
	}
	if e.SourceURL != "" {
		a, err := abruf.Bereinigen(e.SourceURL)
		if err != nil {
			return err
		}
		e.SourceURL = a
	}
	e.Zutaten = bereinigeListe(e.Zutaten, maxZutaten, maxZutatLen)
	e.Schritte = bereinigeListe(e.Schritte, maxSchritte, maxSchrittLn)
	return nil
}

func bereinigeListe(in []string, anzahl, laenge int) []string {
	out := []string{}
	for _, z := range in {
		if z = bereinige(z, laenge); z != "" {
			out = append(out, z)
			if len(out) >= anzahl {
				break
			}
		}
	}
	return out
}

func (s *Service) eintragen(e eingabe, bildName string, user *int) (int, error) {
	var anzahl int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM recipes`).Scan(&anzahl); err == nil && anzahl >= maxRezepte {
		return 0, errors.New("Das Rezeptbuch ist voll")
	}
	var urheber any
	if user != nil {
		urheber = *user
	}
	res, err := s.db.Exec(`
		INSERT INTO recipes (title, source_url, image, servings, prep_min, cook_min, total_min,
			category, note, ingredients, steps, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Title, e.SourceURL, bildName, e.Servings, e.PrepMin, e.CookMin, e.TotalMin,
		e.Category, e.Note, zusammen(e.Zutaten), zusammen(e.Schritte), urheber)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

// Create trägt ein Rezept von Hand ein.
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	var e eingabe
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&e); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := e.pruefen(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	uid, ok := auth.GetUserID(r)
	var user *int
	if ok {
		user = &uid
	}
	id, err := s.eintragen(e, "", user)
	if err != nil {
		log.Error().Err(err).Msg("Rezept konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Das Rezept konnte nicht gespeichert werden")
		return
	}
	s.antwort(w, http.StatusCreated, id)
}

func (s *Service) antwort(w http.ResponseWriter, status, id int) {
	rez, _, err := s.holen(id)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(status)
	auth.WriteJSON(w, rez)
}

// Update ändert ein Rezept. Das Bild bleibt, wie es ist.
func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	id, ok := s.ausAnfrage(w, r)
	if !ok {
		return
	}
	var e eingabe
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&e); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if err := e.pruefen(); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.db.Exec(`
		UPDATE recipes SET title = ?, source_url = ?, servings = ?, prep_min = ?, cook_min = ?,
			total_min = ?, category = ?, note = ?, ingredients = ?, steps = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		e.Title, e.SourceURL, e.Servings, e.PrepMin, e.CookMin, e.TotalMin, e.Category,
		e.Note, zusammen(e.Zutaten), zusammen(e.Schritte), id)
	if err != nil {
		log.Error().Err(err).Msg("Rezept konnte nicht geändert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Das Rezept konnte nicht gespeichert werden")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		auth.HTTPError(w, http.StatusNotFound, "Rezept nicht gefunden")
		return
	}
	s.antwort(w, http.StatusOK, id)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	id, ok := s.ausAnfrage(w, r)
	if !ok {
		return
	}
	_, bild, err := s.holen(id)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Rezept nicht gefunden")
		return
	}
	if _, err := s.db.Exec(`DELETE FROM recipes WHERE id = ?`, id); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if bild != "" {
		_ = os.Remove(filepath.Join(s.dir, filepath.Base(bild)))
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToShopping setzt die Zutaten auf die Einkaufsliste, ohne Doppelte.
func (s *Service) ToShopping(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	id, ok := s.ausAnfrage(w, r)
	if !ok {
		return
	}
	rez, _, err := s.holen(id)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Rezept nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if len(rez.Zutaten) == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Dieses Rezept hat keine Zutaten")
		return
	}
	uid, okUser := auth.GetUserID(r)
	var user *int
	if okUser {
		user = &uid
	}
	added, err := s.einkauf.AddItems(rez.Zutaten, user)
	if err != nil {
		log.Error().Err(err).Msg("Zutaten konnten nicht auf die Liste")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, map[string]int{"added": added, "total": len(rez.Zutaten)})
}

// ------------------------------------------------------------------ Import

// Import holt ein Rezept von einer Adresse ins Rezeptbuch.
func (s *Service) Import(w http.ResponseWriter, r *http.Request) {
	if !s.an(w) {
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	adresse, err := abruf.Bereinigen(req.URL)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, err.Error())
		return
	}

	var da int
	if err := s.db.QueryRow(`SELECT id FROM recipes WHERE source_url = ?`, adresse).Scan(&da); err == nil {
		auth.HTTPError(w, http.StatusConflict, errSchonDa.Error())
		return
	}

	antwort, err := abruf.Holen(r.Context(), s.client(), adresse,
		"text/html, application/xhtml+xml;q=0.9, */*;q=0.1",
		"FamilienDashboard (Rezeptbuch)", seiteMaxBytes, abrufZeitlimit)
	if err != nil {
		auth.HTTPError(w, http.StatusBadGateway, err.Error())
		return
	}
	if antwort.Typ != "" && !strings.Contains(antwort.Typ, "html") && !strings.Contains(antwort.Typ, "xml") {
		auth.HTTPError(w, http.StatusBadGateway, errKeinText.Error())
		return
	}

	g, err := Lesen(antwort.Body, adresse)
	if err != nil {
		auth.HTTPError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	e := eingabe{
		Title: g.Titel, SourceURL: adresse, Servings: g.Portionen, PrepMin: g.Vorb, CookMin: g.Kochen,
		TotalMin: g.Gesamt, Category: g.Kategorie, Note: g.Notiz, Zutaten: g.Zutaten, Schritte: g.Schritte,
	}
	if err := e.pruefen(); err != nil {
		auth.HTTPError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	uid, ok := auth.GetUserID(r)
	var user *int
	if ok {
		user = &uid
	}
	id, err := s.eintragen(e, "", user)
	if err != nil {
		log.Error().Err(err).Msg("Rezept konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Das Rezept konnte nicht gespeichert werden")
		return
	}

	// Das Bild ist ein Zugewinn, keine Bedingung: Scheitert es, bleibt das
	// Rezept stehen.
	if g.Bild != "" {
		if name := s.bildHolen(r.Context(), id, g.Bild); name != "" {
			if _, err := s.db.Exec(`UPDATE recipes SET image = ? WHERE id = ?`, name, id); err != nil {
				_ = os.Remove(filepath.Join(s.dir, name))
			}
		}
	}
	s.antwort(w, http.StatusCreated, id)
}

// bildHolen lädt das Bild eines Rezepts auf den eigenen Datenträger. Es wird
// nicht verlinkt: Sonst riefe jedes Handy der Familie bei jedem Öffnen die
// fremde Seite an.
func (s *Service) bildHolen(ctx context.Context, id int, adresse string) string {
	a, err := abruf.Bereinigen(adresse)
	if err != nil {
		return ""
	}
	antwort, err := abruf.Holen(ctx, s.client(), a, "image/webp, image/png, image/jpeg, image/*;q=0.5",
		"FamilienDashboard (Rezeptbuch)", bildMaxBytes, abrufZeitlimit)
	if err != nil {
		log.Debug().Err(err).Msg("Rezeptbild nicht geholt")
		return ""
	}
	// Der Inhalt zählt, nicht die Angabe des Servers.
	var endung string
	switch http.DetectContentType(antwort.Body) {
	case "image/jpeg":
		endung = ".jpg"
	case "image/png":
		endung = ".png"
	case "image/gif":
		endung = ".gif"
	case "image/webp":
		endung = ".webp"
	default:
		return ""
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return ""
	}
	name := strconv.Itoa(id) + endung
	if err := os.WriteFile(filepath.Join(s.dir, name), antwort.Body, 0o644); err != nil {
		log.Error().Err(err).Msg("Rezeptbild konnte nicht gespeichert werden")
		return ""
	}
	return name
}
