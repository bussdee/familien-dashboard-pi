package weather

import (
	"testing"
	"time"
)

// tag baut Stundenwerte für einen Testtag. Jeder Eintrag in muster steht für
// eine Stunde ab 00:00: 't' trocken, 'n' nass über die Menge, 'w' nass über
// die Wahrscheinlichkeit, 'g' trocken aber grau.
func tag(muster string) []stunde {
	loc := time.UTC
	basis := time.Date(2026, 9, 14, 0, 0, 0, 0, loc)
	out := make([]stunde, 0, len(muster))
	for i, z := range muster {
		st := stunde{Zeit: basis.Add(time.Duration(i) * time.Hour), Gefuehlt: 18, Cloud: 20}
		switch z {
		case 'n':
			st.Precip = 1.2
			st.Prob = 90
		case 'w':
			st.Precip = 0.05
			st.Prob = 60
		case 'g':
			st.Cloud = 85
		}
		out = append(out, st)
	}
	return out
}

func zeiten(f []Fenster) []string {
	out := make([]string, 0, len(f))
	for _, x := range f {
		von, _ := time.Parse(time.RFC3339, x.Von)
		bis, _ := time.Parse(time.RFC3339, x.Bis)
		out = append(out, von.Format("15:04")+"–"+bis.Format("15:04"))
	}
	return out
}

func sonne(h int) time.Time {
	return time.Date(2026, 9, 14, h, 0, 0, 0, time.UTC)
}

// Der Normalfall: Regen am Vormittag, danach wird es trocken.
func TestFensterNachDemRegen(t *testing.T) {
	stunden := tag("nnnnnnnnnnnttttttttnnnnn") // 00–10 nass, 11–18 trocken
	f := fensterFuerTag(sonne(6), sonne(20), stunden, sonne(5), Standardschwellen())
	if len(f) != 1 {
		t.Fatalf("erwartet ein Fenster, waren %d: %v", len(f), zeiten(f))
	}
	if zeiten(f)[0] != "11:00–19:00" {
		t.Errorf("erwartet 11:00–19:00, war %s", zeiten(f)[0])
	}
	if f[0].Stunden != 8 {
		t.Errorf("erwartet 8 Stunden, waren %v", f[0].Stunden)
	}
}

// Nachts wird nicht gerechnet, auch wenn es dort trocken ist.
func TestNurZwischenAufUndUntergang(t *testing.T) {
	// Durchgehend trocken, Sonne von 07 bis 19 Uhr.
	f := fensterFuerTag(sonne(7), sonne(19), tag("tttttttttttttttttttttttt"), sonne(5), Standardschwellen())
	if len(f) != 1 {
		t.Fatalf("erwartet ein Fenster, waren %d: %v", len(f), zeiten(f))
	}
	if zeiten(f)[0] != "07:00–19:00" {
		t.Errorf("erwartet auf den Tag beschnitten, war %s", zeiten(f)[0])
	}
}

// Die Wahrscheinlichkeit zählt mit: 60 % bei 0,05 mm ist nicht trocken.
func TestWahrscheinlichkeitMachtNass(t *testing.T) {
	// 08–11 Uhr "nur" wahrscheinlich nass.
	f := fensterFuerTag(sonne(6), sonne(20), tag("ttttttttwwwwtttttttttttt"), sonne(5), Standardschwellen())
	if len(f) != 2 {
		t.Fatalf("erwartet zwei Fenster, waren %d: %v", len(f), zeiten(f))
	}
	if zeiten(f)[0] != "06:00–08:00" || zeiten(f)[1] != "12:00–20:00" {
		t.Errorf("falsch geteilt: %v", zeiten(f))
	}
}

// Ein laufendes Fenster fängt jetzt an, nicht in der Vergangenheit.
func TestLaufendesFensterBeginntJetzt(t *testing.T) {
	f := fensterFuerTag(sonne(6), sonne(20), tag("tttttttttttttttttttttttt"), sonne(14), Standardschwellen())
	if len(f) != 1 {
		t.Fatalf("erwartet ein Fenster, waren %d", len(f))
	}
	if zeiten(f)[0] != "14:00–20:00" {
		t.Errorf("erwartet ab jetzt, war %s", zeiten(f)[0])
	}
	if !f[0].Jetzt {
		t.Error("das laufende Fenster sollte als Jetzt markiert sein")
	}
}

