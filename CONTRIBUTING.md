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
make verify       # 35 Prüfungen gegen den laufenden Stack
```

Beides muss grün sein. Wenn du an der Oberfläche gearbeitet hast, sieh es
dir bitte zusätzlich auf einem Handy an — das halbe Projekt wird auf
kleinen Bildschirmen bedient.

## Wie der Code aussehen soll

- **Deutsch in der Oberfläche**, deutsch in den Kommentaren. Bezeichner im
  Code bleiben englisch, das ist in Go und TypeScript üblich.
- **Kommentare erklären das Warum**, nicht das Was. Was der Code tut,
  steht im Code.
- Go: `gofmt`, Fehler werden behandelt und nicht verschluckt.
- Svelte 5 mit Runes (`$state`, `$derived`, `$props`). Keine alten Stores
  in neuem Code.
- Tailwind mit den Farbtokens aus `app.css`, keine festen Hex-Werte.
- Shell-Skripte müssen `shellcheck -S warning` bestehen.

## Was nie ins Repository gehört

`.env`, die Datenbank, Notizen, Fotos, Kalenderdateien, echte Namen aus
deinem Haushalt und die IP-Adresse aus deinem Netz. Die `.gitignore` fängt
das meiste ab — der Blick in `git diff` vor dem Commit fängt den Rest.

## Lizenz

Was du beisteuerst, steht unter der [MIT-Lizenz](LICENSE) wie der Rest.
