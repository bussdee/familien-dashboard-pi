package chores

import (
	"testing"
	"time"
)

func wien(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		t.Skip("Zeitzonendaten nicht verfügbar")
	}
	return loc
}

// Der Fall, der das Ganze ausgelöst hat: Eine tägliche Aufgabe wurde erledigt,
// danach konnte sie jeder weitere Benutzer noch einmal abhaken und bekam
// dafür erneut Punkte.
func TestErledigteAufgabeIstAmSelbenTagNichtMehrFaellig(t *testing.T) {
	loc := wien(t)
	erledigt := time.Date(2026, 9, 8, 18, 0, 0, 0, loc)
	stichtag := dueDate(erledigt, 1)

	fuenfMinutenSpaeter := erledigt.Add(5 * time.Minute)
	if isDue(fuenfMinutenSpaeter, &stichtag) {
		t.Error("gerade erledigte Tagesaufgabe gilt weiterhin als fällig")
	}

	kurzVorMitternacht := time.Date(2026, 9, 8, 23, 59, 0, 0, loc)
	if isDue(kurzVorMitternacht, &stichtag) {
		t.Error("Aufgabe wird noch am selben Abend wieder fällig")
	}
}

// Die Kehrseite: Der Stichtag ist ein Tag, keine Uhrzeit. Wer abends erledigt,
// muss am nächsten Morgen wieder abhaken können.
func TestTagesaufgabeIstAmNaechstenMorgenWiederFaellig(t *testing.T) {
	loc := wien(t)
	stichtag := dueDate(time.Date(2026, 9, 8, 22, 0, 0, 0, loc), 1)

	morgens := time.Date(2026, 9, 9, 7, 30, 0, 0, loc)
	if !isDue(morgens, &stichtag) {
		t.Errorf("am Folgetag um 7:30 nicht fällig (Stichtag %s)", stichtag)
	}
}

func TestNieErledigteAufgabeIstFaellig(t *testing.T) {
	if !isDue(time.Now(), nil) {
		t.Error("Aufgabe ohne Stichtag müsste offen sein")
	}
}

func TestUeberfaelligeAufgabeBleibtFaellig(t *testing.T) {
	loc := wien(t)
	stichtag := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	jetzt := time.Date(2026, 9, 8, 12, 0, 0, 0, loc)
	if !isDue(jetzt, &stichtag) {
		t.Error("eine Woche überfällige Aufgabe gilt nicht als fällig")
	}
	if tage := daysUntil(jetzt, stichtag); tage != -7 {
		t.Errorf("daysUntil = %d, erwartet -7", tage)
	}
}

func TestDaysUntilZaehltKalendertage(t *testing.T) {
	loc := wien(t)
	jetzt := time.Date(2026, 9, 8, 23, 30, 0, 0, loc)

	faelle := []struct {
		name string
		due  time.Time
		want int
	}{
		{"heute früh", time.Date(2026, 9, 8, 0, 0, 0, 0, loc), 0},
		{"morgen", time.Date(2026, 9, 9, 0, 0, 0, 0, loc), 1},
		{"in einer Woche", time.Date(2026, 9, 15, 0, 0, 0, 0, loc), 7},
		{"gestern", time.Date(2026, 9, 7, 0, 0, 0, 0, loc), -1},
	}
	for _, f := range faelle {
		if got := daysUntil(jetzt, f.due); got != f.want {
			t.Errorf("%s: daysUntil = %d, erwartet %d", f.name, got, f.want)
		}
	}
}

// Zweimal im Jahr ist ein Tag 23 oder 25 Stunden lang. In Wien endet die
// Sommerzeit am 25. Oktober 2026.
func TestZeitumstellungVerschiebtDenStichtagNicht(t *testing.T) {
	loc := wien(t)
	erledigt := time.Date(2026, 10, 24, 20, 0, 0, 0, loc)
	stichtag := dueDate(erledigt, 1)

	if stichtag.Day() != 25 || stichtag.Month() != time.October {
		t.Fatalf("Stichtag %s, erwartet den 25.10.", stichtag)
	}
	if isDue(erledigt.Add(time.Hour), &stichtag) {
		t.Error("am Abend der Erledigung schon wieder fällig")
	}
	amStichtag := time.Date(2026, 10, 25, 9, 0, 0, 0, loc)
	if !isDue(amStichtag, &stichtag) {
		t.Error("am Tag der Zeitumstellung nicht fällig")
	}
}

// Ein Intervall von 0 oder weniger würde eine Aufgabe sofort wieder fällig
// machen und damit genau die Doppelvergabe erlauben, die abgestellt werden soll.
func TestUngueltigesIntervallWirdAufEinenTagGesetzt(t *testing.T) {
	loc := wien(t)
	erledigt := time.Date(2026, 9, 8, 12, 0, 0, 0, loc)
	for _, intervall := range []int{0, -3} {
		stichtag := dueDate(erledigt, intervall)
		if isDue(erledigt.Add(time.Minute), &stichtag) {
			t.Errorf("Intervall %d macht die Aufgabe sofort wieder fällig", intervall)
		}
	}
}

// Eine frisch angelegte Aufgabe muss sich noch am selben Tag abhaken lassen.
func TestNeueAufgabeIstSofortFaellig(t *testing.T) {
	loc := wien(t)
	jetzt := time.Date(2026, 9, 8, 14, 0, 0, 0, loc)
	stichtag := startOfDay(jetzt)
	if !isDue(jetzt, &stichtag) {
		t.Error("neu angelegte Aufgabe ist am Anlagetag nicht fällig")
	}
	if daysUntil(jetzt, stichtag) != 0 {
		t.Error("neue Aufgabe müsste heute fällig sein")
	}
}
