package neuigkeiten

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"family-dashboard/backend/internal/abruf"
)

var errKeinFeed = errors.New("Die Adresse liefert keinen Feed (RSS oder Atom)")

// Eintrag ist eine Meldung aus einem Feed, schon bereinigt.
type Eintrag struct {
	GUID      string
	Titel     string
	Link      string
	Text      string
	Veroeff   time.Time
	zeitFehlt bool
}

// Gelesen ist ein ganzer Feed.
type Gelesen struct {
	Titel     string
	Eintraege []Eintrag
}

const (
	maxEintraegeJeFeed = 40
	maxTitelLaenge     = 200
	maxTextLaenge      = 400
)

type xmlLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Text string `xml:",chardata"`
}

type xmlEintrag struct {
	Title     string    `xml:"title"`
	Link      []xmlLink `xml:"link"`
	GUID      string    `xml:"guid"`
	ID        string    `xml:"id"`
	Descr     string    `xml:"description"`
	Summary   string    `xml:"summary"`
	Content   string    `xml:"content"`
	Encoded   string    `xml:"encoded"`
	PubDate   string    `xml:"pubDate"`
	Published string    `xml:"published"`
	Updated   string    `xml:"updated"`
	Date      string    `xml:"date"`
}

type xmlFeed struct {
	XMLName xml.Name
	Title   string `xml:"title"`
	Channel struct {
		Title string       `xml:"title"`
		Items []xmlEintrag `xml:"item"`
	} `xml:"channel"`
	Items   []xmlEintrag `xml:"item"`  // RSS 1.0 (RDF): die Meldungen stehen neben dem Kanal
	Entries []xmlEintrag `xml:"entry"` // Atom
}

// Lesen liest RSS 2.0, RSS 1.0 und Atom. Alles, was später angezeigt wird, geht
// durch Klartext: Ein Feed ist fremder Text und wird nie als HTML ausgegeben.
func Lesen(daten []byte) (Gelesen, error) {
	d := xml.NewDecoder(bytes.NewReader(daten))
	d.Strict = false
	d.CharsetReader = zeichensatz
	var f xmlFeed
	if err := d.Decode(&f); err != nil {
		return Gelesen{}, errKeinFeed
	}

	var roh []xmlEintrag
	var titel string
	switch strings.ToLower(f.XMLName.Local) {
	case "rss":
		roh, titel = f.Channel.Items, f.Channel.Title
	case "rdf":
		roh, titel = f.Items, f.Channel.Title
	case "feed":
		roh, titel = f.Entries, f.Title
	default:
		return Gelesen{}, errKeinFeed
	}

	out := Gelesen{Titel: abruf.Klartext(titel, maxTitelLaenge, false)}
	for _, e := range roh {
		titelE := abruf.Klartext(e.Title, maxTitelLaenge, false)
		link := eintragLink(e)
		if titelE == "" && link == "" {
			continue
		}
		if titelE == "" {
			titelE = link
		}
		text := e.Summary
		for _, kandidat := range []string{e.Descr, e.Content, e.Encoded} {
			if text == "" {
				text = kandidat
			}
		}
		guid := strings.TrimSpace(firstNonEmpty(e.GUID, e.ID, link, titelE))
		if len(guid) > 500 {
			guid = guid[:500]
		}
		zeit, ok := datum(firstNonEmpty(e.PubDate, e.Published, e.Updated, e.Date))
		out.Eintraege = append(out.Eintraege, Eintrag{
			GUID: guid, Titel: titelE, Link: link,
			Text:      abruf.Klartext(text, maxTextLaenge, false),
			Veroeff:   zeit,
			zeitFehlt: !ok,
		})
		if len(out.Eintraege) >= maxEintraegeJeFeed {
			break
		}
	}
	return out, nil
}

// eintragLink wählt die Adresse der Meldung. Atom kennt mehrere Links; der
// „alternate" (oder der erste ohne rel) ist die Seite zum Lesen. Nur http und
// https kommen durch: ein javascript:-Link wird nie zu einem Verweis.
func eintragLink(e xmlEintrag) string {
	var wahl string
	for _, l := range e.Link {
		a := strings.TrimSpace(l.Href)
		if a == "" {
			a = strings.TrimSpace(l.Text)
		}
		if a == "" {
			continue
		}
		if l.Rel == "" || l.Rel == "alternate" {
			wahl = a
			break
		}
		if wahl == "" {
			wahl = a
		}
	}
	if wahl == "" {
		return ""
	}
	sauber, err := abruf.Bereinigen(wahl)
	if err != nil || len(sauber) > 1000 {
		return ""
	}
	return sauber
}

func firstNonEmpty(l ...string) string {
	for _, s := range l {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

var datumsformate = []string{
	time.RFC3339, time.RFC3339Nano, time.RFC1123Z, time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", "2 Jan 2006 15:04:05 -0700",
	"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
}

// datum liest das Datum einer Meldung. Fehlt es oder ist es unlesbar, gilt der
// Zeitpunkt des Abrufs; ein Datum aus der Zukunft wird auf jetzt gekappt, damit
// eine falsche Uhr im Feed keine Meldung ewig oben festhält.
func datum(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range datumsformate {
		if t, err := time.Parse(layout, s); err == nil {
			if t.After(time.Now()) {
				return time.Now(), true
			}
			return t, true
		}
	}
	return time.Now(), false
}

var latin1Namen = regexp.MustCompile(`(?i)^(iso-?8859-?1|latin-?1|windows-?1252|cp1252|us-ascii|ascii)$`)

// zeichensatz kann die beiden verbreiteten Nicht-UTF-8-Fälle (Latin-1 und
// Windows-1252) lesen. Alles andere lehnt der Parser ab; ohne zusätzliche
// Abhängigkeit geht es nicht genauer.
func zeichensatz(name string, in io.Reader) (io.Reader, error) {
	if !latin1Namen.MatchString(strings.TrimSpace(name)) {
		return nil, errors.New("unbekannter Zeichensatz")
	}
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, c := range raw {
		b.WriteRune(rune(c)) // Latin-1: die ersten 256 Zeichen von Unicode
	}
	return strings.NewReader(b.String()), nil
}

// ---------------------------------------------------- Feed in einer Webseite

var feedLink = regexp.MustCompile(`(?is)<link\b[^>]*>`)
var attr = func(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)\b` + name + `\s*=\s*["']([^"']*)["']`)
}
var (
	attrRel  = attr("rel")
	attrTyp  = attr("type")
	attrHref = attr("href")
)

// FeedInSeite sucht in einer HTML-Seite den Verweis auf ihren Feed. Wer die
// Adresse einer Zeitung statt ihres Feeds einträgt, bekommt so trotzdem etwas.
func FeedInSeite(seite []byte) string {
	for _, tag := range feedLink.FindAllString(string(seite), 40) {
		rel := attrRel.FindStringSubmatch(tag)
		typ := attrTyp.FindStringSubmatch(tag)
		href := attrHref.FindStringSubmatch(tag)
		if rel == nil || typ == nil || href == nil {
			continue
		}
		if !strings.Contains(strings.ToLower(rel[1]), "alternate") {
			continue
		}
		t := strings.ToLower(typ[1])
		if strings.Contains(t, "rss+xml") || strings.Contains(t, "atom+xml") {
			return strings.TrimSpace(href[1])
		}
	}
	return ""
}
