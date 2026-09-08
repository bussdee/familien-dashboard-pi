package chores

import (
	"math"
	"time"
)

// Haushaltsaufgaben denkt man in Tagen, nicht in Stunden. Wer den Müll am
// Montag um 18 Uhr rausbringt, ist am Dienstag wieder dran — und zwar den
// ganzen Dienstag, nicht erst ab 18 Uhr abends. Deshalb rechnet hier alles mit
// Kalendertagen in der lokalen Zeitzone, nicht mit exakten Zeitpunkten.

// startOfDay schneidet die Uhrzeit ab.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// dueDate liefert den Tag, an dem eine Aufgabe nach dem Erledigen wieder
// ansteht — Tagesbeginn, damit sie den ganzen Tag über abgehakt werden kann.
func dueDate(done time.Time, intervalDays int) time.Time {
	if intervalDays < 1 {
		intervalDays = 1
	}
	return startOfDay(done.AddDate(0, 0, intervalDays))
}

// daysUntil zählt ganze Kalendertage von heute bis zum Stichtag: negativ heißt
// überfällig, 0 heißt heute fällig.
func daysUntil(now, due time.Time) int {
	diff := startOfDay(due.In(now.Location())).Sub(startOfDay(now))
	// Bei der Zeitumstellung ist ein Tag 23 oder 25 Stunden lang. Gerundet
	// statt abgeschnitten, sonst verschluckt sich die Rechnung zweimal im Jahr.
	return int(math.Round(diff.Hours() / 24))
}

// isDue beantwortet die eine Frage, um die es beim Abhaken geht: Ist die
// Aufgabe jetzt dran? Ohne Stichtag wurde sie noch nie erledigt und ist offen.
func isDue(now time.Time, due *time.Time) bool {
	if due == nil {
		return true
	}
	return daysUntil(now, *due) <= 0
}
