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

## [2.0.1] — 2026-09-28

Ein Fehler aus dem Alltag: Der Geburtstag stand im Countdown, war aber nirgends
zu ändern oder zu löschen.

### Behoben

- **Countdown-Einträge liessen sich weder bearbeiten noch löschen.** Der
  Countdown hat keine eigenen Einträge, er filtert Kalendertermine nach
  Stichwörtern wie *Geburtstag* oder *Ferien*. Er holt sich dafür ein Fenster
  von 400 Tagen, damit ein Geburtstag im August überhaupt auftaucht. Die
  Kalender-Kachel zeigt dagegen nur die nächsten 45 Tage — und nur acht Tage
  mit Terminen. Der Termin war also im Countdown zu sehen und im Kalender, dem
  einzigen Ort mit *Bearbeiten* und *Löschen*, nicht. Was man sah, liess sich
  nicht anfassen; was man anfassen konnte, sah man nicht.

  Ein Tipp auf eine Countdown-Zeile klappt jetzt *Bearbeiten* und *Löschen*
  auf, wie im Kalender. Termine aus einer `.ics`-Datei bleiben schreibgeschützt
  und sagen, woran das liegt, statt einen Knopf zu zeigen, der nichts tut. Wird
  im Kalender etwas geändert, zieht der Countdown sofort nach.

- **Beim Bearbeiten einer Wiederholung wurde das Startdatum verschoben.** Im
  Formular stand der Tag, der gerade angezeigt wurde, nicht der Beginn der
  Serie. Wer bei einem jährlichen Geburtstag nur den Namen änderte, machte aus
  dem 14.08.2026 ein 14.08.2027. Bei einem monatlichen Termin am 31. wäre der
  Tag beim Speichern dauerhaft auf den 3. gerutscht. Der Server liefert den
  Serienstart jetzt mit, und das Formular nimmt ihn.

### Geändert

- **Das Termin-Formular öffnet als Fenster** statt oben in der Kalender-Kachel,
  wie bei den Aufgaben. Ein Termin, der aus dem Countdown heraus bearbeitet
  wird, öffnet sonst irgendwo oben auf der Seite, wo man es nicht sieht. Das
  Plus im Kalender und das Plus unten in der Leiste öffnen dasselbe Fenster.

## [2.0.0] — 2026-09-27

Die erste Fassung, die nicht nur etwas anzeigt, sondern den Tag mitorganisiert:
Was gibt's zu essen, was fehlt dafür, und wofür sind die Punkte eigentlich da.
Dazu eine Bedienung, die auf dem Handy dort liegt, wo der Daumen ist.

Bestehende Installationen übernehmen alles ohne Zutun. Die neuen Tabellen legt
das Backend beim ersten Start selbst an; Ansicht, Punkte und Einstellungen
bleiben, wie sie sind.

### Hinzugefügt

