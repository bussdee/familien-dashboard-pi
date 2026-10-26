package rezepte

import (
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Fast jede Rezeptseite trägt ihr Rezept als schema.org/Recipe im HTML, damit
// Suchmaschinen es lesen können (JSON-LD). Genau das liest das Dashboard: Es
// muss keine Seite „verstehen", nur diese eine Beschriftung finden.

var errKeinRezept = errors.New("Auf dieser Seite wurde kein Rezept gefunden. Du kannst es von Hand eintragen.")

var (
	ldBlock   = regexp.MustCompile(`(?is)<script[^>]+type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
	tagMuster = regexp.MustCompile(`(?s)<[^>]*>`)
	umbruch   = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div)\s*/?>`)
	leerzeile = regexp.MustCompile(`[ \t\x{00a0}]+`)
	ogTitel   = regexp.MustCompile(`(?is)<meta[^>]+property\s*=\s*["']og:title["'][^>]+content\s*=\s*["']([^"']*)["']`)
	ogBild    = regexp.MustCompile(`(?is)<meta[^>]+property\s*=\s*["']og:image["'][^>]+content\s*=\s*["']([^"']*)["']`)
	dauerMstr = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:\d+(?:\.\d+)?S)?)?$`)
)

const (
	maxTitel     = 200
	maxZutaten   = 100
	maxZutatLen  = 200
	maxSchritte  = 60
	maxSchrittLn = 2000
	maxNotiz     = 1000
)

// Gelesen ist das Ergebnis des Lesens einer Seite.
type Gelesen struct {
	Titel     string
	Bild      string // Adresse, noch nicht heruntergeladen
	Portionen string
	Vorb      int // Minuten
	Kochen    int
	Gesamt    int
	Zutaten   []string
	Schritte  []string
	Notiz     string
	Kategorie string
}

// Lesen sucht das Rezept in einer HTML-Seite. basis ist die Adresse der Seite,
// gegen die ein relatives Bild aufgelöst wird.
func Lesen(seite []byte, basis string) (Gelesen, error) {
	text := string(seite)
	for _, m := range ldBlock.FindAllStringSubmatch(text, -1) {
		var wurzel any
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &wurzel); err != nil {
			continue
		}
		if knoten := findeRezept(wurzel, 0); knoten != nil {
			g := ausKnoten(knoten, basis)
			if g.Titel == "" {
				if t := ogTitel.FindStringSubmatch(text); t != nil {
					g.Titel = bereinige(t[1], maxTitel)
				}
			}
			if g.Bild == "" {
				if b := ogBild.FindStringSubmatch(text); b != nil {
					g.Bild = aufloesen(basis, html.UnescapeString(b[1]))
				}
			}
			if g.Titel != "" && (len(g.Zutaten) > 0 || len(g.Schritte) > 0) {
				return g, nil
			}
		}
	}
	return Gelesen{}, errKeinRezept
}

// findeRezept geht durch die Hülle (Liste, @graph, mainEntity) bis zum Knoten
// mit dem Typ Recipe.
func findeRezept(v any, tiefe int) map[string]any {
	if tiefe > 6 {
		return nil
	}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if r := findeRezept(e, tiefe+1); r != nil {
				return r
			}
		}
	case map[string]any:
		if hatTyp(x["@type"], "Recipe") {
			return x
		}
		for _, k := range []string{"@graph", "mainEntity", "mainEntityOfPage", "itemListElement", "item"} {
			if sub, ok := x[k]; ok {
				if r := findeRezept(sub, tiefe+1); r != nil {
					return r
				}
			}
		}
	}
	return nil
}

func hatTyp(v any, typ string) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, typ)
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.EqualFold(s, typ) {
				return true
			}
		}
	}
	return false
}

func ausKnoten(k map[string]any, basis string) Gelesen {
	g := Gelesen{
		Titel:     bereinige(text(k["name"]), maxTitel),
		Portionen: bereinige(portionen(k["recipeYield"]), 60),
		Vorb:      minuten(text(k["prepTime"])),
		Kochen:    minuten(text(k["cookTime"])),
		Gesamt:    minuten(text(k["totalTime"])),
		Kategorie: bereinige(erstes(k["recipeCategory"]), 60),
		Notiz:     bereinige(text(k["description"]), maxNotiz),
	}
	if g.Gesamt == 0 && g.Vorb+g.Kochen > 0 {
		g.Gesamt = g.Vorb + g.Kochen
	}

	zutaten := k["recipeIngredient"]
	if zutaten == nil {
		zutaten = k["ingredients"]
	}
	for _, z := range liste(zutaten) {
		if z = bereinige(z, maxZutatLen); z != "" {
			g.Zutaten = append(g.Zutaten, z)
			if len(g.Zutaten) >= maxZutaten {
				break
			}
		}
	}
	for _, s := range anweisungen(k["recipeInstructions"], 0) {
		if s = bereinige(s, maxSchrittLn); s != "" {
			g.Schritte = append(g.Schritte, s)
			if len(g.Schritte) >= maxSchritte {
				break
			}
		}
	}
	if b := bildAdresse(k["image"], 0); b != "" {
		g.Bild = aufloesen(basis, b)
	}
	return g
}

// text macht aus einem JSON-Wert eine Zeichenkette.
func text(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any:
		return text(t["name"])
	}
	return ""
}

func erstes(v any) string {
	if l, ok := v.([]any); ok {
		if len(l) > 0 {
			return text(l[0])
		}
		return ""
	}
	return text(v)
}

func portionen(v any) string {
	if l, ok := v.([]any); ok {
		// Viele Seiten schreiben ["4", "4 Portionen"]: das Längere sagt mehr.
		best := ""
		for _, e := range l {
			if s := text(e); len(s) > len(best) {
				best = s
			}
		}
		return best
	}
	return text(v)
}

func liste(v any) []string {
	switch t := v.(type) {
	case string:
		return strings.Split(t, "\n")
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, text(e))
		}
		return out
	}
	return nil
}

// anweisungen kennt die drei Schreibweisen: ein Text, eine Liste von Texten,
// eine Liste von HowToStep (auch in HowToSection verschachtelt).
func anweisungen(v any, tiefe int) []string {
	if tiefe > 4 {
		return nil
	}
	switch t := v.(type) {
	case string:
		return strings.Split(klartext(t), "\n")
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, anweisungen(e, tiefe+1)...)
		}
		return out
	case map[string]any:
		if sub, ok := t["itemListElement"]; ok {
			return anweisungen(sub, tiefe+1)
		}
		if s := text(t["text"]); s != "" {
			return strings.Split(klartext(s), "\n")
		}
		if s := text(t["name"]); s != "" {
			return []string{s}
		}
	}
	return nil
}

func bildAdresse(v any, tiefe int) string {
	if tiefe > 3 {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, e := range t {
			if s := bildAdresse(e, tiefe+1); s != "" {
				return s
			}
		}
	case map[string]any:
		if s := text(t["url"]); s != "" {
			return s
		}
		return text(t["contentUrl"])
	}
	return ""
}

// aufloesen macht aus einer relativen Bildadresse eine vollständige.
func aufloesen(basis, ref string) string {
	ref = strings.TrimSpace(html.UnescapeString(ref))
	if ref == "" {
		return ""
	}
	b, err := url.Parse(basis)
	if err != nil {
		return ""
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return b.ResolveReference(r).String()
}

// minuten liest eine ISO-8601-Dauer („PT1H30M") und liefert Minuten.
func minuten(d string) int {
	m := dauerMstr.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(d)))
	if m == nil {
		return 0
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	gesamt := n(m[1])*24*60 + n(m[2])*60 + n(m[3])
	if gesamt > 60*24*14 {
		return 0
	}
	return gesamt
}

// klartext macht aus HTML-Schnipseln Text: Umbrüche bleiben, Tags verschwinden,
// Sonderzeichen werden gelesen. Das Ergebnis wird später als Text angezeigt,
// nie als HTML.
func klartext(s string) string {
	s = umbruch.ReplaceAllString(s, "\n")
	s = tagMuster.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

// bereinige macht eine Zeile daraus: kein HTML, keine Steuerzeichen, einfache
// Leerzeichen, höchstens max Zeichen.
func bereinige(s string, max int) string {
	s = klartext(s)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(leerzeile.ReplaceAllString(s, " "))
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}
