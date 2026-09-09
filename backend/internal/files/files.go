// Package files ist die Ablage für Familiendateien: Der Administrator lädt
// hoch, alle laden herunter — am Wandgerät ebenfalls, denn das sind
// Familiendateien und keine persönlichen.
//
// Das Modul ist bewusst nah am Fotomodul gebaut. Zwei Unterschiede gibt es:
//
//   - Es gibt keine erlaubte Liste von Dateitypen. Eine Familie legt hier die
//     Bedienungsanleitung der Waschmaschine ab, den Elternbrief als PDF und
//     die Steuerbescheinigung — eine Endungsliste würde ständig im Weg stehen.
//     Dafür wird jede Datei ausschliesslich als Anhang ausgeliefert und nie im
//     Browser dargestellt. Damit kann eine hochgeladene HTML-Datei kein
//     Skript im Namen des Dashboards ausführen.
//   - Auf einem Pi ist der Platz endlich. Die Liste sagt deshalb mit, wie viel
//     noch frei ist, und warnt, bevor es eng wird.
package files

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

const (
	// Ein Video vom Schulfest ist schnell 200 MB gross, ein Pi hat aber
	// selten viel Luft. 100 MB je Datei ist der Kompromiss: gross genug für
	// Anleitungen, Formulare und ein kurzes Video, klein genug, dass eine
	// einzelne Datei die Karte nicht füllt.
	maxFileBytes  = 100 << 20
	maxUploadSize = 300 << 20 // ganzer Vorgang, also ein Schwung Dateien

	// Ab hier meldet die Liste "es wird eng". 1 GB reicht noch für einige
	// Dateien, ist aber früh genug, um vor dem vollen Datenträger zu warnen.
	engAbBytes = 1 << 30
)

type File struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type Service struct {
	dir string

	mu     sync.RWMutex
	cache  []File
	cached time.Time
}

func NewService(dir string) *Service {
	return &Service{dir: dir}
}

// ------------------------------------------------------------------- Auflisten

type listResponse struct {
	Files []File `json:"files"`
	Count int    `json:"count"`
	// Used ist die Summe der abgelegten Dateien, Free der freie Platz auf dem
	// Datenträger. Tight sagt der Oberfläche, dass sie das erwähnen soll.
	Used  int64 `json:"used_bytes"`
	Free  int64 `json:"free_bytes"`
	Tight bool  `json:"tight"`
	// MaxFile gehört in die Antwort, damit die Oberfläche die Grenze nennen
	// kann, ohne sie ein zweites Mal zu kennen.
	MaxFile int64 `json:"max_file_bytes"`
}

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	dateien, err := s.list()
	if err != nil {
		log.Debug().Err(err).Str("dir", s.dir).Msg("Datei-Ordner nicht lesbar")
		dateien = []File{}
	}

	var belegt int64
	for _, f := range dateien {
		belegt += f.Size
	}
	frei := freierPlatz(s.dir)

	auth.WriteJSON(w, listResponse{
		Files:   dateien,
		Count:   len(dateien),
		Used:    belegt,
		Free:    frei,
		Tight:   frei > 0 && frei < engAbBytes,
		MaxFile: maxFileBytes,
	})
}

func (s *Service) list() ([]File, error) {
	s.mu.RLock()
	if time.Since(s.cached) < 30*time.Second && s.cache != nil {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	dateien := []File{}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		dateien = append(dateien, File{Name: e.Name(), Size: info.Size(), Modified: info.ModTime()})
	}
	// Das Neueste zuerst: Wer hier nachsieht, sucht meistens das, was gerade
	// hochgeladen wurde.
	sort.Slice(dateien, func(i, j int) bool {
		return dateien[i].Modified.After(dateien[j].Modified)
	})

	s.mu.Lock()
	s.cache, s.cached = dateien, time.Now()
	s.mu.Unlock()
	return dateien, nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.cached = time.Time{}
	s.mu.Unlock()
}

