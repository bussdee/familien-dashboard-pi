package rezepte

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

const beispielSeite = `<!doctype html><html><head><title>x</title>
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[
 {"@type":"WebSite","name":"Kochseite"},
 {"@type":["Recipe"],"name":"Spaghetti &amp; Sauce","image":["/bild.png"],
  "recipeYield":["4","4 Portionen"],"prepTime":"PT15M","cookTime":"PT1H30M",
  "recipeCategory":["Hauptgericht"],"description":"<p>Schnell &amp; gut</p>",
  "recipeIngredient":["500 g Spaghetti","<b>2</b> Dosen Tomaten","  ", "Salz"],
  "recipeInstructions":[{"@type":"HowToStep","text":"Wasser kochen."},
   {"@type":"HowToSection","itemListElement":[{"@type":"HowToStep","text":"Nudeln<br>rein."}]}]}
]}</script></head><body></body></html>`

func TestLesenFindetRezeptImGraph(t *testing.T) {
	g, err := Lesen([]byte(beispielSeite), "https://koch.example/rezept/1")
	if err != nil {
		t.Fatal(err)
	}
	if g.Titel != "Spaghetti & Sauce" {
		t.Errorf("Titel: %q", g.Titel)
	}
	if g.Portionen != "4 Portionen" {
		t.Errorf("Portionen: %q", g.Portionen)
	}
	if g.Vorb != 15 || g.Kochen != 90 || g.Gesamt != 105 {
		t.Errorf("Zeiten: %d %d %d", g.Vorb, g.Kochen, g.Gesamt)
	}
	if g.Bild != "https://koch.example/bild.png" {
		t.Errorf("Bild: %q", g.Bild)
	}
	if strings.Join(g.Zutaten, "|") != "500 g Spaghetti|2 Dosen Tomaten|Salz" {
		t.Errorf("Zutaten: %q", g.Zutaten)
	}
	if strings.Join(g.Schritte, "|") != "Wasser kochen.|Nudeln|rein." {
		t.Errorf("Schritte: %q", g.Schritte)
	}
	if g.Notiz != "Schnell & gut" || g.Kategorie != "Hauptgericht" {
		t.Errorf("Notiz/Kategorie: %q %q", g.Notiz, g.Kategorie)
	}
}

func TestLesenOhneRezept(t *testing.T) {
	for name, seite := range map[string]string{
		"leer":        "",
		"kein JSON":   `<script type="application/ld+json">{kaputt</script>`,
		"anderer Typ": `<script type="application/ld+json">{"@type":"Article","name":"Hallo"}</script>`,
		"ohne Inhalt": `<script type="application/ld+json">{"@type":"Recipe","name":"Nur Titel"}</script>`,
	} {
		if _, err := Lesen([]byte(seite), "https://x.example/"); err != errKeinRezept {
			t.Errorf("%s: erwartet errKeinRezept, war %v", name, err)
		}
	}
}

func TestMinuten(t *testing.T) {
	for in, want := range map[string]int{
		"PT30M": 30, "PT1H": 60, "PT1H30M": 90, "P0DT0H20M": 20, "P1D": 1440,
		"pt45m": 45, "": 0, "30 Minuten": 0, "PT": 0, "P999D": 0,
	} {
		if got := minuten(in); got != want {
			t.Errorf("%q: erwartet %d, war %d", in, want, got)
		}
	}
}

func TestBereinigeEntferntHTMLUndSteuerzeichen(t *testing.T) {
	got := bereinige("<script>alert(1)</script>Hallo\x00\n  Welt &lt;b&gt;", 100)
	if strings.Contains(got, "<script") || strings.Contains(got, "\x00") {
		t.Errorf("nicht bereinigt: %q", got)
	}
	if got != "alert(1)Hallo Welt <b>" {
		t.Errorf("unerwartet: %q", got)
	}
	if len([]rune(bereinige(strings.Repeat("ä", 500), 10))) != 10 {
		t.Error("Länge nicht begrenzt")
	}
}

// ---------------------------------------------------------------- Dienst

type einkaufAttrappe struct{ namen []string }

func (e *einkaufAttrappe) AddItems(n []string, _ *int) (int, error) {
	e.namen = append(e.namen, n...)
	return len(n), nil
}

type schalter bool

func (s schalter) An(string) bool { return bool(s) }

func testService(t *testing.T, an bool) (*Service, *einkaufAttrappe) {
	t.Helper()
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	ek := &einkaufAttrappe{}
	s := NewService(st.DB(), filepath.Join(t.TempDir(), "recipes"), ek, schalter(an))
	// Ohne Sperre, damit der Test gegen 127.0.0.1 laufen kann; die Sperre
	// selbst prüft das Paket abruf.
	s.clientFabrik = func() *http.Client { return http.DefaultClient }
	return s, ek
}

