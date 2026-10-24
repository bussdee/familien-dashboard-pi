package weather

import (
	"math"
	"time"
)

// Trockenfenster — der Teil des Wetters, nach dem eine Familie ihren
// Nachmittag plant.
//
// „82 % Regenwahrscheinlichkeit" beantwortet die Frage nicht, die am
// Küchentisch gestellt wird. Die lautet: *Wann können wir raus?* Und die
// Antwort ist ein Zeitraum, keine Zahl.
//
// Zwei Entscheidungen stecken darin:
//
//   - **Gerechnet wird nur zwischen Sonnenauf- und -untergang.** Ein
//     trockenes Fenster um drei Uhr nachts ist statistisch richtig und
//     praktisch wertlos.
//   - **Trocken reicht.** Sonne und Wärme werden dazugeschrieben, entscheiden
//     aber nicht mit. Wer bei zwölf Grad und Wolken spazieren gehen will,
//     soll das Fenster trotzdem sehen.

// Schwellen sind die Zahlen, nach denen entschieden wird. Sie stehen in
// config.yaml unter weather.dry_window; hier sind die Werte, mit denen das
// Dashboard bisher gerechnet hat.
type Schwellen struct {
	// Ab wann eine Stunde als nass gilt. Beide Wege zählen, und das ist der
	// Punkt: Bei „60 % Wahrscheinlichkeit, 0,05 mm" meldete das Dashboard
	// früher trockenes Wetter — die Menge lag unter der Schwelle, die
	// Wahrscheinlichkeit sprach eine andere Sprache.
	NassAbMillimeter     float64
	NassAbWahrscheinlich int

	// Eine einzelne trockene Stunde zwischen zwei Schauern ist kein Fenster,
	// sondern eine Lücke. Bis man Schuhe an hat und vor der Tür steht, ist sie
	// vorbei. Anderthalb Stunden sind das Minimum, für das sich Aufbrechen
	// lohnt.
	MindestStunden float64

	// Ab wann ein Fenster als sonnig beschrieben wird. Beschreibt nur, es
	// entscheidet nichts.
	SonnigBisBewoelkung int
}

// Standardschwellen sind die Werte bis 2.3.
func Standardschwellen() Schwellen {
	return Schwellen{
		NassAbMillimeter:     0.1,
		NassAbWahrscheinlich: 55,
		MindestStunden:       1.5,
		SonnigBisBewoelkung:  40,
	}
}

// stunde ist ein Stundenwert, schon in Ortszeit gelesen. Die Rohdaten kommen
// als sieben parallele Listen; einmal zusammengesetzt lässt sich damit
// arbeiten, ohne überall den gleichen Index mitzuschleppen.
type stunde struct {
	Zeit     time.Time
	Temp     float64
	Gefuehlt float64
	Precip   float64
	Wind     float64
	Prob     int
	Cloud    int
	Code     int
}

func (s stunde) nass(sw Schwellen) bool {
	return s.Precip >= sw.NassAbMillimeter || s.Prob >= sw.NassAbWahrscheinlich
}

// stundenLesen setzt die parallelen Listen der Antwort zu Stundenwerten
// zusammen. Eine Zeile, die sich nicht lesen lässt, wird übersprungen statt
// den ganzen Tag zu verwerfen.
func stundenLesen(api *apiResponse, tz string) []stunde {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	out := make([]stunde, 0, len(api.Hourly.Time))
	for i, t := range api.Hourly.Time {
		// Open-Meteo gibt Ortszeit ohne Zeitzonenangabe zurück.
		zeit, err := time.ParseInLocation("2006-01-02T15:04", t, loc)
		if err != nil {
			continue
		}
		out = append(out, stunde{
			Zeit:     zeit,
			Temp:     at(api.Hourly.Temperature, i),
			Gefuehlt: at(api.Hourly.Apparent, i),
			Precip:   at(api.Hourly.Precip, i),
			Wind:     at(api.Hourly.WindSpeed, i),
			Prob:     at(api.Hourly.PrecipProb, i),
			Cloud:    at(api.Hourly.CloudCover, i),
			Code:     at(api.Hourly.WeatherCode, i),
		})
	}
	return out
}

