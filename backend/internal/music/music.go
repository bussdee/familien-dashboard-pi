// Package music spielt lokale Audiodateien aus einem eingehängten Ordner ab.
//
// Bewusst nur das: keine Radiosender, keine Podcasts, kein Plex. Ein
// Verzeichnis wird schreibgeschützt eingehängt, woher es kommt, ist dem
// Programm gleich — ein Ordner auf dem Pi, eine SMB-Freigabe vom NAS oder eine
// eingebundene Festplatte sind für uns derselbe Pfad.
//
// Drei Dinge prägen den Aufbau:
//
//   - **Die Sammlung ist gross.** Zwanzigtausend Dateien von einer USB-Platte
//     zu lesen dauert beim ersten Mal Minuten. Der Index liegt deshalb in der
//     Datenbank und wird im Hintergrund aufgebaut; der Start wartet nicht
//     darauf.
//   - **Es sind zu zwei Dritteln Hörspiele.** Geblättert wird nach Ordnern,
//     nicht nach Interpret, und sortiert nach Dateiname — ein Hörspiel läuft
//     von Teil 1 bis Teil 12.
//   - **Die Platte kann weg sein.** Eine externe Platte kann abgemeldet sein.
//     Das Modul muss das aushalten, statt beim Start abzustürzen.
package music

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"github.com/dhowden/tag"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Erlaubte Endungen samt Inhaltstyp. Alles andere im Ordner wird ignoriert —
// Titelbilder, .nfo-Dateien und Notizen liegen dort reichlich herum.
var erlaubteEndungen = map[string]string{
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".m4b":  "audio/mp4", // Hörbuchformat
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/ogg",
	".flac": "audio/flac",
	".wav":  "audio/wav",
	".aac":  "audio/aac",
	".wma":  "audio/x-ms-wma",
}

// Wie oft von selbst nachgesehen wird, ob sich etwas geändert hat.
//
// Hier stand im Plan fsnotify. Dagegen sprechen zwei Dinge: inotify überwacht
// Verzeichnisse einzeln, und bei einer Sammlung mit vielen hundert Ordnern
// stösst das schnell an die Grenze des Systems (max_user_watches). Und eine
// SMB-Freigabe meldet ohnehin nichts. Ein gelegentlicher Durchlauf ist
// langweiliger, aber verlässlich — und wer nicht warten will, drückt in der
// Verwaltung auf "Neu einlesen".
const rescanIntervall = 6 * time.Hour

type Track struct {
	ID   int    `json:"id"`
	Path string `json:"-"`
	// Folder ist der Ordnerpfad relativ zur Wurzel, "" für die Wurzel selbst.
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	TrackNo  int    `json:"track_no"`
	// Duration ist die Länge in Sekunden, 0 wenn unbekannt. Sie steht nur in
	// wenigen Dateien als ID3-Feld (TLEN). Alles andere müsste durch die
	// Tonspur hindurchrechnen — bei 20 000 Dateien auf einer USB-Platte zu
	// teuer. Der Browser kennt die Länge ohnehin, sobald er die Datei öffnet.
	Duration int   `json:"duration"`
	Size     int64 `json:"size"`
}