// ------------------------------------------------------------------ Ausliefern

// sicherName reduziert einen Namen aus der URL auf seinen letzten Bestandteil.
// Damit kann "../" den Ordner nicht verlassen — dieselbe Prüfung wie im
// Fotomodul.
func sicherName(roh string) string {
	name := filepath.Base(filepath.Clean("/" + roh))
	if name == "." || name == "/" || name == ".." {
		return ""
	}
	return name
}

// Serve liefert eine Datei aus — immer als Anhang, nie zur Anzeige im
// Browser. Der Unterschied ist keine Kleinigkeit: Ohne ihn könnte eine
// hochgeladene HTML-Datei unter der Adresse des Dashboards laufen und an die
// Sitzung der Familie kommen.
func (s *Service) Serve(w http.ResponseWriter, r *http.Request) {
	name := sicherName(chi.URLParam(r, "name"))
	if name == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	ohneSchreibfrist(w)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Zweimal derselbe Name: einmal schlicht für alte Browser, einmal
	// UTF-8-kodiert für Umlaute. Ohne die zweite Form wird aus "Elternbrief
	// Höflichkeit.pdf" beim Speichern Zeichensalat.
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="%s"; filename*=UTF-8''%s`,
		schlichterName(name), url.PathEscape(name)))
	http.ServeContent(w, r, name, info.ModTime(), f)
}

// schlichterName wirft alles weg, was in einem Header-Wert in
// Anführungszeichen Ärger macht: Anführungszeichen, Backslashes, Zeilenumbrüche
// und alles jenseits von ASCII.
var nichtASCII = regexp.MustCompile(`[^\x20-\x7e]|["\\]`)

func schlichterName(name string) string {
	schlicht := strings.TrimSpace(nichtASCII.ReplaceAllString(name, "_"))
	if schlicht == "" {
		return "datei"
	}
	return schlicht
}

// ------------------------------------------------------------------ Hochladen

var unsicher = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)

// ablageName behält den Namen, den die Datei beim Hochladen hatte — anders als
// beim Foto, wo ein Zeitstempel angehängt wird. Wer eine Anleitung sucht, sucht
// nach ihrem Namen, nicht nach "anleitung_20260909-213000.pdf". Bei einem
// Zusammenstoss wird stattdessen durchnummeriert.
func ablageName(original string) string {
	basis := filepath.Base(original)
	endung := filepath.Ext(basis)
	name := strings.TrimSuffix(basis, endung)

	name = strings.Trim(unsicher.ReplaceAllString(name, "_"), "_. ")
	endung = unsicher.ReplaceAllString(endung, "")
	// Aus ".." macht filepath.Ext einen einzelnen Punkt. Bliebe der stehen,
	// hiesse die Datei danach "datei." — harmlos, aber schlampig.
	if strings.Trim(endung, ".") == "" {
		endung = ""
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	if name == "" {
		name = "datei"
	}
	return name + endung
}

// freierAblageName hängt " (2)", " (3)" an, bis der Name frei ist. Ohne das
// würde eine zweite "Anleitung.pdf" die erste überschreiben.
func (s *Service) freierAblageName(gewuenscht string) (string, error) {
	endung := filepath.Ext(gewuenscht)
	stamm := strings.TrimSuffix(gewuenscht, endung)

	for n := 1; n < 1000; n++ {
		kandidat := gewuenscht
		if n > 1 {
			kandidat = fmt.Sprintf("%s (%d)%s", stamm, n, endung)
		}
		if _, err := os.Stat(filepath.Join(s.dir, kandidat)); os.IsNotExist(err) {
			return kandidat, nil
		}
	}
	return "", fmt.Errorf("zu viele Dateien mit diesem Namen")
}

type uploadResult struct {
	Uploaded []string          `json:"uploaded"`
	Skipped  map[string]string `json:"skipped,omitempty"`
	Count    int               `json:"count"`
}

// Upload nimmt eine oder mehrere Dateien aus dem Formularfeld "files" an.
// Nur für Administratoren — die Route ist entsprechend eingehängt.
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		log.Error().Err(err).Msg("Datei-Ordner nicht anlegbar")
		auth.HTTPError(w, http.StatusInternalServerError, "Datei-Ordner nicht beschreibbar")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		auth.HTTPError(w, http.StatusRequestEntityTooLarge,
			"Upload zu groß (maximal 300 MB pro Vorgang)")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	headers := r.MultipartForm.File["files"]
	if len(headers) == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Keine Datei ausgewählt")
		return
	}

	ergebnis := uploadResult{Uploaded: []string{}, Skipped: map[string]string{}}
	for _, header := range headers {
		name, err := s.speichern(header)
		if err != nil {
			ergebnis.Skipped[header.Filename] = err.Error()
			continue
		}
		ergebnis.Uploaded = append(ergebnis.Uploaded, name)
	}
	ergebnis.Count = len(ergebnis.Uploaded)

	if len(ergebnis.Skipped) == 0 {
		ergebnis.Skipped = nil
	}
	if ergebnis.Count == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Keine Datei konnte übernommen werden")
		return
	}

	s.invalidate()
	log.Info().Int("count", ergebnis.Count).Msg("Dateien hochgeladen")
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, ergebnis)
}