- **Essensplan.** Eine Woche, Montag bis Sonntag, ein Gericht pro Tag — die
  Antwort auf die meistgestellte Frage in einer Küche. Zu jedem Gericht lassen
  sich eine Notiz („Oma kommt") und die Zutaten eintragen, eine pro Zeile oder
  mit Komma getrennt.

  Der eigentliche Gewinn steckt im Knopf daneben: **Die Zutaten wandern mit
  einem Tipp auf die Einkaufsliste.** Was dort schon offen steht, kommt nicht
  doppelt dazu, und die Kategorie kommt aus dem, was die Familie früher schon
  eingekauft hat — die Tomaten landen wieder bei Obst & Gemüse.

  Familien kochen im Kreis, und das ist gut so. Deshalb stehen unter dem
  Eingabefeld die Gerichte der letzten Monate; ein Tipp übernimmt Name und
  Zutaten von damals.

  Eintragen darf jeder, auch das Tablet in der Küche: Dort wird der Plan
  meistens gemacht. Auf der Übersicht gibt es eine eigene Kachel mit heute und
  den nächsten drei Tagen.

- **Belohnungen.** Eine Zahl, die nur steigt, trägt bei einem Kind etwa drei
  Wochen. Dann kommt die Frage: „Und was krieg ich dafür?" Die Antwort legen
  jetzt die Eltern fest — 30 Minuten Bildschirmzeit, ein Eis, den Film am
  Familienabend —, jeweils mit einem Preis in Punkten.

  Ein Kind löst selbst ein, ein Elternteil bestätigt, wenn es so weit ist
  (oder lehnt ab, dann sind die Punkte wieder da). Zweistufig, weil „Ein Eis"
  sonst als erledigt dastünde, während die Gefriertruhe leer ist.

  **Level und Rangliste bleiben davon unberührt.** Ausgegeben wird ein
  Guthaben — Punkte minus Einlösungen. Wer sich ein Eis holt, fällt nicht auf
  Platz drei zurück. Das Guthaben steht im Punkteband auf der Übersicht.

  Einlösen braucht eine persönliche Anmeldung. Am Wandgerät ist das Angebot
  zu sehen, aber es gibt keinen Knopf: Ohne PIN könnte dort jeder das
  Guthaben der Schwester ausgeben. Vier Beispiele sind angelegt und lassen
  sich ändern oder löschen; wer sie löscht, bekommt sie nicht wieder.

- **Einkaufen als eigene Seite.** Auf der Übersicht teilt sich die Liste den
  Platz mit zehn anderen Kacheln. Im Laden braucht man das Gegenteil: die
  ganze Liste, grosse Zeilen, **nach Abteilung sortiert** — und einen
  Bildschirm, der nicht alle dreissig Sekunden dunkel wird. *Bildschirm an*
  hält ihn wach, wo der Browser das erlaubt.

- **Häufig gekauft.** Unter dem Eingabefeld der Einkaufsliste stehen die
  Artikel, die die Familie oft kauft, als Chips zum Antippen. Was schon auf
  der Liste steht, fehlt dort. Was lange niemand gekauft hat, rutscht nach
  hinten, damit die Sonnencreme im November nicht oben steht. Tippfehler
  verschwinden mit dem kleinen × daneben.

- **Löschen mit Rückgängig.** Ein Eintrag auf der Einkaufsliste verschwindet
  ohne Rückfrage — im Laden hält ein Dialog nur auf. Dafür steht unten einige
  Sekunden lang *Rückgängig*.

- **Die Heute-Leiste.** Unter der Begrüssung stand bisher eine Textzeile mit
  zwei Zahlen. Jetzt stehen dort fünf Antworten, die sonst fünf Fragen wären:
  der nächste Termin, das Essen, die offenen Aufgaben, die Einkaufsliste, und
  ab wann alle zu Hause sind. Jede Karte führt dorthin, wo man etwas daran
  ändern kann.

- **Akzentfarben.** Petrol, Blau, Violett, Rose, Orange oder Grün — pro Gerät,
  wie hell und dunkel. Jede Farbe hat einen eigenen Wert für die dunkle
  Oberfläche, weil das Blau, das auf Weiss gut lesbar ist, auf Nachtblau
  versinkt. Auch die Statusleiste des Handys nimmt sie an. Umschalten geht in
  den Einstellungen oder direkt im Menü.

### Geändert

- **Eine Leiste am unteren Rand, auf Handy und Tablet im Hochformat.** Bis
  1.7 gab es dort nur den Menüknopf oben links — genau die Ecke, die der
  Daumen nicht erreicht, und im Laden mit dem Korb in der anderen Hand die
  falscheste. Jetzt liegen Start, Einkauf und Essen unten, dazu *Mehr* für
  alles andere.

- **Ein Plus für alles.** In der Mitte der Leiste (am grossen Bildschirm
  rechts unten, oder mit der Taste **N**) öffnet sich ein Blatt mit den
  häufigsten Handgriffen. Die Einkaufsliste wird direkt dort befüllt, samt
  Vorschlägen. Termin, Notiz und Aufgabe springen zur passenden Kachel und
  öffnen deren Formular — jedes Formular gibt es weiterhin genau einmal.

- **Die Kopfleiste zeigt am grossen Bildschirm sieben Ziele** statt fünf:
  Einkaufen, Essensplan und Belohnungen sind dazugekommen. Damit sie in 1024
  Pixel passen, stehen dort zwischen lg und xl nur die Symbole; die
  Beschriftung erscheint als Tooltip. Die Ziele stehen jetzt an einer Stelle
  im Code, statt in Kopfleiste und Menü getrennt zu altern.

- **Jede Kachel hat ihre Farbe.** Das Symbol sitzt in einem getönten Feld —
  Aufgaben bernstein, Einkaufen grün, Kalender blau. Aus drei Metern erkennt
  man eine Kachel an der Farbe, bevor man den Titel lesen kann. Die Fenster
  haben dazu einen Hauch Tiefe bekommen, nicht mehr: Nachts an der Wand soll
  nichts leuchten.

- **Fenster lassen sich ziehen.** Unter *Ansicht anpassen* hat jede Zeile
  einen Griff — mit Finger und Maus gleichermassen. Die Pfeile bleiben, für
  alle, die lieber tippen oder die Tastatur benutzen. Damit fällt eine der
  „Grenzen" aus dem README weg.

- **Neue Kacheln erscheinen an ihrem Platz.** Wer seine Übersicht schon
  sortiert hatte, bekam eine neue Kachel bisher ganz unten angehängt. Sie
  landet jetzt hinter ihrem Vorgänger aus der Standardreihenfolge — der
  Essensplan also neben dem Kalender.

- **Was schon auf der Einkaufsliste steht, kommt nicht doppelt dazu.** Zwei
  Leute, die unabhängig voneinander „Milch" eintragen, meinen dieselbe Milch.
  Statt einer zweiten Zeile sagt die App jetzt: *steht schon auf der Liste*.
  Gross- und Kleinschreibung spielen dabei keine Rolle.

- **„Einkauf abschliessen" verläuft in der Akzentfarbe.** Bisher lief der
  Knopf fest nach Grün aus — mit Violett oder Rose sah das nach einem Fehler
  aus.

- **Neue Bilder im README**, aufgenommen mit erfundenen Daten: der
  Familien-Modus, die Handy-Ansichten, Essensplan und Belohnungen.

- **Der Rauchtest prüft 64 Dinge statt 55:** die drei neuen Seiten, die neuen
  Endpunkte und einen Eintrag im Essensplan, den er danach wieder entfernt.
  Seine Testeinträge räumt er jetzt auch aus den Einkaufs-Vorschlägen.

## [1.7.0] — 2026-09-14

### Hinzugefügt

- **Trockenfenster.** Die Wetterfrage in dieser Familie lautet nicht „wie
  warm wird es", sondern *wann können wir raus*. Darauf antwortet keine
  Prozentzahl, sondern ein Zeitraum.

  Das Dashboard nennt jetzt das nächste Fenster im Klartext — *„Trocken
  morgen 06:50 bis 19:33 · 12,5 Std."* — und die Wetterseite listet die
  Fenster der ganzen Vorhersage mit Länge, Bewölkung und gefühlter
  Temperatur. Darunter steht der Verlauf Stunde für Stunde; die nassen
  Stunden sind hinterlegt, die hellen Lücken dazwischen sind die Fenster.

  Gerechnet wird nur zwischen Sonnenauf- und Sonnenuntergang: Ein trockenes
  Fenster um drei Uhr nachts ist statistisch richtig und praktisch wertlos.
  Als nass gilt eine Stunde ab 0,1 mm **oder** ab 55 % Wahrscheinlichkeit —
  beide Wege zählen, sonst meldete das Dashboard bei „60 %, 0,05 mm"
  trockenes Wetter. Unter anderthalb Stunden ist kein Fenster, sondern eine
  Lücke zwischen zwei Schauern.

  Dadurch steht an einem Tag mit 83 % Regenwahrscheinlichkeit jetzt
  gegebenenfalls *„Trocken 07:00–19:30"*, wenn der Regen in die Nacht fällt.
  Genau dafür ist die Rechnung da.

- **Einmalige Aufgaben.** Eine Aufgabe konnte bisher nur wiederkehren. Für
  „Keller aufräumen" oder „Zahnarzttermin ausmachen" war das eine Erfindung:
  Sie kommt nicht in sieben Tagen wieder, sie ist dann fertig.

  Beim Anlegen gibt es jetzt *Nur einmal*. Eine solche Aufgabe steht ab
  sofort an, lässt sich genau einmal abhaken und bleibt danach
  durchgestrichen in der Liste stehen — am Monatsende will man sehen, was
  tatsächlich geschafft wurde. Ein Knopf räumt alle erledigten auf einmal
  weg.

- **Monatswertung, die stehen bleibt.** Die Gesamtwertung wächst immer
  weiter; wer im März angefangen hat, holt den Vorsprung nie mehr auf. Ein
  Monat dagegen fängt für alle bei null an.

  Auf der Ranglisten-Seite steht jetzt der laufende Monat als Zwischenstand
  und darunter jeder abgeschlossene mit seinem Endergebnis und den Plätzen.
  Abgeschlossene Monate werden einmal festgeschrieben und danach nicht mehr
  angefasst — auch dann nicht, wenn die einmaligen Aufgaben von damals
  längst gelöscht sind. Gleichstand teilt sich einen Platz, und beide Namen
  stehen da.

- **Termine für einzelne Personen.** Ein Termin gehörte bisher immer der
  ganzen Familie; ein Feld für eine Person gab es gar nicht. Gemeldet war
  das als Rechteproblem — es war keins: Beide Administratorinnen konnten
  schon vorher dasselbe, nämlich beide nicht. Jetzt lässt sich zu jedem
  Termin eine Person wählen, ihr Gesicht steht in der Kachel daneben. Aus
  `.ics`-Dateien gelesene Termine haben keine Person; dort bleibt das Feld
  leer, statt eine zu erfinden.

- **Wöchentliche Termine mit festem Wochentag.** *„Immer donnerstags"* ist
  das, was man sagen will — nicht „ab dem 18., und der ist zufällig ein
  Donnerstag". Die Auswahl schiebt das Datum sichtbar auf den nächsten
  passenden Tag vor.

- **Wiederholung „Jeden Tag".** Für die Tablette am Morgen und den Hund am
  Abend.

- **Vollbild und Wachschutz für die Diashow.** Ein Wandtablet, das mitten in
  der Diashow den Bildschirm abschaltet, ist kein Bilderrahmen. Solange die
  Show läuft, bleibt das Display an. Und eine Berührung genügt, um die
  Adressleiste loszuwerden — ohne die App installieren zu müssen. Geräte
  ohne Vollbild-Funktion (iPhone Safari) bekommen statt eines toten Knopfs
  den Weg, der dort funktioniert.

### Behoben

- **Tägliche und wöchentliche Termine aus der Vergangenheit verschwanden.**
  Die Wiederholungen wurden vom Starttag an einzeln abgezählt, bis zur
  Obergrenze von 500. Ein täglicher Termin, der vor zwei Jahren angelegt
  wurde, war damit am Ende, bevor er das heutige Datum erreichte — und
  stand gar nicht mehr im Kalender. Jetzt wird der vergangene Teil
  übersprungen statt durchgezählt.

- **„Regen ab 20:30" konnte den falschen Tag meinen.** Die Uhrzeit stand
  ohne Tag da und las sich wie heute Abend, auch wenn sie morgen Abend
  meinte. Jetzt steht der Tag dabei, sobald es nicht heute ist.

## [1.6.1] — 2026-09-10

### Behoben

- **Nachtschichten wurden als Fehleingabe abgelehnt.** Die Zeitprüfung
  verlangte, dass das Ende nach dem Anfang liegt. Bei 20:00 bis 07:00 tut es
  das nicht. Dahinter steckte eine Annahme, die nirgends geschrieben stand:
  dass ein Block am selben Tag endet. Für Schichtdienst ist das falsch, und
  damit war die Kachel für genau die Person unbrauchbar, für die sie gebaut
  wurde.

  **Endet eine Zeit vor ihrem Anfang, läuft sie jetzt über Mitternacht.** So
  rechnen Dienstpläne seit jeher. Gleiche Zeiten bleiben abgelehnt — 08:00 bis
  08:00 könnte null Stunden heissen oder vierundzwanzig.

  Ein solcher Block steht in **beiden** Tagen der Übersicht: abends im einen,
  morgens im anderen. Und die Zeile „ab wann sind alle da" nennt an einem Tag
  mit Nachtschicht keine Uhrzeit mehr, sondern sagt, dass nicht mehr jeder
  zurückkommt. Vorher hätte sie den Dienstbeginn genannt und ihn als Rückkehr
  ausgegeben.

- **Ein Tag konnte nur eine Zeit haben.** Ein Teildienst von 6 bis 10 und
  wieder von 15 bis 20 Uhr ist im Schichtdienst normal, war aber nicht
  eintragbar: Beim Speichern ersetzte jeder Eintrag den ganzen Tag. Das war
  gegen doppelte Einträge gedacht und machte den zweiten Block unmöglich.

  Jetzt werden zuerst alle genannten Tage geleert und danach alle Blöcke
  geschrieben. Im Vier-Wochen-Raster bekommt jeder Tag einen Knopf
  *„+ zweite Zeit an diesem Tag"*.

### Geändert

- **Die Willkommens-Notiz erklärt jetzt das Zertifikat.** Der Weg zu einem
  Zertifikat, dem die Geräte trauen, stand nur in `INSTALL.md` — wer das
  Projekt startet, schaut aber zuerst ins Dashboard. Die Notiz führt jetzt
  durch alle drei Schritte: erzeugen, über die Kachel *Dateien* verteilen,
  einmal pro Gerät einrichten. Mit dem Satz, der die Verwechslung ausräumt:
  Verteilt wird nur `familie-ca.crt`, und die enthält keinen Schlüssel.

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

- **Die Fenster der Übersicht sind wieder als einzelne Fenster zu erkennen.**
  Seit dem Umbau der Oberfläche in 1.2.0 standen sie offen nebeneinander,
  getrennt nur durch Haarlinien an den Zellkanten. Auf einem Handy mit einer
  Spalte trägt das. Auf einem breiten Monitor mit vier Spalten und Inhalten
  sehr unterschiedlicher Höhe nicht: Die Linien laufen durch, alles
  verschwimmt zu einer Fläche, und man sieht nicht mehr, wo ein Fenster
  aufhört und das nächste anfängt.

  Jedes Fenster ist jetzt wieder ein eigener Block — Abstand rundum, Linie auf
  allen vier Seiten, runde Ecken, ein Hauch eigener Grund. Ruhig genug für
  *Nachtlicht*, aber wieder ein Ding statt Teil eines Feldes.

  Eine Reihe steht dabei auf einer Höhe. Sobald ein Block sichtbare Kanten
  hat, ist seine Unterkante eine Linie, und die darf nicht woanders enden als
  die daneben. Damit daraus keine leeren Kästen werden, füllt der Inhalt die
  Höhe aus und ein leerer Zustand setzt sich in die Mitte.

  Nebenbei fällt eine fragile Stelle weg: Vorher musste pro Bildschirmbreite
  der Reihenanfang von der Linie ausgenommen werden. Da sich die Fenster
  umsortieren und ausblenden lassen, war das eine Rechnung, die bei jeder
  Änderung neu stimmen musste. *Glas* ist dadurch auf einen reinen
  Materialwechsel geschrumpft.

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

- **Der Zertifikatsordner gehörte nach dem ersten Start root.** Weil er nicht
  mit übertragen wird, legte Docker ihn beim Einhängen selbst an — und das
  geschieht als root. Danach konnte `make-cert.sh` dort nichts erzeugen und
  scheiterte an einer nackten `chmod`-Fehlermeldung. Der Ordner wird jetzt vom
  Deploy und von `init-data.sh` vorher angelegt, und das Skript sagt
  verständlich, was zu tun ist, falls er trotzdem einmal fremd gehört.

- **Der Deploy hätte Zertifikate mit übertragen.** Die Ausschlussliste von
  `rsync` kennt `.gitignore` nicht — `traefik/certs/` stand nicht darin und
  wäre samt privater Schlüssel auf das Zielgerät gewandert. Zertifikate
  entstehen beim Einrichten auf dem jeweiligen Gerät und gehören genau
  dorthin.

  Dasselbe gilt für `traefik/dynamic/certs.yml`, die Datei, die Traefik auf
  diese Zertifikate zeigen lässt. Sie muss mit ihnen zusammenbleiben: Wandert
  sie allein auf ein Gerät, sucht Traefik dort Dateien, die es nicht gibt,
  bricht beim Aufbau des Zertifikatspeichers ab und **bedient den HTTPS-Port
  gar nicht mehr**. Beide stehen jetzt in der Liste.

  Ebenfalls neu darin: `.env.*`, damit `--delete` keine Sicherungskopien der
  `.env` auf dem Zielgerät wegräumt.

- **Die Wetterseite stand in keinem Menü.** Sie war nur über den Wetterblock
  auf der Übersicht erreichbar — wer den Ort einstellen wollte, suchte sie
  vergeblich in der Leiste. Jetzt steht sie dort, zusammen mit *Arbeit &
  Schule*. Damit die Leiste nicht über den Rand hinausläuft, klappt sie erst
  ab 1024 Pixeln auf; darunter übernimmt der Menüknopf, der vorher schon bei
  768 verschwand und die Seite dadurch seitwärts schiebbar machte.

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

[2.0.1]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v2.0.1
[2.0.0]: https://github.com/bussdee/familien-dashboard-pi/releases/tag/v2.0.0
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
