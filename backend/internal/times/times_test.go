package times

import "testing"

// Der Fall, der das Ganze ausgelöst hat: Die Mutter hat Nachtschicht von
// 20:00 bis 07:00. Bis 1.6.0 wurde das als Fehleingabe abgelehnt.
func TestNachtschichtWirdAngenommen(t *testing.T) {
	start, ende, err := pruefeZeiten("20:00", "07:00")
	if err != nil {
		t.Fatalf("Nachtschicht abgelehnt: %v", err)
	}
	if start != "20:00" || ende != "07:00" {
		t.Errorf("Zeiten verändert: %q bis %q", start, ende)
	}
	if !ueberMitternacht(start, ende) {
		t.Error("20:00–07:00 sollte als über Mitternacht erkannt werden")
	}
}

func TestGewoehnlicheSchichtBleibtGewoehnlich(t *testing.T) {
	if ueberMitternacht("08:00", "16:30") {
		t.Error("08:00–16:30 läuft nicht über Mitternacht")
	}
	if _, _, err := pruefeZeiten("08:00", "16:30"); err != nil {
		t.Errorf("Tagschicht abgelehnt: %v", err)
	}
}

// Gleiche Zeiten sind mehrdeutig — null Stunden oder vierundzwanzig.
func TestGleicheZeitenWerdenAbgelehnt(t *testing.T) {
	if _, _, err := pruefeZeiten("08:00", "08:00"); err == nil {
		t.Error("08:00–08:00 hätte abgelehnt werden müssen")
	}
}

// Ganztägig weg braucht keine Uhrzeit — Urlaub, Krankheit.
func TestOhneUhrzeitIstErlaubt(t *testing.T) {
	start, ende, err := pruefeZeiten("", "")
	if err != nil || start != "" || ende != "" {
		t.Errorf("leere Zeiten: %q %q %v", start, ende, err)
	}
}

func TestUnfugWirdAbgelehnt(t *testing.T) {
	for _, f := range [][2]string{
		{"8:00", "16:00"}, {"25:00", "26:00"}, {"08:60", "09:00"}, {"acht", "neun"},
		{"08:00", ""}, {"", "16:00"},
	} {
		if _, _, err := pruefeZeiten(f[0], f[1]); err == nil {
			t.Errorf("%q bis %q hätte abgelehnt werden müssen", f[0], f[1])
		}
	}
}

// „Ab wann sind alle da" darf bei einer Nachtschicht keine Zahl nennen. Die
// späteste Uhrzeit wäre der Dienstbeginn — als Rückkehr gelesen wäre das
// schlicht falsch.
func TestAlleDaAbBeiNachtschicht(t *testing.T) {
	nacht := Block{Kind: KindArbeit, Start: "20:00", End: "07:00", GehtWeiter: true}
	schule := Block{Kind: KindSchule, Start: "08:00", End: "13:00"}

	if got := alleDaAb([]Block{schule, nacht}); got != "" {
		t.Errorf("mit Nachtschicht erwartet leer, war %q", got)
	}
	if got := alleDaAb([]Block{schule}); got != "13:00" {
		t.Errorf("ohne Nachtschicht erwartet 13:00, war %q", got)
	}
}

// Teildienst: zwei Blöcke am selben Tag. Die späteste Rückkehr zählt.
func TestAlleDaAbBeiTeildienst(t *testing.T) {
	frueh := Block{Kind: KindArbeit, Start: "06:00", End: "10:00"}
	spaet := Block{Kind: KindArbeit, Start: "15:00", End: "20:00"}
	if got := alleDaAb([]Block{frueh, spaet}); got != "20:00" {
		t.Errorf("erwartet 20:00, war %q", got)
	}
}

// Urlaub und Frei sind Nicht-Termine: Sie heben das Wochenmuster auf, machen
// aber niemanden abwesend.
func TestFreiUndUrlaubZaehlenNichtAlsAbwesend(t *testing.T) {
	for _, art := range []string{KindFrei, KindUrlaub, KindKrank} {
		if abwesend(art) {
			t.Errorf("%q sollte nicht als abwesend zählen", art)
		}
	}
	for _, art := range []string{KindArbeit, KindSchule, KindSonstiges} {
		if !abwesend(art) {
			t.Errorf("%q sollte als abwesend zählen", art)
		}
	}
	// Ein ganztägiger Urlaub darf die Zeile nicht leeren.
	urlaub := Block{Kind: KindUrlaub, Start: "", End: ""}
	schule := Block{Kind: KindSchule, Start: "08:00", End: "13:00"}
	if got := alleDaAb([]Block{urlaub, schule}); got != "13:00" {
		t.Errorf("erwartet 13:00, war %q", got)
	}
}

// Ein ganztägig Abwesender gibt keine Rückkehrzeit her.
func TestGanztaegigWegGibtKeineZeit(t *testing.T) {
	ganz := Block{Kind: KindArbeit, Start: "", End: ""}
	if got := alleDaAb([]Block{ganz}); got != "" {
		t.Errorf("erwartet leer, war %q", got)
	}
}

// Die Auflösung: Ein konkreter Eintrag sticht das Wochenmuster.
func TestKonkreterTagStichtDasMuster(t *testing.T) {
	p := Person{ID: 1, Name: "Kind"}
	muster := []WeeklyEntry{{UserID: 1, Weekday: 2, Start: "08:00", End: "13:00", Kind: KindSchule}}

	ohne := bloeckeFuer(p, nil, muster, 2)
	if len(ohne) != 1 || ohne[0].Kind != KindSchule || !ohne[0].AusMuster {
		t.Fatalf("ohne konkreten Eintrag erwartet das Muster, war %+v", ohne)
	}

	feiertag := []DayEntry{{UserID: 1, Kind: KindFrei}}
	mit := bloeckeFuer(p, feiertag, muster, 2)
	if len(mit) != 1 || mit[0].Kind != KindFrei || mit[0].AusMuster {
		t.Fatalf("konkreter Eintrag sollte stechen, war %+v", mit)
	}
}

// Am falschen Wochentag gilt das Muster nicht.
func TestMusterGiltNurAmRichtigenTag(t *testing.T) {
	p := Person{ID: 1}
	muster := []WeeklyEntry{{UserID: 1, Weekday: 2, Start: "08:00", End: "13:00", Kind: KindSchule}}
	if got := bloeckeFuer(p, nil, muster, 3); len(got) != 0 {
		t.Errorf("am Donnerstag erwartet nichts, war %+v", got)
	}
}

// Ein Teildienst im Wochenmuster ergibt zwei Blöcke an einem Tag.
func TestZweiBloeckeAmSelbenTag(t *testing.T) {
	p := Person{ID: 2}
	muster := []WeeklyEntry{
		{UserID: 2, Weekday: 5, Start: "06:00", End: "10:00", Kind: KindArbeit},
		{UserID: 2, Weekday: 5, Start: "15:00", End: "20:00", Kind: KindArbeit},
	}
	got := bloeckeFuer(p, nil, muster, 5)
	if len(got) != 2 {
		t.Fatalf("erwartet zwei Blöcke, waren %d", len(got))
	}
	if alleDaAb(got) != "20:00" {
		t.Errorf("erwartet 20:00, war %q", alleDaAb(got))
	}
}