func (s *Service) speichern(header *multipart.FileHeader) (string, error) {
	if header.Size > maxFileBytes {
		return "", fmt.Errorf("Datei zu groß (maximal 100 MB)")
	}

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("Datei nicht lesbar")
	}
	defer src.Close()

	name, err := s.freierAblageName(ablageName(header.Filename))
	if err != nil {
		return "", err
	}
	ziel := filepath.Join(s.dir, name)

	dst, err := os.OpenFile(ziel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("Speichern fehlgeschlagen")
	}

	// Ein Byte mehr als erlaubt lesen: Nur so lässt sich eine Datei erkennen,
	// deren gemeldete Grösse gelogen war.
	geschrieben, err := io.Copy(dst, io.LimitReader(src, maxFileBytes+1))
	schliessFehler := dst.Close()
	if err != nil || schliessFehler != nil || geschrieben > maxFileBytes {
		_ = os.Remove(ziel)
		if geschrieben > maxFileBytes {
			return "", fmt.Errorf("Datei zu groß (maximal 100 MB)")
		}
		return "", fmt.Errorf("Speichern fehlgeschlagen")
	}

	return name, nil
}

// Delete entfernt eine Datei. Nur für Administratoren.
func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	name := sicherName(chi.URLParam(r, "name"))
	if name == "" {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültiger Dateiname")
		return
	}

	if err := os.Remove(filepath.Join(s.dir, name)); err != nil {
		if os.IsNotExist(err) {
			auth.HTTPError(w, http.StatusNotFound, "Datei nicht gefunden")
			return
		}
		log.Error().Err(err).Str("datei", name).Msg("Löschen fehlgeschlagen")
		auth.HTTPError(w, http.StatusInternalServerError, "Löschen fehlgeschlagen")
		return
	}

	s.invalidate()
	w.WriteHeader(http.StatusNoContent)
}

// ohneSchreibfrist hebt das Zeitlimit auf, mit dem der Server sonst jede
// Antwort nach 60 Sekunden abschneidet.
//
// Für eine Anfrage ist die Frist richtig. Für einen Datenstrom ist sie es
// nicht: Ein Hörspielteil von 80 Minuten oder eine Datei von 100 MB über
// schwaches WLAN braucht länger, und der Abbruch käme mitten im Satz. Die
// Frist wird deshalb nur für diese eine Antwort aufgehoben, nicht für den
// ganzen Server.
func ohneSchreibfrist(w http.ResponseWriter) {
	// Ein Server ohne gesetzte Frist kennt den Aufruf nicht — das ist kein
	// Fehler, dann gibt es auch nichts aufzuheben.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
}