// Fortschritt beschreibt einen laufenden oder den letzten Durchlauf.
type Fortschritt struct {
	Running   bool      `json:"running"`
	Scanned   int       `json:"scanned"`
	Added     int       `json:"added"`
	Updated   int       `json:"updated"`
	Removed   int       `json:"removed"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// EinstellungsSpeicher ist der Ausschnitt des Speichers, den dieses Modul
// braucht: Welchen Unterordner die Familie gewählt hat.
type EinstellungsSpeicher interface {
	Setting(key string) (string, bool, error)
	SetSetting(key, value string) error
}

// SchluesselUnterordner steht in der settings-Tabelle. Der Wert ist relativ
// zum eingehängten Ordner; leer heisst "alles".
const SchluesselUnterordner = "music.subdir"

type Service struct {
	// mount ist der Ordner, der in den Container eingehängt wurde. Er kommt
	// aus der .env und lässt sich zur Laufzeit nicht ändern — was der
	// Container nicht sieht, kann keine Einstellung herbeizaubern.
	mount string
	db    *sql.DB
	store EinstellungsSpeicher

	mu       sync.RWMutex
	progress Fortschritt
	// gewaehlterOrdner ist der Teil davon, den die Familie hören will. Das ist
	// die Einstellung, die im Adminbereich gesetzt wird.
	gewaehlterOrdner string

	// scanning sorgt dafür, dass nie zwei Durchläufe gleichzeitig laufen. Ein
	// zweiter Aufruf prallt ab, statt sich mit dem ersten zu überschneiden.
	scanning sync.Mutex
	// anstoss weckt den Hintergrundlauf, wenn jemand "Neu einlesen" drückt.
	anstoss chan struct{}
}

func NewService(db *sql.DB, mount string, store EinstellungsSpeicher) *Service {
	svc := &Service{db: db, mount: mount, store: store, anstoss: make(chan struct{}, 1)}
	if store != nil {
		if wert, da, err := store.Setting(SchluesselUnterordner); err == nil && da {
			svc.gewaehlterOrdner = saubererOrdner(wert)
		}
	}
	return svc
}

// wurzel ist der Ordner, aus dem wirklich gelesen wird: der eingehängte Ordner
// plus der im Adminbereich gewählte Unterordner.
func (s *Service) wurzel() string {
	s.mu.RLock()
	sub := s.gewaehlterOrdner
	s.mu.RUnlock()
	if sub == "" {
		return s.mount
	}
	return filepath.Join(s.mount, filepath.FromSlash(sub))
}

// Enabled sagt, ob überhaupt ein Ordner eingerichtet ist.
func (s *Service) Enabled() bool { return strings.TrimSpace(s.mount) != "" }

// Available sagt, ob der Ordner gerade erreichbar ist. Eine abgemeldete
// Festplatte ist kein Fehler, nur ein Zustand — die Oberfläche sagt es dann.
func (s *Service) Available() bool {
	if !s.Enabled() {
		return false
	}
	info, err := os.Stat(s.wurzel())
	return err == nil && info.IsDir()
}

// ------------------------------------------------------------------ Einlesen

func (s *Service) Start(ctx context.Context) {
	if !s.Enabled() {
		log.Info().Msg("Musik: kein Ordner eingerichtet (MUSIC_DIR leer) — Modul ruht")
		return
	}
	if !s.Available() {
		log.Warn().Str("dir", s.wurzel()).
			Msg("Musik: Ordner nicht erreichbar — der Index bleibt, bis die Platte wieder da ist")
	}

	// Der erste Durchlauf läuft sofort, aber im Hintergrund: Der Start des
	// Servers darf nicht auf zwanzigtausend Dateien warten.
	go s.scan()

	ticker := time.NewTicker(rescanIntervall)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scan()
		case <-s.anstoss:
			s.scan()
		}
	}
}

func (s *Service) scan() {
	if !s.scanning.TryLock() {
		return // es läuft schon einer
	}
	defer s.scanning.Unlock()

	s.setProgress(func(p *Fortschritt) {
		*p = Fortschritt{Running: true, StartedAt: time.Now()}
	})

	err := s.durchlauf()

	s.setProgress(func(p *Fortschritt) {
		p.Running = false
		p.EndedAt = time.Now()
		if err != nil {
			p.Error = err.Error()
		}
	})

	if err != nil {
		log.Error().Err(err).Msg("Musik: Einlesen fehlgeschlagen")
		return
	}
	s.mu.RLock()
	p := s.progress
	s.mu.RUnlock()
	log.Info().Int("gefunden", p.Scanned).Int("neu", p.Added).
		Int("geändert", p.Updated).Int("entfernt", p.Removed).
		Dur("dauer", p.EndedAt.Sub(p.StartedAt)).Msg("Musik: Index aktualisiert")
}

func (s *Service) setProgress(f func(*Fortschritt)) {
	s.mu.Lock()
	f(&s.progress)
	s.mu.Unlock()
}

// bekannt ist, was schon im Index steht — Grösse und Änderungszeit reichen, um
// zu entscheiden, ob eine Datei neu gelesen werden muss. Nur dafür wird die
// Datei geöffnet; bei einem zweiten Durchlauf bleibt die Platte still.
type bekannt struct {
	id      int
	size    int64
	modTime time.Time
}

func (s *Service) durchlauf() error {
	if !s.Available() {
		return fmt.Errorf("Ordner %s ist nicht erreichbar", s.wurzel())
	}

	alt, err := s.indexLesen()
	if err != nil {
		return err
	}

	gesehen := make(map[string]bool, len(alt))
	var neu, geaendert, gezaehlt int

	wurzel, err := filepath.EvalSymlinks(s.wurzel())
	if err != nil {
		return fmt.Errorf("Ordner %s: %w", s.wurzel(), err)
	}

	walkErr := filepath.Walk(wurzel, func(pfad string, info os.FileInfo, err error) error {
		if err != nil {
			// Ein einzelner unlesbarer Ordner darf den ganzen Durchlauf nicht
			// abbrechen. Bei 20 000 Dateien ist immer mal eine dabei.
			return nil
		}
		if info.IsDir() {
			// Versteckte Ordner überspringen: .Trash, .Spotlight und was
			// Betriebssysteme sonst noch auf einer Platte hinterlassen.
			if name := info.Name(); name != "." && strings.HasPrefix(name, ".") && pfad != wurzel {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, ok := erlaubteEndungen[strings.ToLower(filepath.Ext(info.Name()))]; !ok {
			return nil
		}

		rel, err := filepath.Rel(wurzel, pfad)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		gesehen[rel] = true
		gezaehlt++

		if vorher, ok := alt[rel]; ok &&
			vorher.size == info.Size() && vorher.modTime.Equal(info.ModTime().UTC().Truncate(time.Second)) {
			return nil // unverändert, Datei bleibt zu
		}

		t := s.lesen(pfad, rel, info)
		war := alt[rel].id
		if err := s.speichern(t, info); err != nil {
			log.Debug().Err(err).Str("datei", rel).Msg("Musik: Titel nicht speicherbar")
			return nil
		}
		if war == 0 {
			neu++
		} else {
			geaendert++
		}

		// Zwischenstand melden, damit die Oberfläche beim ersten Durchlauf
		// nicht minutenlang bei null steht.
		if gezaehlt%200 == 0 {
			zwischenNeu, zwischenGeaendert, zwischenGezaehlt := neu, geaendert, gezaehlt
			s.setProgress(func(p *Fortschritt) {
				p.Scanned, p.Added, p.Updated = zwischenGezaehlt, zwischenNeu, zwischenGeaendert
			})
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	entfernt, err := s.aufraeumen(alt, gesehen)
	if err != nil {
		return err
	}

	s.setProgress(func(p *Fortschritt) {
		p.Scanned, p.Added, p.Updated, p.Removed = gezaehlt, neu, geaendert, entfernt
	})
	return nil
}

func (s *Service) indexLesen() (map[string]bekannt, error) {
	rows, err := s.db.Query("SELECT id, path, size, mod_time FROM music_tracks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alt := map[string]bekannt{}
	for rows.Next() {
		var b bekannt
		var pfad string
		if err := rows.Scan(&b.id, &pfad, &b.size, &b.modTime); err != nil {
			continue
		}
		b.modTime = b.modTime.UTC().Truncate(time.Second)
		alt[pfad] = b
	}
	return alt, rows.Err()
}

// lesen holt die ID3-Felder. Schlägt das fehl — bei einer beschädigten Datei
// oder einer ohne Felder —, bleibt der Dateiname übrig. Das ist wenig, aber
// besser als die Datei ganz wegzulassen.
func (s *Service) lesen(pfad, rel string, info os.FileInfo) Track {
	ordner := filepath.ToSlash(filepath.Dir(rel))
	if ordner == "." {
		ordner = ""
	}
	t := Track{
		Path:     rel,
		Folder:   ordner,
		Filename: info.Name(),
		Size:     info.Size(),
	}

	f, err := os.Open(pfad)
	if err != nil {
		t.Title = ohneEndung(info.Name())
		return t
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Title = ohneEndung(info.Name())
		return t
	}

	t.Title = strings.TrimSpace(m.Title())
	t.Artist = strings.TrimSpace(m.Artist())
	t.Album = strings.TrimSpace(m.Album())
	if t.Title == "" {
		t.Title = ohneEndung(info.Name())
	}
	if nr, _ := m.Track(); nr > 0 {
		t.TrackNo = nr
	}
	t.Duration = laengeAusTags(m)
	return t
}

func ohneEndung(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// laengeAusTags liest TLEN, wenn es da ist. Das Feld steht in Millisekunden
// und fehlt in den meisten Dateien — dann bleibt es bei 0 und der Browser
// sagt die Länge, sobald er die Datei öffnet.
func laengeAusTags(m tag.Metadata) int {
	roh, ok := m.Raw()["TLEN"]
	if !ok {
		return 0
	}
	text, ok := roh.(string)
	if !ok {
		return 0
	}
	ms, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || ms <= 0 {
		return 0
	}
	return ms / 1000
}

func (s *Service) speichern(t Track, info os.FileInfo) error {
	_, err := s.db.Exec(`
		INSERT INTO music_tracks
			(path, folder, filename, title, artist, album, track_no, duration_sec, size, mod_time, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			folder = excluded.folder, filename = excluded.filename,
			title = excluded.title, artist = excluded.artist, album = excluded.album,
			track_no = excluded.track_no, duration_sec = excluded.duration_sec,
			size = excluded.size, mod_time = excluded.mod_time,
			indexed_at = CURRENT_TIMESTAMP`,
		t.Path, t.Folder, t.Filename, t.Title, t.Artist, t.Album,
		t.TrackNo, t.Duration, t.Size, info.ModTime().UTC().Truncate(time.Second))
	return err
}

// aufraeumen entfernt aus dem Index, was auf der Platte nicht mehr liegt.
//
// Wichtig: Das läuft nur nach einem Durchlauf, der die Wurzel wirklich lesen
// konnte. Sonst würde eine abgemeldete Festplatte den ganzen Index löschen —
// und nach dem Wiedereinstecken müssten 20 000 Dateien neu gelesen werden.
func (s *Service) aufraeumen(alt map[string]bekannt, gesehen map[string]bool) (int, error) {
	var weg []int
	for pfad, b := range alt {
		if !gesehen[pfad] {
			weg = append(weg, b.id)
		}
	}
	if len(weg) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range weg {
		if _, err := tx.Exec("DELETE FROM music_tracks WHERE id = ?", id); err != nil {
			return 0, err
		}
	}
	return len(weg), tx.Commit()
}

// ------------------------------------------------------------------ Handlers

// Status ist die eine Auskunft, die die Oberfläche beim Aufbau braucht.
func (s *Service) Status(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	p := s.progress
	s.mu.RUnlock()

	var titel int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM music_tracks").Scan(&titel)

	s.mu.RLock()
	sub := s.gewaehlterOrdner
	s.mu.RUnlock()

	auth.WriteJSON(w, map[string]any{
		"enabled":   s.Enabled(),
		"available": s.Available(),
		"tracks":    titel,
		"progress":  p,
		// Woher die Musik kommt — die Oberfläche zeigt es im Adminbereich.
		"mount":  s.mount,
		"subdir": sub,
	})
}

// Rescan stösst einen Durchlauf an. Nur für Administratoren: Bei einer grossen
// Sammlung ist das minutenlange Arbeit für die Platte.
func (s *Service) Rescan(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		auth.HTTPError(w, http.StatusServiceUnavailable, "Es ist kein Musikordner eingerichtet")
		return
	}
	if !s.Available() {
		auth.HTTPError(w, http.StatusServiceUnavailable,
			"Der Musikordner ist gerade nicht erreichbar")
		return
	}
	select {
	case s.anstoss <- struct{}{}:
	default: // es steht schon einer an
	}
	w.WriteHeader(http.StatusAccepted)
}

type ordnerEintrag struct {
	// Path ist der vollständige Pfad relativ zur Wurzel, Name nur der letzte
	// Bestandteil — die Oberfläche zeigt den Namen und blättert über den Pfad.
	Path   string `json:"path"`
	Name   string `json:"name"`
	Tracks int    `json:"tracks"`
}

// Browse listet einen Ordner: erst die Unterordner, dann die Titel darin.
//
// Sortiert wird nach Dateiname und nicht nach ID3-Titel. Das ist Absicht: Ein
// Hörspiel läuft von Teil 1 bis Teil 12, und diese Reihenfolge steht
// zuverlässig im Dateinamen — im Titelfeld steht sie oft gar nicht.
func (s *Service) Browse(w http.ResponseWriter, r *http.Request) {
	pfad := saubererOrdner(r.URL.Query().Get("path"))

	ordner, err := s.unterordner(pfad)
	if err != nil {
		log.Error().Err(err).Msg("Musik: Ordner nicht lesbar")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	titel, err := s.titelIn(pfad)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	auth.WriteJSON(w, map[string]any{
		"path":    pfad,
		"parent":  elternOrdner(pfad),
		"folders": ordner,
		"tracks":  titel,
	})
}

// saubererOrdner macht aus einer Eingabe von aussen einen harmlosen
// Ordnerpfad. Der Wert landet nur in einer SQL-Abfrage mit Platzhalter und nie
// im Dateisystem — trotzdem fliegen "..", führende Schrägstriche und
// Backslashes raus, damit gar nicht erst die Frage aufkommt.
func saubererOrdner(roh string) string {
	p := strings.ReplaceAll(strings.TrimSpace(roh), "\\", "/")
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return ""
	}
	teile := []string{}
	for _, teil := range strings.Split(p, "/") {
		if teil == "" || teil == "." || teil == ".." {
			continue
		}
		teile = append(teile, teil)
	}
	return strings.Join(teile, "/")
}

func elternOrdner(pfad string) string {
	if pfad == "" {
		return ""
	}
	if i := strings.LastIndex(pfad, "/"); i > 0 {
		return pfad[:i]
	}
	return ""
}

// unterordner findet die direkten Kinder eines Ordners.
//
// Der Index kennt nur vollständige Ordnerpfade, keine Baumstruktur. Bei rund
// tausend Ordnern ist es billiger, die Liste einmal zu holen und in Go zu
// gruppieren, als für jede Ebene eine eigene Abfrage zu bauen.
func (s *Service) unterordner(eltern string) ([]ordnerEintrag, error) {
	var rows *sql.Rows
	var err error
	if eltern == "" {
		rows, err = s.db.Query(
			"SELECT folder, COUNT(*) FROM music_tracks WHERE folder != '' GROUP BY folder")
	} else {
		rows, err = s.db.Query(
			"SELECT folder, COUNT(*) FROM music_tracks WHERE folder LIKE ? ESCAPE '\\' GROUP BY folder",
			likeEscape(eltern)+"/%")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	praefix := ""
	if eltern != "" {
		praefix = eltern + "/"
	}

	zaehler := map[string]int{}
	for rows.Next() {
		var voll string
		var n int
		if err := rows.Scan(&voll, &n); err != nil {
			continue
		}
		rest := strings.TrimPrefix(voll, praefix)
		if rest == "" {
			continue
		}
		// Alles unterhalb zählt zum direkten Kind: Ein Ordner mit
		// Unterordnern soll nicht als leer erscheinen.
		kind := rest
		if i := strings.Index(rest, "/"); i >= 0 {
			kind = rest[:i]
		}
		zaehler[praefix+kind] += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	liste := make([]ordnerEintrag, 0, len(zaehler))
	for voll, n := range zaehler {
		name := voll
		if i := strings.LastIndex(voll, "/"); i >= 0 {
			name = voll[i+1:]
		}
		liste = append(liste, ordnerEintrag{Path: voll, Name: name, Tracks: n})
	}
	sort.Slice(liste, func(i, j int) bool {
		return strings.ToLower(liste[i].Name) < strings.ToLower(liste[j].Name)
	})
	return liste, nil
}

// likeEscape entschärft die Platzhalter von LIKE. Ein Ordner, der "100%
// Kinderlieder" heisst, würde sonst auf alles passen.
func likeEscape(v string) string {
	r := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return r.Replace(v)
}

func (s *Service) titelIn(ordner string) ([]Track, error) {
	rows, err := s.db.Query(`
		SELECT id, folder, filename, title, artist, album, track_no, duration_sec, size
		FROM music_tracks WHERE folder = ? ORDER BY filename COLLATE NOCASE`, ordner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTracks(rows)
}

func scanTracks(rows *sql.Rows) ([]Track, error) {
	titel := []Track{}
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.Folder, &t.Filename, &t.Title, &t.Artist,
			&t.Album, &t.TrackNo, &t.Duration, &t.Size); err != nil {
			return nil, err
		}
		titel = append(titel, t)
	}
	return titel, rows.Err()
}

// Search sucht in Titel, Interpret, Album und Dateiname. Bei 20 000 Dateien
// ist das Blättern allein zu mühsam, wenn man weiss, was man hören will.
func (s *Service) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		auth.WriteJSON(w, map[string]any{"tracks": []Track{}, "query": q})
		return
	}

	muster := "%" + likeEscape(q) + "%"
	rows, err := s.db.Query(`
		SELECT id, folder, filename, title, artist, album, track_no, duration_sec, size
		FROM music_tracks
		WHERE title LIKE ? ESCAPE '\' OR artist LIKE ? ESCAPE '\'
		   OR album LIKE ? ESCAPE '\' OR filename LIKE ? ESCAPE '\'
		ORDER BY folder COLLATE NOCASE, filename COLLATE NOCASE
		LIMIT 200`, muster, muster, muster, muster)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	titel, err := scanTracks(rows)
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, map[string]any{"tracks": titel, "query": q})
}

// Stream liefert eine Datei aus.
//
// Angesprochen wird über die Kennung aus dem Index, nicht über den Pfad. Das
// löst gleich zwei Dinge: Ordner wie "Musik (Kinder)" mit Klammern,
// Leerzeichen und Umlauten brauchen keine Kodierung in der Adresse, und ein
// Pfad von aussen kann gar nicht erst irgendwohin zeigen.
//
// http.ServeContent beantwortet Bereichsanfragen von selbst — Spulen und
// Springen funktionieren dadurch ohne Zutun.
func (s *Service) Stream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var rel string
	err = s.db.QueryRow("SELECT path FROM music_tracks WHERE id = ?", id).Scan(&rel)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	mime, ok := erlaubteEndungen[strings.ToLower(filepath.Ext(rel))]
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	ohneSchreibfrist(w)

	pfad, err := s.aufloesen(rel)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(pfad)
	if err != nil {
		// Die Platte kann abgemeldet sein. 503 statt 404: Die Datei ist nicht
		// weg, sie ist gerade nur nicht da.
		http.Error(w, "Musikordner nicht erreichbar", http.StatusServiceUnavailable)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", mime)
	// Der Browser darf im Speicher halten, was er will, aber nichts davon
	// gehört in den Zwischenspeicher der App — siehe service-worker.ts.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// aufloesen setzt Wurzel und relativen Pfad zusammen und prüft danach noch
// einmal, dass das Ergebnis wirklich innerhalb der Wurzel liegt. Die zweite
// Prüfung ist gegen symbolische Verweise: Eine Verknüpfung im Musikordner, die
// nach /etc zeigt, würde sonst brav ausgeliefert.
func (s *Service) aufloesen(rel string) (string, error) {
	wurzel, err := filepath.EvalSymlinks(s.wurzel())
	if err != nil {
		return "", err
	}
	pfad := filepath.Join(wurzel, filepath.FromSlash(rel))

	echt, err := filepath.EvalSymlinks(pfad)
	if err != nil {
		return "", err
	}
	drin, err := filepath.Rel(wurzel, echt)
	if err != nil || drin == ".." || strings.HasPrefix(drin, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Pfad liegt ausserhalb des Musikordners")
	}
	return echt, nil
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

// -------------------------------------------------------------- Verwaltung

// Was hier NICHT geht, und warum: Der Ordner auf dem Rechner selbst lässt sich
// nicht aus dem Adminbereich wählen. Ein Container sieht nur, was in ihn
// eingehängt wurde — was nicht eingehängt ist, existiert für ihn nicht, und
// keine Einstellung in einer Weboberfläche ändert daran etwas. Welcher Ordner
// eingehängt wird, steht deshalb weiterhin in der .env (`MUSIC_HOST_DIR`).
//
// Was hier geht: den Teil davon wählen, der gehört werden soll. Wer die ganze
// Platte einhängt, aber nur "AUDIO/Hörspiele" im Dashboard haben will, stellt
// das hier ein — ohne .env, ohne Neustart.

type verzeichnisEintrag struct {
	Path string `json:"path"`
	Name string `json:"name"`
	// HatAudio sagt, ob direkt in diesem Ordner Audiodateien liegen. Beim
	// Auswählen ist das die Frage, die man hat.
	HatAudio bool `json:"has_audio"`
	// HatUnterordner entscheidet, ob es sich lohnt, tiefer zu tippen.
	HatUnterordner bool `json:"has_subfolders"`
}

// AdminFolders zeigt die echten Verzeichnisse im eingehängten Ordner — nicht
// den Index, sondern das Dateisystem. Beim Einrichten ist der Index ja noch
// leer.
func (s *Service) AdminFolders(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		auth.HTTPError(w, http.StatusServiceUnavailable,
			"Es ist kein Ordner eingehängt. Dafür MUSIC_HOST_DIR in der .env setzen.")
		return
	}

	rel := saubererOrdner(r.URL.Query().Get("path"))
	pfad, err := s.imMount(rel)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ordner nicht gefunden")
		return
	}

	eintraege, err := os.ReadDir(pfad)
	if err != nil {
		auth.HTTPError(w, http.StatusServiceUnavailable,
			"Ordner nicht lesbar. Ist die Festplatte angeschlossen?")
		return
	}

	ordner := []verzeichnisEintrag{}
	var audioHier bool
	for _, e := range eintraege {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			kind := name
			if rel != "" {
				kind = rel + "/" + name
			}
			audio, unter := inhaltPruefen(filepath.Join(pfad, name))
			ordner = append(ordner, verzeichnisEintrag{
				Path: kind, Name: name, HatAudio: audio, HatUnterordner: unter,
			})
			continue
		}
		if _, ok := erlaubteEndungen[strings.ToLower(filepath.Ext(name))]; ok {
			audioHier = true
		}
	}
	sort.Slice(ordner, func(i, j int) bool {
		return strings.ToLower(ordner[i].Name) < strings.ToLower(ordner[j].Name)
	})

	s.mu.RLock()
	gewaehlt := s.gewaehlterOrdner
	s.mu.RUnlock()

	auth.WriteJSON(w, map[string]any{
		"path":      rel,
		"parent":    elternOrdner(rel),
		"folders":   ordner,
		"has_audio": audioHier,
		"selected":  gewaehlt,
		// Der eingehängte Pfad steht mit in der Antwort, damit die Oberfläche
		// erklären kann, worauf sich das alles bezieht.
		"mount": s.mount,
	})
}

// inhaltPruefen sieht einmal flach in einen Ordner. Bewusst nicht rekursiv:
// Beim Blättern durch eine grosse Sammlung würde das die Platte für jede
// Ansicht komplett durchlaufen.
func inhaltPruefen(pfad string) (audio bool, unterordner bool) {
	eintraege, err := os.ReadDir(pfad)
	if err != nil {
		return false, false
	}
	for _, e := range eintraege {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.IsDir() {
			unterordner = true
			continue
		}
		if _, ok := erlaubteEndungen[strings.ToLower(filepath.Ext(e.Name()))]; ok {
			audio = true
		}
		if audio && unterordner {
			return true, true
		}
	}
	return audio, unterordner
}

// imMount setzt einen relativen Pfad an den eingehängten Ordner an und prüft
// danach, dass das Ergebnis wirklich darin liegt. saubererOrdner allein
// genügt nicht: Ein symbolischer Verweis im Ordner könnte hinausführen.
func (s *Service) imMount(rel string) (string, error) {
	wurzel, err := filepath.EvalSymlinks(s.mount)
	if err != nil {
		return "", err
	}
	if rel == "" {
		return wurzel, nil
	}

	echt, err := filepath.EvalSymlinks(filepath.Join(wurzel, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	drin, err := filepath.Rel(wurzel, echt)
	if err != nil || drin == ".." || strings.HasPrefix(drin, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Pfad liegt ausserhalb des eingehängten Ordners")
	}
	return echt, nil
}

// SetDir wählt den Unterordner, aus dem gehört wird.
//
// Der Index wird dabei geleert. Er muss es: Die Pfade darin sind relativ zur
// alten Wurzel und würden nach dem Wechsel ins Leere zeigen.
func (s *Service) SetDir(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		auth.HTTPError(w, http.StatusServiceUnavailable,
			"Es ist kein Ordner eingehängt. Dafür MUSIC_HOST_DIR in der .env setzen.")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	rel := saubererOrdner(req.Path)
	ziel, err := s.imMount(rel)
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Dieser Ordner ist nicht erreichbar")
		return
	}
	info, err := os.Stat(ziel)
	if err != nil || !info.IsDir() {
		auth.HTTPError(w, http.StatusBadRequest, "Dieser Ordner ist nicht erreichbar")
		return
	}

	if s.store != nil {
		if err := s.store.SetSetting(SchluesselUnterordner, rel); err != nil {
			log.Error().Err(err).Msg("Musik: Ordner konnte nicht gespeichert werden")
			auth.HTTPError(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
			return
		}
	}

	s.mu.Lock()
	gewechselt := s.gewaehlterOrdner != rel
	s.gewaehlterOrdner = rel
	s.mu.Unlock()

	if gewechselt {
		if _, err := s.db.Exec("DELETE FROM music_tracks"); err != nil {
			log.Error().Err(err).Msg("Musik: alter Index nicht löschbar")
		}
	}

	select {
	case s.anstoss <- struct{}{}:
	default:
	}

	log.Info().Str("unterordner", rel).Msg("Musik: Ordner gewechselt, lese neu ein")
	auth.WriteJSON(w, map[string]any{"path": rel, "mount": s.mount})
}
