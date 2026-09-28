package calendar

import (
	"testing"
	"time"
)

func tagAm(j int, m time.Month, t int) time.Time {
	return time.Date(j, m, t, 9, 0, 0, 0, time.UTC)
}

// „Immer donnerstags" heisst: das Datum rutscht auf den nächsten Donnerstag.
func TestNaechsterWochentag(t *testing.T) {
	// 14.09.2026 ist ein Montag.
	montag := tagAm(2026, time.September, 14)
	faelle := []struct {
		gewuenscht int
		erwartet   int
	}{
		{0, 14}, // Montag: bleibt
		{3, 17}, // Donnerstag
		{6, 20}, // Sonntag
	}
	for _, f := range faelle {
		got := naechsterWochentag(montag, f.gewuenscht)
		if got.Day() != f.erwartet {
			t.Errorf("Wochentag %d: erwartet %d., war %d.", f.gewuenscht, f.erwartet, got.Day())
		}
		// Die Uhrzeit darf dabei nicht verrutschen.
		if got.Hour() != 9 {
			t.Errorf("Wochentag %d: Uhrzeit verschoben auf %d Uhr", f.gewuenscht, got.Hour())
		}
	}
}

// Ein täglicher Termin aus dem letzten Jahr lief früher in die Obergrenze für
// Wiederholungen und verschwand dadurch aus dem Kalender.
func TestTaeglicherTerminAusDerVergangenheit(t *testing.T) {
	s := &Service{loc: time.UTC}
	von := tagAm(2026, time.September, 14)
	bis := von.AddDate(0, 0, 14)

	e := StoredEvent{
		ID:     1,
		Title:  "Tabletten",
		Start:  tagAm(2024, time.January, 3),
		End:    tagAm(2024, time.January, 3).Add(30 * time.Minute),
		Repeat: RepeatDaily,
	}

	events := s.expandStored(e, von, bis)
	if len(events) != 15 {
		t.Fatalf("erwartet 15 Termine (zwei Wochen täglich), waren %d", len(events))
	}
	if !events[0].Start.Equal(von) {
		t.Errorf("erster Termin erwartet am %s, war %s", von, events[0].Start)
	}
	if events[0].Start.Hour() != 9 {
		t.Errorf("Uhrzeit verrutscht: %s", events[0].Start)
	}
}

// Vorspulen darf nie zu weit springen — eine übersprungene Wiederholung
// fehlt im Kalender und niemand merkt es.
func TestVorspulenSpringtNieZuWeit(t *testing.T) {
	start := tagAm(2026, time.January, 1)
	dauer := time.Hour
	for _, wdh := range []Repeat{RepeatDaily, RepeatWeekly, RepeatMonthly, RepeatYearly} {
		for _, tage := range []int{0, 1, 5, 40, 400, 4000} {
			von := start.AddDate(0, 0, tage)
			got := vorspulen(start, wdh, von, dauer)
			if got.After(von) && tage > 0 {
				t.Errorf("%s nach %d Tagen: zu weit gesprungen (%s > %s)", wdh, tage, got, von)
			}
			if got.Before(start) {
				t.Errorf("%s nach %d Tagen: rückwärts gesprungen", wdh, tage)
			}
		}
	}
}

// Ein einmaliger Termin bleibt einmalig, auch wenn er in der Zukunft liegt.
func TestEinmaligerTerminBleibtEinmalig(t *testing.T) {
	s := &Service{loc: time.UTC}
	von := tagAm(2026, time.September, 14)
	bis := von.AddDate(0, 0, 30)

	e := StoredEvent{
		ID:     7,
		Title:  "Zahnarzt",
		Start:  tagAm(2026, time.September, 22),
		End:    tagAm(2026, time.September, 22).Add(time.Hour),
		Repeat: RepeatNone,
	}
	events := s.expandStored(e, von, bis)
	if len(events) != 1 {
		t.Fatalf("erwartet genau einen Termin, waren %d", len(events))
	}
	if events[0].Recurring {
		t.Error("ein einmaliger Termin ist keine Wiederholung")
	}
}

// Wer eine Serie bearbeitet, braucht ihren ersten Termin und nicht den, der
// gerade im Kalender steht. Sonst rutscht ein monatlicher Termin am 31. beim
// Speichern auf den 3.
func TestSerieKenntIhrenStart(t *testing.T) {
	s := &Service{loc: time.UTC}
	start := tagAm(2026, time.January, 31)
	e := StoredEvent{
		ID: 3, Title: "Miete", Start: start, End: start.Add(time.Hour), Repeat: RepeatMonthly,
	}
	events := s.expandStored(e, tagAm(2026, time.September, 1), tagAm(2026, time.December, 31))
	if len(events) == 0 {
		t.Fatal("erwartet Wiederholungen im Herbst")
	}
	for _, ev := range events {
		if ev.SeriesStart == nil || !ev.SeriesStart.Equal(start) {
			t.Fatalf("Serienstart erwartet %s, war %v", start, ev.SeriesStart)
		}
	}

	einmal := StoredEvent{ID: 4, Title: "Zahnarzt", Start: start, End: start.Add(time.Hour), Repeat: RepeatNone}
	got := s.expandStored(einmal, tagAm(2026, time.January, 1), tagAm(2026, time.February, 28))
	if len(got) != 1 || got[0].SeriesStart != nil {
		t.Errorf("ein einmaliger Termin hat keine Serie, war %+v", got)
	}
}
