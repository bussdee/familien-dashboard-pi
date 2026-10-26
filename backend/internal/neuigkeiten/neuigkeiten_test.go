package neuigkeiten

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/store"
	"github.com/go-chi/chi/v5"
)

const rssBeispiel = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel>
<title>Kinder &amp; Co</title>
<item><title>Erster &lt;b&gt;Artikel&lt;/b&gt;</title><link>https://zeitung.example/a1</link>
<guid>a1</guid><description>&lt;p&gt;Hallo &lt;script&gt;alert(1)&lt;/script&gt; Welt&lt;/p&gt;</description>
<pubDate>Mon, 26 Oct 2026 08:00:00 +0100</pubDate></item>
<item><title>Gefährlicher Link</title><link>javascript:alert(1)</link><guid>a2</guid>
<pubDate>Sun, 25 Oct 2026 08:00:00 GMT</pubDate></item>
<item><link>https://zeitung.example/a3</link><guid>a3</guid></item>
<item><description>nichts</description></item>
</channel></rss>`

const atomBeispiel = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Schule</title>
<entry><title>Elternabend</title><id>urn:1</id>
<link rel="self" href="https://schule.example/self"/><link rel="alternate" href="https://schule.example/e1"/>
<updated>2026-10-20T10:00:00Z</updated><summary>Am Donnerstag</summary></entry></feed>`

func TestLesenRSS(t *testing.T) {
	g, err := Lesen([]byte(rssBeispiel))
	if err != nil {
		t.Fatal(err)
	}
	if g.Titel != "Kinder & Co" {
		t.Errorf("Titel: %q", g.Titel)
	}
	if len(g.Eintraege) != 3 {
		t.Fatalf("erwartet 3 Meldungen (die ohne Titel und Link fällt weg), war %d", len(g.Eintraege))
	}
	e := g.Eintraege[0]
	if e.Titel != "Erster Artikel" || strings.Contains(e.Text, "<") || e.Text != "Hallo alert(1) Welt" {
		t.Errorf("nicht bereinigt: %+v", e)
	}
	if e.Link != "https://zeitung.example/a1" {
		t.Errorf("Link: %q", e.Link)
	}
	if g.Eintraege[1].Link != "" {
		t.Errorf("javascript:-Link muss wegfallen, war %q", g.Eintraege[1].Link)
	}
	if g.Eintraege[2].Titel != "https://zeitung.example/a3" {
		t.Errorf("ohne Titel gilt der Link: %q", g.Eintraege[2].Titel)
	}
}

func TestLesenAtomUndLatin1(t *testing.T) {
	g, err := Lesen([]byte(atomBeispiel))
	if err != nil || len(g.Eintraege) != 1 || g.Eintraege[0].Link != "https://schule.example/e1" {
		t.Fatalf("Atom: %v %+v", err, g)
	}
	latin := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><rss><channel><title>Gr\xfc\xdfe</title>" +
		"<item><title>K\xe4se</title><guid>1</guid></item></channel></rss>"
	g, err = Lesen([]byte(latin))
	if err != nil || g.Titel != "Grüße" || g.Eintraege[0].Titel != "Käse" {
		t.Errorf("Latin-1: %v %+v", err, g)
	}
	for _, kaputt := range []string{"", "nur Text", "<html><body>Hallo</body></html>", "<rss><channel>"} {
		if _, err := Lesen([]byte(kaputt)); err == nil && kaputt != "<rss><channel>" {
			t.Errorf("%q sollte kein Feed sein", kaputt)
		}
	}
}

func TestFeedInSeite(t *testing.T) {
	seite := `<html><head><link rel="stylesheet" href="/s.css">
	<link rel="alternate" type="application/rss+xml" title="Feed" href="/feed.xml"></head></html>`
	if got := FeedInSeite([]byte(seite)); got != "/feed.xml" {
		t.Errorf("erwartet /feed.xml, war %q", got)
	}
	if got := FeedInSeite([]byte("<html></html>")); got != "" {
		t.Errorf("kein Feed: %q", got)
	}
}

// ---------------------------------------------------------------- Dienst

type schalter map[string]bool

func (s schalter) An(k string) bool { return s[k] }