func aufruf(h http.HandlerFunc, body, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r = auth.AlsPerson(r, 1, "admin", false)
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	rec := httptest.NewRecorder()
	h(rec, r)
	return rec
}

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func rezeptServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rezept", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(beispielSeite))
	})
	mux.HandleFunc("/bild.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	})
	mux.HandleFunc("/pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF"))
	})
	mux.HandleFunc("/gesperrt", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/leer", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>kein Rezept</html>"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestImportLebenslauf(t *testing.T) {
	s, ek := testService(t, true)
	srv := rezeptServer(t)

	rec := aufruf(s.Import, `{"url":"`+srv.URL+`/rezept"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("Import: %d %s", rec.Code, rec.Body)
	}
	var rez Rezept
	_ = json.Unmarshal(rec.Body.Bytes(), &rez)
	if rez.Title != "Spaghetti & Sauce" || len(rez.Zutaten) != 3 || len(rez.Schritte) != 3 {
		t.Fatalf("Rezept: %+v", rez)
	}
	if !rez.HasImage {
		t.Error("das Bild sollte gespeichert sein")
	}
	if _, err := os.Stat(filepath.Join(s.dir, "1.png")); err != nil {
		t.Errorf("Bilddatei fehlt: %v", err)
	}

	// Dasselbe Rezept kein zweites Mal.
	if rec := aufruf(s.Import, `{"url":"`+srv.URL+`/rezept"}`, ""); rec.Code != http.StatusConflict {
		t.Errorf("zweiter Import: erwartet 409, war %d", rec.Code)
	}

	// Bild ausliefern.
	rec = aufruf(s.Image, "", "1")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Bild: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Zutaten auf die Einkaufsliste.
	rec = aufruf(s.ToShopping, "", "1")
	if rec.Code != http.StatusOK || len(ek.namen) != 3 {
		t.Errorf("Einkauf: %d %v", rec.Code, ek.namen)
	}

	// Liste und Suche.
	rec = aufruf(s.List, "", "")
	var liste []Rezept
	_ = json.Unmarshal(rec.Body.Bytes(), &liste)
	if len(liste) != 1 || len(liste[0].Schritte) != 0 {
		t.Errorf("Liste: %+v", liste)
	}
	r := httptest.NewRequest(http.MethodGet, "/?q=tomaten", nil)
	w := httptest.NewRecorder()
	s.List(w, auth.AlsPerson(r, 1, "admin", false))
	_ = json.Unmarshal(w.Body.Bytes(), &liste)
	if len(liste) != 1 {
		t.Errorf("Suche nach Zutat: %d Treffer", len(liste))
	}

	// Löschen räumt das Bild auf.
	if rec := aufruf(s.Delete, "", "1"); rec.Code != http.StatusNoContent {
		t.Fatalf("Löschen: %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "1.png")); !os.IsNotExist(err) {
		t.Error("Bilddatei sollte gelöscht sein")
	}
}

func TestImportFehlerfaelle(t *testing.T) {
	s, _ := testService(t, true)
	srv := rezeptServer(t)

	for name, c := range map[string]struct {
		body string
		code int
	}{
		"keine Seite mit Rezept": {`{"url":"` + srv.URL + `/leer"}`, http.StatusUnprocessableEntity},
		"kein Text":              {`{"url":"` + srv.URL + `/pdf"}`, http.StatusBadGateway},
		"Seite sperrt Roboter":   {`{"url":"` + srv.URL + `/gesperrt"}`, http.StatusBadGateway},
		"file://":                {`{"url":"file:///etc/passwd"}`, http.StatusBadRequest},
		"Zugangsdaten":           {`{"url":"https://u:p@example.com/"}`, http.StatusBadRequest},
		"leer":                   {`{"url":""}`, http.StatusBadRequest},
		"kein JSON":              {`nein`, http.StatusBadRequest},
	} {
		if rec := aufruf(s.Import, c.body, ""); rec.Code != c.code {
			t.Errorf("%s: erwartet %d, war %d (%s)", name, c.code, rec.Code, rec.Body)
		}
	}
}

// Der echte Client darf keinen Server im Heimnetz anwählen — auch nicht über
// den Rezept-Import.
func TestImportSperrtHeimnetz(t *testing.T) {
	s, _ := testService(t, true)
	s.clientFabrik = nil
	srv := rezeptServer(t) // läuft auf 127.0.0.1
	rec := aufruf(s.Import, `{"url":"`+srv.URL+`/rezept"}`, "")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "Heimnetz") {
		t.Fatalf("erwartet gesperrt, war %d %s", rec.Code, rec.Body)
	}
}

func TestVonHandUndAusgeschaltet(t *testing.T) {
	s, _ := testService(t, true)
	if rec := aufruf(s.Create, `{"title":"  "}`, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("ohne Titel: %d", rec.Code)
	}
	rec := aufruf(s.Create, `{"title":"Pfannkuchen","ingredients":["Mehl","Milch"],"steps":["Rühren"],"prep_min":-5}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("Anlegen: %d %s", rec.Code, rec.Body)
	}
	var rez Rezept
	_ = json.Unmarshal(rec.Body.Bytes(), &rez)
	if rez.PrepMin != 0 {
		t.Errorf("negative Zeit nicht abgefangen: %d", rez.PrepMin)
	}
	if rec := aufruf(s.Update, `{"title":"Pfannkuchen süß","ingredients":["Mehl"]}`, "1"); rec.Code != http.StatusOK {
		t.Errorf("Ändern: %d", rec.Code)
	}
	if rec := aufruf(s.Update, `{"title":"X"}`, "77"); rec.Code != http.StatusNotFound {
		t.Errorf("unbekannt: %d", rec.Code)
	}
	if rec := aufruf(s.Image, "", "1"); rec.Code != http.StatusNotFound {
		t.Errorf("ohne Bild: %d", rec.Code)
	}

	aus, _ := testService(t, false)
	for name, h := range map[string]http.HandlerFunc{"Liste": aus.List, "Import": aus.Import, "Anlegen": aus.Create} {
		if rec := aufruf(h, `{}`, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s bei ausgeschaltetem Rezeptbuch: %d", name, rec.Code)
		}
	}
}
