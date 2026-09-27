// Package meals ist der Essensplan: ein Gericht je Tag, dazu auf Wunsch eine
// Notiz und die Zutaten. Er beantwortet die meistgestellte Frage in einer
// Küche — „Was gibt's heute?" — und die zweitmeiste gleich mit: „Haben wir
// dafür alles?"
//
// Eintragen darf jeder, auch das Wandgerät: Der Plan gehört der Familie, und
// er wird ohnehin meist am Tablet in der Küche gemacht.
package meals

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

const (
	dayLayout      = "2006-01-02"
	maxDays        = 42
	maxIngredients = 40
)

// ShoppingAdder ist der Teil der Einkaufsliste, den der Essensplan braucht.
type ShoppingAdder interface {
	AddItems(names []string, userID *int) (int, error)
}

type Service struct {
	db       *sql.DB
	shopping ShoppingAdder
}

func NewService(db *sql.DB, shopping ShoppingAdder) *Service {
	return &Service{db: db, shopping: shopping}
}

type Meal struct {
	Day         string   `json:"day"`
	Title       string   `json:"title"`
	Note        string   `json:"note"`
	Ingredients []string `json:"ingredients"`
}

// ------------------------------------------------------------------ Lesen

// List liefert die eingetragenen Tage eines Zeitraums. Leere Tage fehlen —
// die Oberfläche weiss selbst, welche Tage es gibt.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	from := time.Now()
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		t, err := time.ParseInLocation(dayLayout, v, time.Local)
		if err != nil {
			auth.HTTPError(w, http.StatusBadRequest, "Ungültiges Datum")
			return
		}
		from = t
	}
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = min(n, maxDays)
		}
	}
	start := from.Format(dayLayout)
	end := from.AddDate(0, 0, days-1).Format(dayLayout)

	rows, err := s.db.Query(`
		SELECT day, title, note, ingredients FROM meals
		WHERE day BETWEEN ? AND ? ORDER BY day`, start, end)
	if err != nil {
		log.Error().Err(err).Msg("Essensplan konnte nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	meals := []Meal{}
	for rows.Next() {
		var m Meal
		var ingredients string
		if err := rows.Scan(&m.Day, &m.Title, &m.Note, &ingredients); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		m.Ingredients = splitIngredients(ingredients)
		meals = append(meals, m)
	}
	auth.WriteJSON(w, map[string]any{"from": start, "to": end, "meals": meals})
}

// Recent sind die Gerichte der letzten Monate, häufigste zuerst. Daraus wird
// die Auswahl „Schon mal gekocht" — Familien kochen im Kreis, und das ist
// gut so.
func (s *Service) Recent(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT title, COUNT(*) AS n, MAX(day) AS last,
		       (SELECT m2.ingredients FROM meals m2 WHERE m2.title = m.title
		         ORDER BY m2.day DESC LIMIT 1) AS ingredients
		FROM meals m
		WHERE day >= date('now', '-180 days')
		GROUP BY title
		ORDER BY n DESC, last DESC
		LIMIT 16`)
	if err != nil {
		log.Error().Err(err).Msg("Gerichte konnten nicht gelesen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	type recent struct {
		Title       string   `json:"title"`
		Count       int      `json:"count"`
		Ingredients []string `json:"ingredients"`
	}
	list := []recent{}
	for rows.Next() {
		var rc recent
		var last, ingredients string
		if err := rows.Scan(&rc.Title, &rc.Count, &last, &ingredients); err != nil {
			continue
		}
		rc.Ingredients = splitIngredients(ingredients)
		list = append(list, rc)
	}
	auth.WriteJSON(w, list)
}

// ------------------------------------------------------------- Schreiben

type mealPayload struct {
	Title       string   `json:"title"`
	Note        string   `json:"note"`
	Ingredients []string `json:"ingredients"`
}

func (s *Service) Save(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r)
	if !ok {
		return
	}
	var p mealPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	m, msg := normalize(day, p)
	if msg != "" {
		auth.HTTPError(w, http.StatusBadRequest, msg)
		return
	}

	if _, err := s.db.Exec(`
		INSERT INTO meals (day, title, note, ingredients) VALUES (?, ?, ?, ?)
		ON CONFLICT(day) DO UPDATE SET
			title = excluded.title, note = excluded.note,
			ingredients = excluded.ingredients, updated_at = CURRENT_TIMESTAMP`,
		m.Day, m.Title, m.Note, strings.Join(m.Ingredients, "\n")); err != nil {
		log.Error().Err(err).Msg("Essen konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, m)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r)
	if !ok {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM meals WHERE day = ?`, day); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToShopping setzt die Zutaten eines Tages auf die Einkaufsliste. Was schon
// offen draufsteht, kommt nicht doppelt dazu.
func (s *Service) ToShopping(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r)
	if !ok {
		return
	}
	var ingredients string
	err := s.db.QueryRow(`SELECT ingredients FROM meals WHERE day = ?`, day).Scan(&ingredients)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Für diesen Tag ist nichts eingetragen")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	list := splitIngredients(ingredients)
	if len(list) == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Zu diesem Gericht sind keine Zutaten eingetragen")
		return
	}

	var who *int
	if id, ok := auth.GetUserID(r); ok {
		who = &id
	}
	added, err := s.shopping.AddItems(list, who)
	if err != nil {
		log.Error().Err(err).Msg("Zutaten konnten nicht übernommen werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, map[string]int{"added": added, "skipped": len(list) - added})
}

// ---------------------------------------------------------------- Helfer

func parseDay(w http.ResponseWriter, r *http.Request) (string, bool) {
	day := chi.URLParam(r, "day")
	if _, err := time.Parse(dayLayout, day); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültiges Datum")
		return "", false
	}
	return day, true
}

// normalize bereinigt einen Eintrag und sagt, was daran nicht stimmt.
func normalize(day string, p mealPayload) (Meal, string) {
	m := Meal{
		Day:         day,
		Title:       strings.TrimSpace(p.Title),
		Note:        strings.TrimSpace(p.Note),
		Ingredients: []string{},
	}
	switch {
	case m.Title == "":
		return m, "Was gibt es denn?"
	case utf8.RuneCountInString(m.Title) > 80:
		return m, "Der Name des Gerichts ist zu lang (höchstens 80 Zeichen)"
	case utf8.RuneCountInString(m.Note) > 500:
		return m, "Die Notiz ist zu lang (höchstens 500 Zeichen)"
	}

	seen := map[string]bool{}
	for _, raw := range p.Ingredients {
		// Auch „Tomaten, Zwiebeln" in einer Zeile ist eine Liste.
		for _, part := range strings.Split(raw, ",") {
			item := strings.TrimSpace(part)
			key := strings.ToLower(item)
			if item == "" || seen[key] {
				continue
			}
			if utf8.RuneCountInString(item) > 80 {
				return m, "Eine Zutat ist zu lang (höchstens 80 Zeichen)"
			}
			seen[key] = true
			m.Ingredients = append(m.Ingredients, item)
		}
	}
	if len(m.Ingredients) > maxIngredients {
		return m, "Höchstens 40 Zutaten"
	}
	return m, ""
}

func splitIngredients(raw string) []string {
	out := []string{}
	for _, line := range strings.Split(raw, "\n") {
		if item := strings.TrimSpace(line); item != "" {
			out = append(out, item)
		}
	}
	return out
}