// Was vorbei ist, wird nicht mehr vorgeschlagen.
func TestVergangenesFensterFaelltWeg(t *testing.T) {
	// 06–10 trocken, danach Regen bis zum Abend.
	f := fensterFuerTag(sonne(6), sonne(20), tag("ttttttttttnnnnnnnnnnnnnn"), sonne(15), Standardschwellen())
	if len(f) != 0 {
		t.Errorf("erwartet kein Fenster mehr, waren %v", zeiten(f))
	}
}

// Eine trockene Stunde zwischen zwei Schauern ist kein Fenster.
func TestZuKurzZaehltNicht(t *testing.T) {
	f := fensterFuerTag(sonne(6), sonne(20), tag("nnnnnnnnnntnnnnnnnnnnnnn"), sonne(5), Standardschwellen())
	if len(f) != 0 {
		t.Errorf("eine Stunde Lücke ist kein Fenster, war %v", zeiten(f))
	}
}

// Grau ist trocken. Das Fenster gilt, wird aber nicht als sonnig beschrieben.
func TestGrauBleibtEinFenster(t *testing.T) {
	f := fensterFuerTag(sonne(6), sonne(20), tag("gggggggggggggggggggggggg"), sonne(5), Standardschwellen())
	if len(f) != 1 {
		t.Fatalf("erwartet ein Fenster, waren %d", len(f))
	}
	if f[0].Sonnig {
		t.Error("bei 85 %% Bewölkung ist nichts sonnig")
	}
	if f[0].Bewoelkung != 85 {
		t.Errorf("Bewölkung erwartet 85, war %d", f[0].Bewoelkung)
	}
}

func TestSonnigWirdErkannt(t *testing.T) {
	f := fensterFuerTag(sonne(6), sonne(20), tag("tttttttttttttttttttttttt"), sonne(5), Standardschwellen())
	if len(f) != 1 || !f[0].Sonnig {
		t.Errorf("bei 20 %% Bewölkung erwartet sonnig, war %+v", f)
	}
}

// Ein Tag ohne Sonnenzeiten oder mit unsinnigen ergibt kein Fenster statt
// einer Panik.
func TestOhneSonnenzeitenKeinFenster(t *testing.T) {
	if f := fensterFuerTag(time.Time{}, sonne(20), tag("tttt"), sonne(5), Standardschwellen()); f != nil {
		t.Errorf("ohne Sonnenaufgang erwartet nichts, war %v", zeiten(f))
	}
	if f := fensterFuerTag(sonne(20), sonne(6), tag("tttt"), sonne(5), Standardschwellen()); f != nil {
		t.Errorf("Untergang vor Aufgang erwartet nichts, war %v", zeiten(f))
	}
}

// Eine Lücke in den Daten trennt zwei Fenster — sonst behauptete das
// Dashboard einen durchgehenden Zeitraum, den es nicht kennt.
func TestDatenlueckeTrenntFenster(t *testing.T) {
	stunden := tag("tttttttttttttttttttttttt")
	// Die Stunden 12 und 13 herausnehmen.
	mitLuecke := append([]stunde{}, stunden[:12]...)
	mitLuecke = append(mitLuecke, stunden[14:]...)

	f := fensterFuerTag(sonne(6), sonne(20), mitLuecke, sonne(5), Standardschwellen())
	if len(f) != 2 {
		t.Fatalf("erwartet zwei Fenster, waren %d: %v", len(f), zeiten(f))
	}
	if zeiten(f)[0] != "06:00–12:00" || zeiten(f)[1] != "14:00–20:00" {
		t.Errorf("falsch getrennt: %v", zeiten(f))
	}
}

func TestNassSchwellen(t *testing.T) {
	if (stunde{Precip: 0.09, Prob: 54}).nass(Standardschwellen()) {
		t.Error("0,09 mm bei 54 %% ist trocken")
	}
	if !(stunde{Precip: 0.1}).nass(Standardschwellen()) {
		t.Error("0,1 mm ist nass")
	}
	if !(stunde{Prob: 55}).nass(Standardschwellen()) {
		t.Error("55 %% ist nass")
	}
}