// fensterFuerTag sucht die Trockenfenster eines Tages.
//
// jetzt wird gebraucht, weil der heutige Tag anders zu behandeln ist als die
// kommenden: Ein Fenster, das um zehn Uhr war, hilft um sechs Uhr abends
// nicht mehr — und ein laufendes Fenster fängt nicht um zehn an, sondern
// jetzt.
func fensterFuerTag(auf, unter time.Time, alle []stunde, jetzt time.Time, sw Schwellen) []Fenster {
	if auf.IsZero() || unter.IsZero() || !unter.After(auf) {
		return nil
	}

	// Die Stunden dieses Tages zwischen Auf- und Untergang, der Reihe nach.
	var tagsueber []stunde
	for _, st := range alle {
		// Eine Stunde zählt, wenn sie vor Sonnenuntergang beginnt und nicht
		// vor Sonnenaufgang endet.
		ende := st.Zeit.Add(time.Hour)
		if st.Zeit.Before(unter) && !ende.Before(auf) {
			tagsueber = append(tagsueber, st)
		}
	}
	if len(tagsueber) == 0 {
		return nil
	}

	var fenster []Fenster
	var lauf []stunde

	abschliessen := func() {
		if len(lauf) == 0 {
			return
		}
		if f, ok := fensterAus(lauf, auf, unter, jetzt, sw); ok {
			fenster = append(fenster, f)
		}
		lauf = nil
	}

	for _, st := range tagsueber {
		if st.nass(sw) {
			abschliessen()
			continue
		}
		// Eine Lücke in den Daten bricht den Lauf ebenfalls: Zwei Stunden,
		// zwischen denen etwas fehlt, sind kein durchgehendes Fenster.
		if len(lauf) > 0 && !st.Zeit.Equal(lauf[len(lauf)-1].Zeit.Add(time.Hour)) {
			abschliessen()
		}
		lauf = append(lauf, st)
	}
	abschliessen()

	return fenster
}

// fensterAus macht aus einem Lauf trockener Stunden ein Fenster — beschnitten
// auf Tageslicht und, am heutigen Tag, auf „ab jetzt".
func fensterAus(lauf []stunde, auf, unter, jetzt time.Time, sw Schwellen) (Fenster, bool) {
	von := lauf[0].Zeit
	bis := lauf[len(lauf)-1].Zeit.Add(time.Hour)

	// Auf das Tageslicht beschneiden.
	if von.Before(auf) {
		von = auf
	}
	if bis.After(unter) {
		bis = unter
	}
	// Und auf das, was noch kommt. Ein Fenster, das vorbei ist, wird nicht
	// gezeigt; eines, das läuft, fängt jetzt an.
	laeuftGerade := !jetzt.Before(von) && jetzt.Before(bis)
	if laeuftGerade {
		von = jetzt
	} else if bis.Before(jetzt) {
		return Fenster{}, false
	}

	stunden := bis.Sub(von).Hours()
	if stunden < sw.MindestStunden {
		return Fenster{}, false
	}

	// Beschreibung: was einen dort erwartet. Gemittelt wird über die
	// ursprünglichen Stunden, nicht über den beschnittenen Zeitraum — für
	// „ist es sonnig" ist das genau genug.
	var wolkenSumme int
	gefuehltMin, gefuehltMax := math.Inf(1), math.Inf(-1)
	for _, st := range lauf {
		wolkenSumme += st.Cloud
		gefuehltMin = math.Min(gefuehltMin, st.Gefuehlt)
		gefuehltMax = math.Max(gefuehltMax, st.Gefuehlt)
	}
	wolken := wolkenSumme / len(lauf)

	return Fenster{
		Von:         von.Format(time.RFC3339),
		Bis:         bis.Format(time.RFC3339),
		Stunden:     math.Round(stunden*2) / 2,
		Bewoelkung:  wolken,
		GefuehltAb:  math.Round(gefuehltMin*10) / 10,
		GefuehltBis: math.Round(gefuehltMax*10) / 10,
		Sonnig:      wolken < sw.SonnigBisBewoelkung,
		Jetzt:       laeuftGerade,
	}, true
}

// tagesZeit liest eine Zeitangabe von Open-Meteo in Ortszeit. Leere oder
// unlesbare Werte geben die Nullzeit zurück; der Aufrufer prüft darauf.
func tagesZeit(wert, tz string) time.Time {
	if wert == "" {
		return time.Time{}
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", wert, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}
