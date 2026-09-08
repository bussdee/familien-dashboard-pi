package photos

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
	// A phone photo is a few MB; 25 MB leaves room for a DSLR JPEG without
	// letting anyone fill the Pi's card in one request.
	maxPhotoBytes = 25 << 20
	maxUploadSize = 120 << 20 // whole request, i.e. a batch of photos
)

var allowedExt = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".avif": "image/avif",
}

type Photo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type Service struct {
	dir string

	mu     sync.RWMutex
	cache  []Photo
	cached time.Time
}

func NewService(dir string) *Service {
	return &Service{dir: dir}
}

// List returns the photo filenames; the frontend builds /api/photos/<name> URLs.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	photos, err := s.list()
	if err != nil {
		log.Debug().Err(err).Str("dir", s.dir).Msg("Photo dir not readable")
		photos = []Photo{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"photos": photos,
		"count":  len(photos),
	})
}

// invalidate drops the listing cache after an upload or delete, so the widget
// sees the change immediately instead of up to 30 s later.
func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.cached = time.Time{}
	s.mu.Unlock()
}

func (s *Service) list() ([]Photo, error) {
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

	photos := []Photo{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := allowedExt[strings.ToLower(filepath.Ext(e.Name()))]; !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		photos = append(photos, Photo{Name: e.Name(), Size: info.Size(), Modified: info.ModTime()})
	}
	sort.Slice(photos, func(i, j int) bool { return photos[i].Name < photos[j].Name })

	s.mu.Lock()
	s.cache, s.cached = photos, time.Now()
	s.mu.Unlock()
	return photos, nil
}

// Serve streams one photo. The name is reduced to its base so "../" can never
// escape the photo directory.
func (s *Service) Serve(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(filepath.Clean("/" + chi.URLParam(r, "name")))
	if name == "." || name == "/" || name == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	mime, ok := allowedExt[strings.ToLower(filepath.Ext(name))]
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	path := filepath.Join(s.dir, name)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, name, info.ModTime(), f)
}

// ------------------------------------------------------------------- Uploads

var unsafeName = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)

// storageName keeps a recognisable part of the original filename but strips
// anything that could escape the photo directory, and appends a timestamp so
// two holiday photos called IMG_1234.jpg can coexist.
func storageName(original, ext string) string {
	base := strings.TrimSuffix(filepath.Base(original), filepath.Ext(original))
	base = strings.Trim(unsafeName.ReplaceAllString(base, "_"), "_.")
	if len([]rune(base)) > 48 {
		base = string([]rune(base)[:48])
	}
	if base == "" {
		base = "foto"
	}
	return fmt.Sprintf("%s_%s%s", base, time.Now().Format("20060102-150405.000"), ext)
}

type uploadResult struct {
	Uploaded []string          `json:"uploaded"`
	Skipped  map[string]string `json:"skipped,omitempty"`
	Count    int               `json:"count"`
}

// Upload accepts one or more images from the "photos" form field.
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		log.Error().Err(err).Msg("Cannot create photo dir")
		auth.HTTPError(w, http.StatusInternalServerError, "Foto-Ordner nicht beschreibbar")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		auth.HTTPError(w, http.StatusRequestEntityTooLarge,
			"Upload zu groß (maximal 120 MB pro Vorgang)")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	files := r.MultipartForm.File["photos"]
	if len(files) == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Keine Datei ausgewählt")
		return
	}

	result := uploadResult{Uploaded: []string{}, Skipped: map[string]string{}}
	for _, header := range files {
		name, err := s.saveUpload(header)
		if err != nil {
			result.Skipped[header.Filename] = err.Error()
			continue
		}
		result.Uploaded = append(result.Uploaded, name)
	}
	result.Count = len(result.Uploaded)

	if len(result.Skipped) == 0 {
		result.Skipped = nil
	}
	if result.Count == 0 {
		auth.HTTPError(w, http.StatusUnsupportedMediaType,
			"Keine Datei konnte übernommen werden - erlaubt sind JPG, PNG, WEBP, GIF und AVIF")
		return
	}

	s.invalidate()
	log.Info().Int("count", result.Count).Msg("Photos uploaded")
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, result)
}

func (s *Service) saveUpload(header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	declared, ok := allowedExt[ext]
	if !ok {
		return "", fmt.Errorf("Dateityp nicht unterstützt")
	}
	if header.Size > maxPhotoBytes {
		return "", fmt.Errorf("Datei zu groß (maximal 25 MB)")
	}

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("Datei nicht lesbar")
	}
	defer src.Close()

	// Trust the bytes, not the extension: sniff the real type before writing.
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	sniffed := http.DetectContentType(head[:n])
	if !strings.HasPrefix(sniffed, "image/") {
		return "", fmt.Errorf("Datei ist kein Bild")
	}
	// DetectContentType does not know AVIF/WEBP reliably; the extension already
	// limited us to image formats, so only a clear mismatch is rejected.
	if sniffed != declared && sniffed != "application/octet-stream" &&
		!strings.HasPrefix(sniffed, "image/") {
		return "", fmt.Errorf("Inhalt passt nicht zur Dateiendung")
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("Datei nicht lesbar")
	}

	name := storageName(header.Filename, ext)
	target := filepath.Join(s.dir, name)

	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("Speichern fehlgeschlagen")
	}

	written, err := io.Copy(dst, io.LimitReader(src, maxPhotoBytes+1))
	closeErr := dst.Close()
	if err != nil || closeErr != nil || written > maxPhotoBytes {
		_ = os.Remove(target)
		if written > maxPhotoBytes {
			return "", fmt.Errorf("Datei zu groß (maximal 25 MB)")
		}
		return "", fmt.Errorf("Speichern fehlgeschlagen")
	}

	return name, nil
}

// Delete removes one photo. The name is reduced to its base first, so a crafted
// name cannot reach outside the photo directory.
func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(filepath.Clean("/" + chi.URLParam(r, "name")))
	if name == "." || name == "/" || name == "" {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültiger Dateiname")
		return
	}
	if _, ok := allowedExt[strings.ToLower(filepath.Ext(name))]; !ok {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültiger Dateiname")
		return
	}

	if err := os.Remove(filepath.Join(s.dir, name)); err != nil {
		if os.IsNotExist(err) {
			auth.HTTPError(w, http.StatusNotFound, "Foto nicht gefunden")
			return
		}
		log.Error().Err(err).Str("photo", name).Msg("Failed to delete photo")
		auth.HTTPError(w, http.StatusInternalServerError, "Löschen fehlgeschlagen")
		return
	}

	s.invalidate()
	w.WriteHeader(http.StatusNoContent)
}
