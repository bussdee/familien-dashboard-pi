package einstellungen

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type speicher map[string]string

func (m speicher) Setting(k string) (string, bool, error) { v, ok := m[k]; return v, ok, nil }
func (m speicher) SetSetting(k, v string) error           { m[k] = v; return nil }

func TestStandardwerte(t *testing.T) {
	s := NewService(speicher{})
	if s.An("modul.neuigkeiten") {
		t.Error("Neuigkeiten rufen fremde Adressen ab und stehen standardmäßig aus")
	}
	if !s.An("modul.rezepte") {
		t.Error("Rezepte sind standardmäßig an")
	}
	if got := s.Wert("schoner.modus"); got != "wandgeraet" {
		t.Errorf("Bildschirmschoner: erwartet wandgeraet, war %q", got)
	}
	if got := s.Wert("gibt.es.nicht"); got != "" {
		t.Errorf("unbekannter Schlüssel liefert %q", got)
	}
}

func TestKaputterWertZaehltAlsNichtGesetzt(t *testing.T) {
	s := NewService(speicher{"schoner.leerlauf_min": "abc", "schoner.modus": "bunt"})
	if got := s.Wert("schoner.leerlauf_min"); got != "5" {
		t.Errorf("erwartet Standard 5, war %q", got)
	}
	if got := s.Wert("schoner.modus"); got != "wandgeraet" {
		t.Errorf("erwartet Standard, war %q", got)
	}
}

func TestPutPrueftAlles(t *testing.T) {
	sp := speicher{}
	s := NewService(sp)

	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Put(w, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)))
		return w
	}

	if w := put(`{"modul.neuigkeiten":"1","schoner.leerlauf_min":"10","schoner.nacht_von":"21:30"}`); w.Code != http.StatusOK {
		t.Fatalf("gültige Werte: %d %s", w.Code, w.Body)
	}
	if sp["modul.neuigkeiten"] != "1" || sp["schoner.leerlauf_min"] != "10" || sp["schoner.nacht_von"] != "21:30" {
		t.Errorf("nicht gespeichert: %v", sp)
	}

	for name, body := range map[string]string{
		"unbekannt":     `{"x.y":"1"}`,
		"kein Schalter": `{"modul.rezepte":"vielleicht"}`,
		"zu klein":      `{"schoner.leerlauf_min":"0"}`,
		"zu groß":       `{"schoner.leerlauf_min":"500"}`,
		"keine Uhrzeit": `{"schoner.nacht_von":"25:00"}`,
		"falsche Wahl":  `{"schoner.modus":"immer"}`,
		"leer":          `{}`,
		"kein JSON":     `nein`,
	} {
		if w := put(body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: erwartet 400, war %d", name, w.Code)
		}
	}

	// Ein einziger schlechter Wert verhindert die ganze Änderung.
	vorher := sp["schoner.modus"]
	if w := put(`{"schoner.modus":"alle","schoner.leerlauf_min":"0"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("erwartet 400, war %d", w.Code)
	}
	if sp["schoner.modus"] != vorher {
		t.Error("ein Teil der Änderung wurde trotz Fehler übernommen")
	}
}
