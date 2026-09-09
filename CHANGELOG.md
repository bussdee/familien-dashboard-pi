# Änderungen

Alle nennenswerten Änderungen an diesem Projekt.
Format nach [Keep a Changelog](https://keepachangelog.com/de/1.1.0/),
Versionierung nach [SemVer](https://semver.org/lang/de/).

## [1.3.0] — 2026-09-09

### Neu

- **Diashow im Vollbild.** Der Knopf im Foto-Rahmen öffnet die Bilder
  formatfüllend, dazu nur Uhrzeit, Wetter und was heute noch ansteht. Die
  Bedienung erscheint bei Berührung und verschwindet von allein wieder;
  Leertaste hält an, Pfeiltasten blättern, Escape beendet. Gedacht für das
  Tablet an der Wand, wenn gerade niemand etwas eintragen will.

### Geändert

- Die Unterseiten (Rangliste, Links, Einstellungen, Verwaltung) sprechen jetzt
  dieselbe Sprache wie die Übersicht: Haarlinie statt kräftigem Rahmen, kein
  Schlagschatten. Bei der Oberfläche *Glas* werden auch sie zu Scheiben.

### Behoben

- Auf hellem Grund waren die Titel überfälliger Aufgaben unsichtbar — sie
  hatten eine weiße Schriftfarbe geerbt.

## [1.2.0] — 2026-09-09

### Neu

- **Die Übersicht ist neu gestaltet.** Statt gleich großer Kästen, die auf
  verschiedenen Höhen enden, stehen die Fenster jetzt als Flächen
  nebeneinander, getrennt durch feine Linien. Eigene Schriften (Fraunces für
  große Zahlen und Namen, Manrope daneben), ein weicher Lichtschein im
  Hintergrund. Gedacht für ein Tablet, das im Flur an der Wand hängt.
- **Zwei Oberflächen zur Wahl** in den Einstellungen: *Nachtlicht* (offen, mit
  feinen Linien) und *Glas* (die Fenster als milchige Scheiben). Beide
  funktionieren hell wie dunkel; hell und Glas ist die freundlichste
  Kombination.
- **Wetter mit Regenzeiten.** Neu sind Stundenwerte als Kurve und ein klarer
  Satz statt einer Prozentzahl: „Regen ab 18:30" oder „Trocken bis 23:00".
  Grundlage sind Viertelstundenwerte, wo Open-Meteo sie liefert — für
  Mitteleuropa aus dem DWD-Modell. Die Stundenwerte allein hätten den
  Regenbeginn um bis zu einer Stunde verfehlt.
- **Der Punktestand steht jetzt quer über der Seite** statt klein in der
  Aufgaben-Kachel: Punkte, Level, Fortschritt, Serie und Abzeichen. Aus zwei
  Metern Entfernung lesbar.

### Geändert

- Die Übersicht nutzt die **volle Bildschirmbreite**; ab 1536 px kommt eine
  vierte Spalte dazu, statt drei Spalten in die Länge zu ziehen.
- Die Trennlinien sitzen nach Reihenposition, nicht nach „alle außer dem
  ersten". Fenster ausblenden und umsortieren funktioniert dadurch weiterhin,
  ohne dass eine Linie ins Leere zeigt.
- Das Formular zum Anlegen einer Aufgabe öffnet sich als Fenster über der
  Seite. Vorher wuchs die Kachel dabei um die halbe Höhe.
- Schriften liegen im Projekt (163 KB) und werden nicht von Google nachgeladen
   — das Dashboard bleibt ohne Internet vollständig.

## [1.1.0] — 2026-09-09

### Neu

- **Zuständigkeit „Alle" und „Wer mag".** Bisher rotierte eine Aufgabe reihum
  oder gehörte einer festen Person. Wer wenig Zeit für den Haushalt hat, stand
  dadurch trotzdem überall im Plan. Jetzt gibt es vier Möglichkeiten:
  *Reihum*, *Alle*, *Wer mag* und eine feste Person. Bei „Alle" und „Wer mag"
  steht kein Name mehr an der Aufgabe, und die Weitergabe reihum überspringt
  sie.

### Geändert

- Im Formular hat die Zuständigkeit eine eigene Zeile bekommen — in drei
  Spalten war das Auswahlfeld so schmal, dass „Reihum" abgeschnitten wurde.
- Bestehende Aufgaben werden beim ersten Start übernommen: was reihum lief,
  läuft weiter reihum; was einer Person gehörte, bleibt bei ihr.

## [1.0.2] — 2026-09-08

### Behoben

- **Dieselbe Aufgabe konnte mehrfach abgehakt werden — jedes Mal mit vollen
  Punkten.** Hatte das Kind den Müll rausgebracht, konnten Mama und Papa
  danach denselben Müllsack abhaken und bekamen dafür ebenfalls Punkte. Eine
  erledigte Aufgabe ist jetzt bis zu ihrem nächsten Stichtag geschlossen; die
  Oberfläche bietet das Abhaken gar nicht mehr an und zeigt stattdessen, wer
  dran war.
- **Stichtage sind jetzt Tage, keine Uhrzeiten.** Wer eine tägliche Aufgabe
  abends um 22 Uhr erledigt hat, war vorher erst am Folgetag um 22 Uhr wieder
  dran. Jetzt gilt der ganze nächste Tag.
- **Eine neu angelegte Aufgabe steht sofort an** statt erst nach einem vollen
  Intervall.
- **Überfällig** heißt jetzt „der Stichtag ist vorbei" und nicht mehr „der
  Zeitpunkt ist ein paar Stunden her".
- Die Weitergabe reihum verglich Zeitstempel als Text. Weil in der Datenbank
  zwei verschiedene Textformate stehen, konnte der Vergleich danebengreifen;
  entschieden wird das jetzt im Programm.

### Neu

- Erste automatische Tests im Backend, die genau diese Fälle festhalten
- Der Rauchtest prüft mit, dass ein zweites Abhaken abgelehnt wird

## [1.0.1] — 2026-09-08

### Behoben

- **Das Backend startete nicht, wenn der eigene Benutzer nicht die Kennung 1000
  hat.** SQLite konnte die Datenbank in `backend/data` nicht anlegen und meldete
  irreführend `unable to open database file: out of memory (14)`. Betraf unter
  anderem viele NAS-Konten und zweite Benutzer eines Systems. Die Kennung kommt
  jetzt aus `PUID`/`PGID` in der `.env`, `make setup` trägt die eigene ein.

### Geändert

- Go-Werkzeugkette auf 1.25, Abhängigkeiten aktualisiert — darunter
  `golang-jwt` (Anmeldung) und `gorilla/websocket` (Einkaufsliste in Echtzeit)
- GitHub Actions aktualisiert
- Dependabot zurückhaltender eingestellt: monatlich, gebündelt, keine
  Hauptversionssprünge

## [1.0.0] — 2026-09-08

Erste öffentliche Fassung.

### Enthalten

**Aufgaben und Punkte**
- Wiederkehrende Aufgaben mit Intervall, Punktwert und rotierender Zuständigkeit
- Gemeinsames Punktesystem für Aufgaben und Einkäufe (`point_events`)
- Level alle 100 Punkte mit Namen von Neuling bis Legende, Fortschrittsbalken
- Serien über aufeinanderfolgende aktive Tage
- Elf Abzeichen, Siegertreppchen, Wochenwertung, geteilte Plätze bei Gleichstand
- Punkte fürs Einkaufen: 15 Grundpunkte plus 2 je Artikel, gedeckelt bei 80
- Adminbereich: Verlauf, einzelne Einträge zurücknehmen, manuell buchen,
  Punktestände zurücksetzen. Das Zurücknehmen einer Aufgabe macht sie wieder
  fällig, statt nur den Punkt zu streichen.

**Listen und Inhalte**
- Einkaufsliste mit Live-Sync über WebSocket
- Kalender: eigene Termine (einmalig, wöchentlich, monatlich, jährlich) plus
  `.ics`-Import mit RRULE- und EXDATE-Auswertung
- Notizen als Markdown, zweiseitig mit dem Dateisystem synchronisiert
- Links mit Kategorien, Anpinnen auf die Startseite, Teilen mit der Familie
- Foto-Rahmen mit Upload per Drag & Drop, Prüfung nach Dateiinhalt

**Umgebung**
- Wetter über Open-Meteo mit Ortssuche, ohne API-Schlüssel; Zwischenspeicher in
  der Datenbank für den Offline-Fall
- Geräte-Health-Checks über HTTP oder TCP, im Adminbereich konfigurierbar,
  mit Verbindungstest vor dem Speichern
- Countdowns, automatisch aus Kalendereinträgen abgeleitet

**Benutzer und Oberfläche**
- PIN-Anmeldung mit argon2id, Sperre nach fünf Fehlversuchen
- Eigenes Profil: Name, Avatar, Farbe, PIN
- Rollen Administrator und Familienmitglied
- Fenster pro Person anordnen und ausblenden
- Heller und dunkler Modus, dem System folgend
- Schubladenmenü auf dem Handy, Navigationsleiste am Rechner
- Eigener Bestätigungsdialog statt `window.confirm()` — letzteres wird von
  manchen Browsern unterdrückt und liefert dann stumm `false`

**Technik**
- Go-Backend mit Chi, SQLite über `modernc.org/sqlite` (kein CGO)
- SvelteKit-Frontend als SPA mit `adapter-static`
- Traefik nur über Datei-Provider, ohne Zugriff auf den Docker-Socket
- Container als non-root mit read-only Dateisystem
- Nächtliche Sicherung per SQLite `VACUUM INTO`, sieben Tage Aufbewahrung
- PWA: Service Worker mit Offline-Ansicht, Installations-Vorschlag,
  Update-Benachrichtigung
- 35 automatische Prüfungen über `make verify`

### Bekannte Einschränkungen

- Oberfläche nur auf Deutsch
- Offline-Modus und Installation brauchen HTTPS; über `http://` funktioniert
  beides nur auf `localhost`
- Fenster lassen sich per Pfeiltasten sortieren, nicht per Drag & Drop
- HEIC-Bilder von iPhones werden beim direkten Upload nicht unterstützt
- Zweiwöchentliche Termine gehen nur über eine `.ics` mit `INTERVAL=2`
- Ein Punkt lässt sich nicht vom Benutzer selbst zurücknehmen, nur von einem
  Administrator

[1.3.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.3.0
[1.2.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.2.0
[1.1.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.1.0
[1.0.2]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.0.2
[1.0.1]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.0.1
[1.0.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.0.0
