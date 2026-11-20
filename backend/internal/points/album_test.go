package points

import "testing"

// Das Sammelalbum: Jede Kennung kommt einmal vor, geheime Abzeichen verraten
// sich erst, wenn man sie hat.
func TestAlbumUndGeheimeAbzeichen(t *testing.T) {
	ids := map[string]bool{}
	for _, d := range katalog() {
		if ids[d.ID] {
			t.Errorf("Kennung doppelt: %s", d.ID)
		}
		ids[d.ID] = true
	}

	sc := Score{ID: 3, ChoreCount: 1, Level: 1}
	got := badgesFor(sc, extras{bonusBeste: 7, frueh: true}, 0)
	hat := map[string]bool{}
	for _, b := range got {
		hat[b.ID] = true
	}
	for _, id := range []string{"first-chore", "serie-7", "fruehaufsteher"} {
		if !hat[id] {
			t.Errorf("%s fehlt in %v", id, got)
		}
	}
	if hat["serie-14"] || hat["post-5"] {
		t.Errorf("zu viele Abzeichen: %v", got)
	}
}
