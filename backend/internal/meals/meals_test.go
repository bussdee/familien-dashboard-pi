package meals

import (
	"reflect"
	"testing"
)

// „Tomaten, Zwiebeln" in einer Zeile ist genauso eine Liste wie zwei Zeilen.
// Doppelte Zutaten fallen heraus, unabhängig von der Schreibweise.
func TestZutatenWerdenAufgeteiltUndEntdoppelt(t *testing.T) {
	m, msg := normalize("2026-09-28", mealPayload{
		Title:       "  Spaghetti Bolognese ",
		Ingredients: []string{"Tomaten, Zwiebeln", " Hackfleisch ", "", "tomaten", "Spaghetti"},
	})
	if msg != "" {
		t.Fatalf("unerwartet abgelehnt: %s", msg)
	}
	if m.Title != "Spaghetti Bolognese" {
		t.Errorf("Titel nicht bereinigt: %q", m.Title)
	}
	want := []string{"Tomaten", "Zwiebeln", "Hackfleisch", "Spaghetti"}
	if !reflect.DeepEqual(m.Ingredients, want) {
		t.Errorf("Zutaten: %v, erwartet %v", m.Ingredients, want)
	}
}

func TestOhneTitelWirdAbgelehnt(t *testing.T) {
	if _, msg := normalize("2026-09-28", mealPayload{Title: "   "}); msg == "" {
		t.Error("leerer Titel hätte abgelehnt werden müssen")
	}
}

func TestOhneZutatenIstErlaubt(t *testing.T) {
	m, msg := normalize("2026-09-28", mealPayload{Title: "Reste"})
	if msg != "" || len(m.Ingredients) != 0 || m.Ingredients == nil {
		t.Errorf("Reste ohne Zutaten: %v %q", m.Ingredients, msg)
	}
}

func TestSplitIngredientsUeberspringtLeereZeilen(t *testing.T) {
	got := splitIngredients("Milch\n\n  Mehl \n")
	if !reflect.DeepEqual(got, []string{"Milch", "Mehl"}) {
		t.Errorf("erwartet [Milch Mehl], war %v", got)
	}
}