func testService(t *testing.T, sw schalter) *Service {
	t.Helper()
	st, err := store.NewStore(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	s := NewService(st.DB(), sw)
	s.clientFabrik = func() *http.Client { return http.DefaultClient }
	return s
}

func aufruf(h http.HandlerFunc, wer int, rolle string, geraet bool, body, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/?limit=10", strings.NewReader(body))
	if wer > 0 || geraet {
		r = auth.AlsPerson(r, wer, rolle, geraet)
	}
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	rec := httptest.NewRecorder()
	h(rec, r)
	return rec
}

func feedServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssBeispiel))
	})
	mux.HandleFunc("/atom", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(atomBeispiel))
	})
	mux.HandleFunc("/seite", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><link rel="alternate" type="application/atom+xml" href="/atom"></head></html>`))
	})
	mux.HandleFunc("/nichts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>Hallo</html>"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

const (
	papa = 1
	mama = 2
	kind = 3
)

var beide = schalter{"modul.neuigkeiten": true, "modul.neuigkeiten_persoenlich": true}

func TestFamilienFeedLebenslauf(t *testing.T) {
	s := testService(t, beide)
	srv := feedServer(t)

	if rec := aufruf(s.CreateFamily, papa, "admin", false, `{"url":"`+srv.URL+`/feed.xml"}`, ""); rec.Code != http.StatusCreated {
		t.Fatalf("anlegen: %d %s", rec.Code, rec.Body)
	}
	if rec := aufruf(s.CreateFamily, papa, "admin", false, `{"url":"`+srv.URL+`/feed.xml"}`, ""); rec.Code != http.StatusConflict {
		t.Errorf("doppelt: %d", rec.Code)
	}

	// Alle sehen den Familien-Feed — auch ein Kind und das Wandgerät.
	for name, c := range map[string]struct {
		wer    int
		rolle  string
		geraet bool
	}{"kind": {kind, "member", false}, "wand": {0, "device", true}} {
		rec := aufruf(s.Items, c.wer, c.rolle, c.geraet, "", "")
		var items []Meldung
		_ = json.Unmarshal(rec.Body.Bytes(), &items)
		if len(items) != 3 || items[0].FeedTitle != "Kinder & Co" {
			t.Errorf("%s sieht %+v", name, items)
		}
	}

	// Ein Kind darf den Familien-Feed nicht über den persönlichen Weg löschen.
	if rec := aufruf(s.DeletePersonal, kind, "member", false, "", "1"); rec.Code != http.StatusNotFound {
		t.Errorf("Kind löscht Familien-Feed: %d", rec.Code)
	}
	if rec := aufruf(s.DeleteFamily, papa, "admin", false, "", "1"); rec.Code != http.StatusNoContent {
		t.Errorf("Eltern löschen: %d", rec.Code)
	}
	rec := aufruf(s.Items, papa, "admin", false, "", "")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("Meldungen müssen mit dem Feed verschwinden: %s", rec.Body)
	}
}

func TestPersoenlicheFeedsSindPrivat(t *testing.T) {
	s := testService(t, beide)
	srv := feedServer(t)

	if rec := aufruf(s.CreatePersonal, kind, "member", false, `{"url":"`+srv.URL+`/atom","title":"Meine Schule"}`, ""); rec.Code != http.StatusCreated {
		t.Fatalf("anlegen: %d %s", rec.Code, rec.Body)
	}
	titel := func(wer int, rolle string, geraet bool) int {
		rec := aufruf(s.ListFeeds, wer, rolle, geraet, "", "")
		var l []Feed
		_ = json.Unmarshal(rec.Body.Bytes(), &l)
		return len(l)
	}
	if n := titel(kind, "member", false); n != 1 {
		t.Errorf("das Kind sieht %d Feeds", n)
	}
	if n := titel(papa, "admin", false); n != 0 {
		t.Errorf("Papa sieht den Feed des Kindes (%d)", n)
	}
	if n := titel(0, "device", true); n != 0 {
		t.Errorf("das Wandgerät sieht persönliche Feeds (%d)", n)
	}
	if rec := aufruf(s.Items, papa, "admin", false, "", ""); strings.Contains(rec.Body.String(), "Elternabend") {
		t.Error("Meldungen des Kindes bei Papa")
	}

	// Löschen: nur die eigenen.
	if rec := aufruf(s.DeletePersonal, mama, "admin", false, "", "1"); rec.Code != http.StatusNotFound {
		t.Errorf("fremder Feed: %d", rec.Code)
	}
	if rec := aufruf(s.DeletePersonal, kind, "member", false, "", "1"); rec.Code != http.StatusNoContent {
		t.Errorf("eigener Feed: %d", rec.Code)
	}
	// Eltern-Weg zeigt keinen persönlichen Feed an.
	_ = aufruf(s.CreatePersonal, kind, "member", false, `{"url":"`+srv.URL+`/atom"}`, "")
	if rec := aufruf(s.DeleteFamily, papa, "admin", false, "", "2"); rec.Code != http.StatusNotFound {
		t.Errorf("Eltern-Weg darf keinen persönlichen Feed löschen: %d", rec.Code)
	}
}

func TestSchalterUndFehler(t *testing.T) {
	srv := feedServer(t)

	aus := testService(t, schalter{})
	if rec := aufruf(aus.CreateFamily, papa, "admin", false, `{"url":"`+srv.URL+`/feed.xml"}`, ""); rec.Code != http.StatusForbidden {
		t.Errorf("ausgeschaltet: %d", rec.Code)
	}
	if rec := aufruf(aus.Items, papa, "admin", false, "", ""); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("ausgeschaltet liefert %s", rec.Body)
	}

	nurFamilie := testService(t, schalter{"modul.neuigkeiten": true})
	if rec := aufruf(nurFamilie.CreatePersonal, kind, "member", false, `{"url":"`+srv.URL+`/feed.xml"}`, ""); rec.Code != http.StatusForbidden {
		t.Errorf("persönlich ausgeschaltet: %d", rec.Code)
	}

	s := testService(t, beide)
	for name, c := range map[string]struct {
		body string
		code int
	}{
		"kein Feed":       {`{"url":"` + srv.URL + `/nichts"}`, http.StatusBadRequest},
		"file://":         {`{"url":"file:///etc/passwd"}`, http.StatusBadRequest},
		"leer":            {`{"url":""}`, http.StatusBadRequest},
		"kein JSON":       {`ja`, http.StatusBadRequest},
		"Webseite → Feed": {`{"url":"` + srv.URL + `/seite"}`, http.StatusCreated},
	} {
		if rec := aufruf(s.CreateFamily, papa, "admin", false, c.body, ""); rec.Code != c.code {
			t.Errorf("%s: erwartet %d, war %d (%s)", name, c.code, rec.Code, rec.Body)
		}
	}
}

func TestGrenzenJeListe(t *testing.T) {
	s := testService(t, beide)
	for i := 0; i < maxPersoenlich; i++ {
		if _, err := s.db.Exec(`INSERT INTO rss_feeds (title, url, owner_id) VALUES ('x', ?, 3)`, "https://x.example/"+strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
	}
	srv := feedServer(t)
	if rec := aufruf(s.CreatePersonal, kind, "member", false, `{"url":"`+srv.URL+`/feed.xml"}`, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("zu viele: %d %s", rec.Code, rec.Body)
	}
	// Eine andere Person hat ihr eigenes Kontingent.
	if rec := aufruf(s.CreatePersonal, mama, "admin", false, `{"url":"`+srv.URL+`/feed.xml"}`, ""); rec.Code != http.StatusCreated {
		t.Errorf("andere Person: %d %s", rec.Code, rec.Body)
	}
}

// Der echte Client darf keinen Feed im Heimnetz abrufen.
func TestFeedImHeimnetzGesperrt(t *testing.T) {
	s := testService(t, beide)
	s.clientFabrik = nil
	srv := feedServer(t)
	rec := aufruf(s.CreateFamily, papa, "admin", false, `{"url":"`+srv.URL+`/feed.xml"}`, "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Heimnetz") {
		t.Fatalf("erwartet gesperrt, war %d %s", rec.Code, rec.Body)
	}
}

func TestAktualisierenBehaeltAlteMeldungenBeiFehler(t *testing.T) {
	s := testService(t, beide)
	srv := feedServer(t)
	_ = aufruf(s.CreateFamily, papa, "admin", false, `{"url":"`+srv.URL+`/feed.xml"}`, "")

	// Der Feed antwortet nicht mehr.
	if _, err := s.db.Exec(`UPDATE rss_feeds SET url = ? WHERE id = 1`, srv.URL+"/gibts-nicht"); err != nil {
		t.Fatal(err)
	}
	if err := s.aktualisieren(context.Background(), 1, srv.URL+"/gibts-nicht"); err == nil {
		t.Fatal("erwartet Fehler")
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM rss_items`).Scan(&n)
	if n != 3 {
		t.Errorf("Meldungen müssen bleiben, waren %d", n)
	}
	var fehler string
	_ = s.db.QueryRow(`SELECT last_error FROM rss_feeds WHERE id = 1`).Scan(&fehler)
	if fehler == "" {
		t.Error("der Fehler sollte festgehalten sein")
	}

	// Doppelter Abruf legt keine Doppelten an.
	_ = s.aktualisieren(context.Background(), 1, srv.URL+"/feed.xml")
	_ = s.aktualisieren(context.Background(), 1, srv.URL+"/feed.xml")
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM rss_items`).Scan(&n)
	if n != 3 {
		t.Errorf("nach zwei Abrufen %d Meldungen", n)
	}
}

func TestDatum(t *testing.T) {
	if _, ok := datum("kein Datum"); ok {
		t.Error("unlesbar sollte nicht ok sein")
	}
	z, ok := datum("2999-01-01T00:00:00Z")
	if !ok || z.After(time.Now().Add(time.Minute)) {
		t.Errorf("Zukunft muss auf jetzt gekappt werden: %v", z)
	}
}
