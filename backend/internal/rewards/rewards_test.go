package rewards

import "testing"

func TestPruefeSetztStandardSymbol(t *testing.T) {
	p := rewardPayload{Title: "  Ein Eis ", Cost: 80}
	if msg := pruefe(&p); msg != "" {
		t.Fatalf("unerwartet abgelehnt: %s", msg)
	}
	if p.Title != "Ein Eis" || p.Emoji != "🎁" {
		t.Errorf("nicht bereinigt: %q %q", p.Title, p.Emoji)
	}
}

// Eine Belohnung für null Punkte wäre ein Geschenk, keine Belohnung — und
// ein negativer Preis würde das Guthaben beim Einlösen erhöhen.
func TestPreisMussPositivSein(t *testing.T) {
	for _, cost := range []int{0, -5, maxCost + 1} {
		p := rewardPayload{Title: "Kino", Cost: cost}
		if pruefe(&p) == "" {
			t.Errorf("Preis %d hätte abgelehnt werden müssen", cost)
		}
	}
}

func TestOhneTitelWirdAbgelehnt(t *testing.T) {
	p := rewardPayload{Title: " ", Cost: 10}
	if pruefe(&p) == "" {
		t.Error("leerer Titel hätte abgelehnt werden müssen")
	}
}
