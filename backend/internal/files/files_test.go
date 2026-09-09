package files

import "testing"

// sicherName ist die Pfadprüfung beim Herunterladen und Löschen. Kommt hier
// etwas durch, liest oder löscht das Dashboard Dateien ausserhalb seiner
// Ablage.
func TestSicherNameBleibtImOrdner(t *testing.T) {
	faelle := map[string]string{
		"Anleitung.pdf":           "Anleitung.pdf",
		"../../etc/passwd":        "passwd",
		"/etc/passwd":             "passwd",
		"..":                      "",
		".":                       "",
		"":                        "",
		"/":                       "",
		"Unterordner/Datei.txt":   "Datei.txt",
		"Elternbrief Höflich.pdf": "Elternbrief Höflich.pdf",
	}
	for eingabe, erwartet := range faelle {
		if got := sicherName(eingabe); got != erwartet {
			t.Errorf("sicherName(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}

// ablageName behält den Namen lesbar — wer eine Anleitung sucht, sucht nach
// ihrem Namen — wirft aber alles heraus, was den Ordner verlassen könnte.
func TestAblageNameBleibtLesbarUndHarmlos(t *testing.T) {
	faelle := map[string]string{
		"Anleitung.pdf":           "Anleitung.pdf",
		"../../boese.sh":          "boese.sh",
		"/etc/passwd":             "passwd",
		"Elternbrief Höflich.pdf": "Elternbrief Höflich.pdf",
		// Klammern werden zu Unterstrichen, der am Ende fällt dann weg.
		"Formular (2026).pdf": "Formular _2026.pdf",
		"..":                  "datei",
		"":                    "datei",
	}
	for eingabe, erwartet := range faelle {
		if got := ablageName(eingabe); got != erwartet {
			t.Errorf("ablageName(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}

// Der Dateiname landet in einem Content-Disposition-Header. Ein
// Anführungszeichen oder ein Zeilenumbruch darin würde den Header zerreissen.
func TestSchlichterNameZerreisstKeinenHeader(t *testing.T) {
	faelle := map[string]string{
		"Anleitung.pdf":            "Anleitung.pdf",
		`Datei"mit"Anfuehrung.pdf`: "Datei_mit_Anfuehrung.pdf",
		"Zeile\r\numbruch.pdf":     "Zeile__umbruch.pdf",
		"Höflich.pdf":              "H_flich.pdf",
		`Backslash\weg.pdf`:        "Backslash_weg.pdf",
		"":                         "datei",
	}
	for eingabe, erwartet := range faelle {
		if got := schlichterName(eingabe); got != erwartet {
			t.Errorf("schlichterName(%q) = %q, erwartet %q", eingabe, got, erwartet)
		}
	}
}
