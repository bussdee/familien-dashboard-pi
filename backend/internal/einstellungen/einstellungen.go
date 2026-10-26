// Package einstellungen hält die Schalter der Familie: welche Bereiche es gibt
// (Neuigkeiten, Rezepte) und wie sich der Bildschirmschoner verhält.
//
// Das sind keine Dinge für die config.yaml: Ein Elternteil soll sie in der
// Verwaltung umlegen können, ohne dass jemand eine Datei auf dem Pi anfasst.
// Der Schlüsselvorrat ist geschlossen und jeder Wert wird geprüft — über
// diesen Weg lässt sich nichts anderes ablegen.
//
// Was Nutzerinnen und Nutzer für sich abschalten, ist etwas anderes und liegt
// in ihren eigenen Einstellungen (auth: preferences).
package einstellungen

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"family-dashboard/backend/internal/auth"
	"github.com/rs/zerolog/log"
)

// Speicher ist der Teil der Datenbank, den die Schalter brauchen.
type Speicher interface {
	Setting(key string) (string, bool, error)
	SetSetting(key, value string) error
}

type art int

const (
	schalter art = iota // "0" oder "1"
	wahl                // einer aus einer festen Liste
	zahl                // ganze Zahl in einem Bereich
	uhrzeit             // HH:MM
)

type eintrag struct {
	art      art
	standard string
	erlaubt  []string
	min, max int
}

// Neue Schalter stehen standardmäßig AUS, wenn sie etwas nach außen tun
// (Neuigkeiten rufen fremde Adressen ab).
var eintraege = map[string]eintrag{
	"modul.neuigkeiten":             {art: schalter, standard: "0"},
	"modul.neuigkeiten_persoenlich": {art: schalter, standard: "0"},
	"modul.rezepte":                 {art: schalter, standard: "1"},
	"schoner.modus":                 {art: wahl, standard: "wandgeraet", erlaubt: []string{"aus", "wandgeraet", "alle"}},
	"schoner.inhalt":                {art: wahl, standard: "diashow", erlaubt: []string{"diashow", "uhr"}},
	"schoner.leerlauf_min":          {art: zahl, standard: "5", min: 1, max: 120},
	"schoner.nacht_von":             {art: uhrzeit, standard: "22:00"},
	"schoner.nacht_bis":             {art: uhrzeit, standard: "06:00"},
	"schoner.nacht_dunkel":          {art: zahl, standard: "60", min: 0, max: 90},
}

var uhrzeitMuster = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func (e eintrag) gueltig(wert string) bool {
	switch e.art {
	case schalter:
		return wert == "0" || wert == "1"
	case wahl:
		for _, w := range e.erlaubt {
			if w == wert {
				return true
			}
		}
		return false
	case zahl:
		n, err := strconv.Atoi(wert)
		return err == nil && n >= e.min && n <= e.max
	case uhrzeit:
		return uhrzeitMuster.MatchString(wert)
	}
	return false
}

type Service struct {
	sp Speicher
}

func NewService(sp Speicher) *Service { return &Service{sp: sp} }

// Wert liefert den eingestellten Wert oder den Standard. Ein kaputter
// gespeicherter Wert zählt als nicht gesetzt.
func (s *Service) Wert(key string) string {
	e, ok := eintraege[key]
	if !ok {
		return ""
	}
	if s == nil || s.sp == nil {
		return e.standard
	}
	v, da, err := s.sp.Setting(key)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Schalter konnte nicht gelesen werden")
		return e.standard
	}
	if !da || !e.gueltig(v) {
		return e.standard
	}
	return v
}

// An sagt, ob ein Schalter (Wert "1") umgelegt ist.
func (s *Service) An(key string) bool { return s.Wert(key) == "1" }

// Alle liefert jeden Schlüssel mit seinem Wert.
func (s *Service) Alle() map[string]string {
	out := make(map[string]string, len(eintraege))
	for k := range eintraege {
		out[k] = s.Wert(k)
	}
	return out
}

// Get liefert alle Schalter. Lesen darf jeder, auch das Wandgerät: Die
// Oberfläche richtet Menü und Bildschirmschoner danach aus.
func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	auth.WriteJSON(w, s.Alle())
}

// Put ändert einen oder mehrere Schalter. Entweder werden alle übernommen
// oder keiner.
func (s *Service) Put(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil || len(req) == 0 {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	for k, v := range req {
		e, ok := eintraege[k]
		if !ok {
			auth.HTTPError(w, http.StatusBadRequest, "Unbekannte Einstellung")
			return
		}
		if !e.gueltig(v) {
			auth.HTTPError(w, http.StatusBadRequest, "Ungültiger Wert für die Einstellung")
			return
		}
	}
	for k, v := range req {
		if err := s.sp.SetSetting(k, v); err != nil {
			log.Error().Err(err).Str("key", k).Msg("Schalter konnte nicht gespeichert werden")
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
	}
	auth.WriteJSON(w, s.Alle())
}
