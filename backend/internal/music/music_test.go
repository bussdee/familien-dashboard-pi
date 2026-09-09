package music

import "testing"

// saubererOrdner ist die einzige Stelle, an der ein Pfad von aussen ins Modul
// kommt. Er landet zwar nur in einer SQL-Abfrage mit Platzhalter und nie im
// Dateisystem — aber genau solche Annahmen halten selten ein ganzes
// Programmleben lang.
func TestSaubererOrdnerLaesstNichtsAusbrechen(t *testing.T) {
	faelle := map[string]string{
		"":                    "",
		".":                   "",
		"/":                   "",
		"Hörspiele":           "Hörspiele",
		"/Hörspiele/":         "Hörspiele",
		"../../etc":           "etc",
		"..":                  "",
		"../..":               "",
		"Hörspiele/../../..":  "Hörspiele",
		"Hörspiele/./Folge 1": "Hörspiele/Folge 1",
		`Musik\..\..\etc`:     "Musik/etc",
		"Musik (Kinder)":      "Musik (Kinder)",
		"Album mit 100% Lärm": "Album mit 100% Lärm",
		"  Hörspiele  ":       "Hörspiele",
		"a//b":                "a/b",
	}

	for eingabe, erwartet := range faelle {
		if got := saubererOrdner(eingabe); got != erwartet {
			t.Errorf("saubererOrdner(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}

// Ein Ordner, der "100% Lärm" heisst, würde ohne Maskierung in einer
// LIKE-Abfrage auf alles passen — dann stünden im Ordner "Musik" plötzlich
// auch die Titel aus "Musik (Kinder)".
func TestLikeEscapeEntschaerftPlatzhalter(t *testing.T) {
	faelle := map[string]string{
		"Album mit 100% Lärm": `Album mit 100\% Lärm`,
		"Track_01":            `Track\_01`,
		`Pfad\mit\Backslash`:  `Pfad\\mit\\Backslash`,
		"Hörspiele":           "Hörspiele",
	}
	for eingabe, erwartet := range faelle {
		if got := likeEscape(eingabe); got != erwartet {
			t.Errorf("likeEscape(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}

func TestElternOrdner(t *testing.T) {
	faelle := map[string]string{
		"":                         "",
		"Hörspiele":                "",
		"Hörspiele/Folge 1":        "Hörspiele",
		"Hörspiele/Folge 1/Teil 1": "Hörspiele/Folge 1",
	}
	for eingabe, erwartet := range faelle {
		if got := elternOrdner(eingabe); got != erwartet {
			t.Errorf("elternOrdner(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}

func TestOhneEndung(t *testing.T) {
	faelle := map[string]string{
		"01 - Teil 1.mp3":    "01 - Teil 1",
		"Ohne Endung":        "Ohne Endung",
		"Punkt.im.Namen.m4b": "Punkt.im.Namen",
	}
	for eingabe, erwartet := range faelle {
		if got := ohneEndung(eingabe); got != erwartet {
			t.Errorf("ohneEndung(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}
