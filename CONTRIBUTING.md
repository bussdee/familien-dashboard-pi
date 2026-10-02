# Mitmachen

Schön, dass du hier bist. Das Familien Dashboard ist ein kleines Projekt
mit einem klaren Zweck: den Alltag von Eltern mit Kindern leichter machen,
ohne dass jemand dafür Technik lernen muss.

## Der Maßstab für jede Änderung

> Kann meine Mutter das benutzen, ohne zu fragen?
> Kann jemand ohne Vorwissen das installieren?

Wenn eine Änderung eine dieser beiden Antworten von Ja auf Nein dreht,
hat sie es schwer — egal wie elegant sie technisch ist.

Deshalb gilt auch: **keine Cloud, keine Konten, keine Telemetrie.** Die
Daten einer Familie bleiben auf dem Gerät im Flur.

## Erst reden, dann bauen

Für Tippfehler, kaputte Links und offensichtliche Fehler: einfach einen
Pull Request aufmachen.

Für alles Größere lieber vorher ein Issue. Es wäre schade um deine Zeit,
wenn eine fertige Funktion an der Ausrichtung des Projekts scheitert.

## Umgebung einrichten

Du brauchst nur Docker. Go und Node laufen in Containern.

```bash
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup        # legt .env mit zufälligem Schlüssel an
make dev          # Entwicklungsmodus mit automatischem Neuladen
```

Das Dashboard liegt dann auf <http://localhost:8088>. Alle Benutzer haben
die PIN `1234`.

## Vor dem Pull Request

```bash
make check        # Backend baut und besteht go vet, Frontend typprüft
make up           # produktionsnaher Stack
make verify       # 74 Prüfungen gegen den laufenden Stack
```

Beides muss grün sein. Wenn du an der Oberfläche gearbeitet hast, sieh es
dir bitte zusätzlich auf einem Handy an — das halbe Projekt wird auf
kleinen Bildschirmen bedient.

## Wie der Code aussehen soll

- **Deutsch im Code der Oberfläche**, deutsch in den Kommentaren. Bezeichner
  im Code bleiben englisch, das ist in Go und TypeScript üblich. Jeder Text,
  den jemand liest, läuft durch `t()` — siehe [Übersetzungen](#übersetzungen).
- **Kommentare erklären das Warum**, nicht das Was. Was der Code tut,
  steht im Code.
- Go: `gofmt`, Fehler werden behandelt und nicht verschluckt.
- Svelte 5 mit Runes (`$state`, `$derived`, `$props`). Keine alten Stores
  in neuem Code.
- Tailwind mit den Farbtokens aus `app.css`, keine festen Hex-Werte.
- Shell-Skripte müssen `shellcheck -S warning` bestehen.

## Übersetzungen

Die Oberfläche gibt es auf Deutsch und Englisch, umgeschaltet wird pro Gerät
unter *Einstellungen → Sprache* oder im Menü. Der deutsche Text ist der
Schlüssel:

```svelte
<script lang="ts">
  import { t } from '$lib/i18n';
</script>

<h2>{t('Aufgaben')}</h2>
<p>{t('Noch {0} Punkte bis {1}', [rest, ziel])}</p>
```

- **Ganze Sätze, keine Bausteine.** `t('Wieder {0}', [tag])` statt
  `t('Wieder ') + tag` — im Englischen steht das Wort oft woanders.
- **Platzhalter** sind `{0}`, `{1}` (Liste) oder `{name}` (Objekt). Sie
  müssen in der Übersetzung erhalten bleiben.
- **Die englische Fassung** steht in `frontend/src/lib/i18n/en.ts`,
  alphabetisch sortiert. Fehlt ein Eintrag, erscheint der deutsche Text.
- **Meldungen des Servers** bleiben im Go-Code deutsch und werden im Browser
  über `tServer()` übersetzt — feste Texte über `en.ts`, solche mit Zahlen
  oder Namen über die Muster in `frontend/src/lib/i18n/index.ts`.
- **Daten bleiben, wie sie sind.** Was in der Datenbank steht (Kategorien
  der Einkaufsliste, Namen von Aufgaben), wird nur in der Anzeige übersetzt
  oder gar nicht.
- **Datumsangaben** mit `dfLocale` (date-fns) und `intlLocale`
  (`toLocaleDateString`), nie fest `de`.

`make check` (genauer: `npm run check`) ruft `scripts/i18n-check.mjs` auf.
Das Skript sammelt jedes `t('…')` im Frontend und die festen Meldungen aus
`backend/internal` und bricht ab, wenn eine Übersetzung fehlt:

```bash
cd frontend
node scripts/i18n-check.mjs              # prüfen
node scripts/i18n-check.mjs --liste      # fehlende als Vorlage für en.ts
node scripts/i18n-check.mjs --unbenutzt  # Einträge, die niemand mehr abruft
```

Eine weitere Sprache braucht eine Datei neben `en.ts`, einen Eintrag in
`SPRACHEN` und die passende date-fns-Locale in `index.ts`.

## Was nie ins Repository gehört

`.env`, die Datenbank, Notizen, Fotos, Kalenderdateien, echte Namen aus
deinem Haushalt und die IP-Adresse aus deinem Netz. Die `.gitignore` fängt
das meiste ab — der Blick in `git diff` vor dem Commit fängt den Rest.

## Lizenz

Was du beisteuerst, steht unter der [MIT-Lizenz](LICENSE) wie der Rest.
