# Änderungen

Alle nennenswerten Änderungen an diesem Projekt.
Format nach [Keep a Changelog](https://keepachangelog.com/de/1.1.0/),
Versionierung nach [SemVer](https://semver.org/lang/de/).

Eine Version wird **gebündelt** eingetragen, nicht jeder Handgriff einzeln.
Die 1.4.x-Reihe hat an einem einzigen Tag acht Einträge bekommen — das liest
niemand, und es sagt auch nichts.

Deshalb gibt es hier **keine 1.4.5**. Sie stand eine Weile in dieser Datei,
wurde aber nie veröffentlicht: Auf GitHub steht v1.4.4, und der Pi läuft
darauf. Was unter 1.4.5 gesammelt war, ist in 1.5.0 aufgegangen.

## [1.6.0] — 2026-09-10

### Hinzugefügt

- **Arbeitszeiten und Schulzeiten.** Eine neue Kachel *Arbeit & Schule* und
  die Seite dahinter beantworten die Frage, die am Küchentisch gestellt wird:
  ab wann sind alle da.

  Dahinter liegen zwei verschiedene Leben, deshalb zwei Wege, sie einzutragen:

  - **Fester Wochenplan** für alles, was jede Woche gleich ist. Ein
    Stundenplan wird ein- bis zweimal im Jahr angefasst und gilt bis dahin.
  - **Nächste vier Wochen** als Kalenderblatt, für Schichten, die jede Woche
    anders liegen. Vier Wochen am Stück, Uhrzeiten direkt in die Felder, ein
    Knopf übernimmt die Woche darüber. Gespeichert wird alles auf einmal.

  **Ein eingetragener Tag sticht den Wochenplan.** Ein Feiertag hebt den
  Stundenplan für diesen einen Tag auf, ohne ihn zu löschen.

  Jeder pflegt seine eigenen Zeiten, ein Administrator die aller. Am Wandgerät
  wird nur gelesen — im Flur ist „ab 16:30 sind alle da" gerade die nützliche
  Zeile.

- **Ein Zertifikat, dem eure Geräte trauen.** `bash scripts/make-cert.sh`
  legt eine kleine eigene Zertifizierungsstelle an und stellt damit ein
  Zertifikat für eure LAN-Adresse aus. Wird die Stelle einmal pro Gerät
  eingerichtet — das Skript sagt für Android, iPhone, Windows und Linux, wie —,
  verschwindet die Browserwarnung, und **erst dann lässt sich die App wirklich
  installieren**. Traefiks eingebautes Platzhalter-Zertifikat reicht dafür
  nicht: Es trägt die eigene Adresse gar nicht, und ein weggeklickter
  Zertifikatsfehler macht aus einer Seite keine sichere Herkunft.

  Der Schlüssel der Stelle bleibt im Haus und ist von Git ausgenommen.

- **Mehr Verknüpfungen im App-Symbol.** Langes Drücken bietet jetzt auch
  *Musik*, *Diashow* und *Arbeit & Schule*. Sichtbar wird das allerdings erst,
  wenn die App installiert ist — siehe oben.

### Geändert

- **Alle Kacheln haben denselben Rahmen bekommen.** Vorher hatte jede ihren
  eigenen: mal mit Symbol im Titel, mal mit Symbol rechts, mal ganz ohne; mal
  mit Innenabstand, mal ohne; Meldungen und leere Zustände jedes Mal anders
  gebaut. Auf einer Übersicht aus elf Kacheln nebeneinander fällt das auf.

  Jetzt gilt überall: Symbol und Titel links, darunter eine Zeile, die sagt
  wie viel oder was los ist, Knöpfe rechts. Der Inhalt jeder Kachel ist
  unverändert.

- **Die Einkaufsliste klappt ihr Formular auf.** Eingabefeld, Mengenfeld und
  Kategorieauswahl standen dauerhaft da und belegten den grössten Teil der
  Kachel, auch wenn niemand etwas eintragen wollte. Jetzt öffnet ein Plus in
  der Kopfzeile das Formular — wie bei Notizen und Kalender. Nach dem
  Eintragen bleibt es offen: Eine Einkaufsliste füllt man in einem Rutsch.

- **Leere Kacheln erklären sich.** Statt eines blassen Symbols mit zwei Worten
  steht dort jetzt, was hier hingehört und wie es dorthin kommt — mit einem
  Knopf, wo einer weiterhilft.

### Behoben

- **Der Kopfbereich verschob sich auf dem Handy.** Der Wetterblock wurde neben
  die Begrüssung gequetscht statt darunter zu rutschen, und die grosse
  Temperaturzahl schob die Seite dann seitwärts. Ursache war ein `flex-1` ohne
  Basisbreite: So wickelt der Block nie um, er wird nur zusammengedrückt.

- **Die Stundenzahlen unter der Wetterkurve klebten zusammen.** Sechs
  Uhrzeiten in einer handybreiten Zeile ergaben „212301030507" statt sechs
  Zahlen. Auf schmalen Bildschirmen steht jetzt nur noch jede zweite.

- **Die Foto-Kachel zog die ganze Rasterzeile in die Höhe.** Ihre Höhe wuchs
  mit der Breite mit; auf einem breiten Bildschirm hingen die Nachbarkacheln
  dadurch in der Luft. Die Höhe ist jetzt gedeckelt.

- **Hochkantfotos zeigten nur einen Streifen.** In der Kachel wie in der
  Diashow schnitt `object-cover` alles bis auf die Bildmitte weg — man sah
  Bauchnabel und Kinn, aber kein Gesicht. Jetzt steht das ganze Bild da, vor
  einem weichgezeichneten Hintergrund aus sich selbst.

- **Geburtstage aus dem nächsten Jahr fehlten im Countdown.** Die Kachel
  bekam die Termine der nächsten 45 Tage gereicht — ein Geburtstag in acht
  Monaten war darin gar nicht enthalten. Sie holt sich ihr Fenster jetzt
  selbst, über gut ein Jahr, und schreibt bei langen Fristen die Monate dazu:
  „in 250 Tagen · gut 8 Monate".

- **Das Release-Archiv wird strenger geprüft.** Es packt keine Zertifikate
  mehr ein, und die Prüfung auf persönliche Daten sucht jetzt auch nach
  Schlüsseldateien. Die Prüfung auf private Netzadressen läuft wieder und
  meldet auch, wenn sie selbst scheitert — statt still ein leeres Ergebnis zu
  liefern.

- **Die Zeiten-Seite passte nicht auf ein Handy.** Vier Spalten nebeneinander
  ergaben 509 Pixel auf einem 375er Bildschirm, die Seite liess sich seitwärts
  schieben. Die Auswahl der Art rutscht auf schmalen Bildschirmen jetzt in
  eine eigene Zeile.

- **`make pi-verify` konnte auf einem echten Pi nie durchlaufen.** Der
  Rauchtest meldet sich mit der Standard-PIN an, und die ist auf jeder
  benutzten Installation längst geändert. Er brach deshalb mit einem Fehler
  ab, obwohl nichts kaputt war — das gewöhnt einem an, rote Ausgaben zu
  übergehen. Jetzt wird sauber zwischen *übersprungen* und *fehlgeschlagen*
  unterschieden. Mit `SMOKE_PIN=1234 make verify` läuft der volle Satz.

## [1.5.0] — 2026-09-10

### Hinzugefügt

- **Ein MP3-Player für die eigene Sammlung.** Ein Ordner mit Musik wird
  schreibgeschützt eingehängt — ein Verzeichnis auf dem Pi, eine Freigabe vom
  NAS, eine angesteckte Festplatte. Neu sind eine Kachel *Musik* zum Stöbern
  und eine schmale Leiste am unteren Rand, die auf **jeder** Seite sichtbar
  bleibt. Das ist der Punkt: Das Audio-Element steht im Seitenlayout, deshalb
  bricht die Musik nicht ab, wenn jemand auf „Rangliste" tippt. Auch in der
  Diashow läuft sie weiter; dort blendet die Leiste mit den übrigen Knöpfen
  aus und wieder ein.

  Nur lokale Dateien: keine Radiosender, keine Podcasts, kein Plex — dafür
  gibt es die Gerätekachel.

  Zugeschnitten auf eine Sammlung, die zu zwei Dritteln aus Hörspielen
  besteht:

  - **Geblättert wird nach Ordnern**, nicht nach Interpret. Bei tausenden
    Hörspieldateien ist die Ordnerstruktur die Gliederung, nicht das ID3-Feld.
  - **Sortiert wird nach Dateiname.** Ein Hörspiel läuft von Teil 1 bis
    Teil 12. Zufallswiedergabe gibt es, aber nicht als Voreinstellung.
  - **Ein Titel startet seinen ganzen Ordner** ab dieser Stelle — Teil 2 läuft
    danach von selbst weiter.
  - **Der Index liegt in der Datenbank** und wird im Hintergrund aufgebaut.
    Der Serverstart wartet nicht darauf. Ein zweiter Durchlauf öffnet nur noch
    die Dateien, deren Grösse oder Änderungszeit sich geändert hat.
  - **Suchen** über Titel, Interpret, Album und Dateiname.
  - **Sperrbildschirm und Kopfhörer-Knöpfe** funktionieren über die Media
    Session API.
  - **Die Platte darf verschwinden.** Ist der Ordner nicht erreichbar, sagt
    die Kachel das und der Index bleibt stehen, statt sich zu löschen.

  Eingerichtet wird in zwei Schritten, weil zwei verschiedene Dinge
  dahinterstecken: **Welcher Ordner des Rechners hereingereicht wird**, steht
  in der `.env` unter `MUSIC_HOST_DIR` — ein Container sieht nur, was in ihn
  eingehängt wurde, daran ändert keine Weboberfläche etwas. **Welcher Teil
  davon gehört wird**, steht unter *Verwaltung → Musik*: dort lässt sich durch
  die Ordner blättern und einer auswählen, ohne Neustart. Neu einlesen darf
  ebenfalls nur ein Administrator — bei einer grossen Sammlung ist das
  minutenlange Arbeit für die Platte. Der Service Worker nimmt Musik ausdrücklich vom
  Zwischenspeicher aus, sonst füllte sich der Browserspeicher mit Hörspielen.

  Was ich vorher sagen muss: **Ton startet nie von allein.** Browser verbieten
  das. Nach einem Neustart des Wandtablets muss jemand einmal tippen; die
  Leiste sagt es dann auch. Dagegen lässt sich nichts machen.

- **Dateien zum Herunterladen.** Eine neue Kachel *Dateien*: Ein Administrator
  legt dort ab, was die ganze Familie braucht — die Bedienungsanleitung der
  Waschmaschine, den Elternbrief, das Formular fürs Ferienlager. Herunterladen
  darf jeder, auch das Wandgerät im Flur; hochladen und löschen nur ein
  Administrator.

  Bis 100 MB je Datei, 300 MB je Vorgang. Dateitypen sind nicht eingeschränkt,
  dafür wird jede Datei ausschliesslich als Anhang ausgeliefert und nie im
  Browser dargestellt — sonst könnte eine abgelegte HTML-Datei unter der
  Adresse des Dashboards laufen. Wird der Platz auf dem Datenträger knapp,
  sagt es die Kachel.

  Die Dateien liegen in `backend/data/files/` und werden **mitgesichert** —
  im nächtlichen Backup und in `scripts/backup.sh`.

- **Wer arbeitet, muss nicht mehr reihum drankommen.** In der Verwaltung hat
  jede Person jetzt das Häkchen *nimmt an der Reihum-Verteilung teil*. Wer es
  abwählt, steht bei „reihum" nicht mehr im Plan — feste Zuständigkeiten,
  „alle" und „wer mag" bleiben davon unberührt. Aufgaben, die eine
  abgemeldete Person gerade hält, wandern sofort weiter statt erst zur
  nächsten Fälligkeit.

### Geändert

- **Der Familien-Modus ist jetzt das erste Bild im README.** Bisher stand dort
  die persönliche Ansicht mit Namen und Punkteband — also ausgerechnet nicht
  das, was dieses Projekt von anderen Dashboards unterscheidet. Dazu neu: der
  Dialog „Wer war das?" mit den drei Gesichtern.

- **Die Beispielgeräte werden abgeschaltet ausgeliefert.** Sie zeigen auf
  `192.168.1.20`, die Adresse aus der Vorlage. Bisher begrüsste eine frische
  Installation ihren Besitzer deshalb mit drei roten Kacheln. Die Beispiele
  stehen in `backend/config.yaml` jetzt auf `enabled: false`; wer Geräte will,
  trägt seine Adressen ein und schaltet sie an. Fehlt das Feld ganz, gilt ein
  Gerät weiterhin als aktiv — bestehende Installationen merken nichts davon.

### Behoben

- **Grosse Downloads brachen nach 60 Sekunden ab.** Das Zeitlimit für Anfragen
  galt auch für alles, was einen Datenstrom offen hält. Für eine Anfrage ist
  es richtig, für ein Hörspiel von 80 Minuten oder eine Datei von 100 MB über
  schwaches WLAN nicht. Musik und Familiendateien sind jetzt davon ausgenommen
  — beim Server ebenso wie beim Proxy davor, dessen Obergrenze für Uploads
  ausserdem unter der des Backends lag.

- **Eine grüne Kachel konnte ins Leere führen.** Prüf-Adresse und Link zur
  Oberfläche sind zwei Felder, und genau deshalb geraten sie auseinander: Wer
  beim Umzug ins neue Netz nur die Prüfung anpasst, bekommt eine Kachel, die
  „erreichbar" meldet und beim Antippen woanders hinführt. Das Geräteformular
  weist jetzt darauf hin, wenn die beiden Felder auf verschiedene Hosts
  zeigen. Kein Fehler, nur ein Hinweis — es kann gewollt sein.

- **Der Wetterort war nicht mehr einstellbar.** Beim Umbau der Oberfläche in
  1.2.0 ist das kleine Wetter-Symbol dem grossen Wetterblock gewichen — nur
  war das Symbol verlinkt und der Block nicht. Da die Wetterseite in der
  Kopfleiste bewusst nicht auftaucht (sie war ja über das Symbol erreichbar),
  gab es auf einem breiten Bildschirm überhaupt keinen Weg mehr dorthin.
  Der Wetterblock führt jetzt wieder auf die Wetterseite.

## [1.4.4] — 2026-09-09

### Behoben

- **Ein als Wandgerät eingerichtetes Tablet blieb persönlich angemeldet.** Der
  Knopf versprach ein Familiengerät, das Tablet zeigte danach aber weiter
  „Guten Abend, Papa" — und alles, was jemand abhakte, lief auf dessen Konto.
  Genau der Fehler, den der Familien-Modus verhindern soll.

  Ursache war meine Annahme, ein zusätzliches „bitte jetzt abmelden" reiche
  aus. An einem Gerät, das an der Wand hängt, denkt daran niemand. Das
  Einrichten beendet die persönliche Sitzung jetzt selbst und führt direkt
  auf die Übersicht im Familien-Modus.

## [1.4.3] — 2026-09-09

### Geändert

- **Neue Screenshots im README.** Die alten zeigten noch die Fassung mit
  Kästen und harten Rahmen — wer auf die Seite kam, sah etwas anderes als das,
  was er herunterlud. Jetzt zu sehen: die Übersicht auf einem breiten
  Bildschirm mit vier Spalten, dieselbe in drei Spalten, Anmeldung, Rangliste
  und die Einstellungen samt Wahl der Oberfläche.

## [1.4.2] — 2026-09-09

### Geändert

- Die Beschreibung war auf dem Stand der ersten Fassung. README und
  Installationsanleitung führen jetzt auf, was seither dazugekommen ist:
  Zuständigkeiten *alle* und *wer mag*, Wetter mit Regenzeiten, Diashow im
  Vollbild, die beiden Oberflächen und der Familien-Modus fürs Wandtablet —
  samt einer Anleitung, wie man ein Tablet dafür einrichtet.
- Bei den Grenzen steht jetzt offen, dass am Wandgerät ohne PIN abgehakt
  werden kann, und warum das so gewollt ist.
- Die Zahl der Prüfungen in allen Anleitungen auf 40 berichtigt.

## [1.4.1] — 2026-09-09

### Behoben

- Am Wandgerät fehlten die **geteilten Links**. Sie waren zusammen mit den
  persönlichen ausgeblendet, obwohl sie der ganzen Familie gehören und genau
  dorthin passen. Die Kachel zeigt sie jetzt wieder — ohne die Wege zum
  Anlegen und Bearbeiten, die weiterhin eine Anmeldung brauchen.

## [1.4.0] — 2026-09-09

### Neu

- **Familien-Modus für das Wandtablet.** Ein Tablet, das fest an der Wand
  hängt, wird in der Verwaltung einmalig als Wandgerät eingerichtet. Danach
  ist nicht mehr eine Person angemeldet, sondern das Gerät: Es zeigt Aufgaben,
  Einkaufsliste, Termine und Wetter, aber keine persönlichen Daten — keine
  Rangliste, keine Einstellungen, keine eigenen Links.

  Wer eine Aufgabe abhakt, wird kurz gefragt: **„Wer war das?"** Ein Tipp aufs
  eigene Gesicht, keine PIN. Damit landen die Punkte bei dem, der die Arbeit
  gemacht hat — und nicht bei dem, der sich zuletzt angemeldet hatte.

- **Konto-Wechsel ohne Verlust des Wandgeräts.** Wer das Tablet mit aufs Sofa
  nimmt, meldet sich normal an und bekommt seine persönliche Ansicht. Nach dem
  Abmelden fällt das Gerät von allein in den Familien-Modus zurück, statt auf
  dem Anmeldebildschirm stehen zu bleiben. Möglich macht das ein zweiter,
  getrennter Sitzungsschlüssel für das Gerät.

- **Ruhezustand.** Nach fünf Minuten ohne Berührung wechselt ein Wandgerät von
  selbst in die Diashow. Eine Berührung führt zurück in den Familien-Modus.

### Sicherheit

- Im Familien-Modus sind persönliche Wege gesperrt: Profil, PIN, eigene
  Ansicht und eigene Links antworten mit einem klaren Hinweis statt mit Daten.
  Die Verwaltung bleibt Administratoren vorbehalten.
- Bewusste Entscheidung: Wer das Wandtablet in der Hand hält, kann ohne PIN
  Punkte für jedes Familienmitglied buchen. Für ein Gerät im eigenen Flur ist
  das richtig; die Alternative wäre, dass niemand es benutzt.

### Behoben

- Eine einzelne abgewiesene Anfrage warf die ganze Oberfläche auf den
  Anmeldebildschirm. Im Familien-Modus hätte das Wandtablet dadurch eine
  PIN-Abfrage im Flur gezeigt.

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

[1.4.5]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.4.5
[1.4.4]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.4.4
[1.4.3]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.4.3
[1.4.2]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.4.2
[1.4.1]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.4.1
[1.4.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.4.0
[1.3.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.3.0
[1.2.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.2.0
[1.1.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.1.0
[1.0.2]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.0.2
[1.0.1]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.0.1
[1.0.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v1.0.0
