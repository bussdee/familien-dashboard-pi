# Installation

Vom Download bis zum laufenden Dashboard. Rechne mit 10 Minuten plus Bauzeit.

---

## Was du brauchst

| | |
|---|---|
| **Rechner** | Raspberry Pi 4/5, NAS, Mini-PC oder ein alter Laptop – alles mit Linux |
| **Arbeitsspeicher** | 2 GB empfohlen. Mit 1 GB geht es, aber siehe [Wenig Arbeitsspeicher](#wenig-arbeitsspeicher) |
| **Speicherplatz** | rund 2 GB für die Images, plus Platz für eure Fotos |
| **Software** | Docker mit Compose-Plugin, `make`, `git` (oder nur das Archiv) |
| **Netzwerk** | Nur LAN. Es ist **nicht** dafür gebaut, aus dem Internet erreichbar zu sein |

Internet braucht das Dashboard nur für zwei Dinge: das Wetter und die Ortssuche.
Ohne Internet läuft alles andere weiter.

### Docker installieren

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

Danach **einmal ab- und wieder anmelden**, sonst greift die Gruppenzugehörigkeit
nicht. Prüfen mit:

```bash
docker ps && docker compose version
```

---

## Installieren

### Variante A – Archiv

```bash
tar -xzf familien-dashboard-pi-1.6.0.tar.gz
cd familien-dashboard-pi-1.6.0
```

### Variante B – Git

```bash
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
```

---

## Einrichten und starten

```bash
make setup
```

Das legt die Datenverzeichnisse an und erzeugt eine `.env` mit einem zufälligen
`JWT_SECRET`. **Diese Datei nie weitergeben und nie in ein Repository geben** –
sie ist der Schlüssel für alle Anmeldungen.

Danach anpassen, was für dich gilt:

```bash
nano .env
```

```env
# Wo lauscht das Dashboard?
#   0.0.0.0      = auf allen Netzwerkschnittstellen (bequem zum Ausprobieren)
#   192.168.1.20 = nur auf dieser Adresse (empfohlen im Dauerbetrieb)
BIND_ADDR=0.0.0.0
HTTP_PORT=8088
HTTPS_PORT=8443

# Startwert für den Wetterort – änderbar später in der App
WEATHER_LAT=48.2082
WEATHER_LON=16.3738
WEATHER_TZ=Europe/Vienna
TZ=Europe/Vienna
```

Vor dem ersten Start kurz prüfen lassen, ob das Gerät passt — das Skript ändert
nichts:

```bash
make preflight
```

Es meldet freie Ports, Konflikte mit vorhandenen Containern, freien
Arbeitsspeicher und Plattenplatz. Läuft auf dem Gerät schon etwas anderes, ist
das der Moment, es zu merken.

Dann starten:

```bash
make up
```

Der **erste Start baut die Images** und dauert je nach Gerät 5–20 Minuten. Danach
geht es in Sekunden.

Prüfen, ob alles läuft:

```bash
make verify
```

Erwartet: **55 Prüfungen bestanden.**

---

## Erste Schritte im Dashboard

Im Browser öffnen: `http://<IP-des-Servers>:8088`
(auf dem Gerät selbst: `http://localhost:8088`)

Es sind drei Benutzer angelegt – **Papa**, **Mama**, **Kind** – alle mit der PIN
`1234`.

**Das Erste, was du tust:**

1. Anmelden und unter **Einstellungen → PIN ändern** eine eigene PIN wählen.
   Für jede Person einzeln.
2. Unter **Einstellungen → Mein Profil** Name, Avatar und Farbe anpassen.
3. Unter **Verwaltung → Familienmitglieder** weitere Personen anlegen oder
   nicht gebrauchte löschen.

Solange jemand noch `1234` benutzt, weist das Dashboard oben darauf hin.

**Danach einrichten:**

- **Wetter** antippen → Ort suchen und auswählen
- **Verwaltung → Geräte** → die drei Beispiele stehen abgeschaltet da und
  zeigen auf `192.168.1.20`, eine Adresse aus der Vorlage. Eigene Adressen
  eintragen, mit *Verbindung testen* prüfen und dann auf **Aktiv** stellen.
  Achte darauf, dass *Prüf-Adresse* und *Oberfläche zum Antippen* denselben
  Rechner meinen — das Formular weist darauf hin, wenn nicht.
- **Musik** → siehe unten, dafür ist ein Eintrag in der `.env` nötig
- **Verwaltung → Familienmitglieder** → wer wenig Zeit hat, nimmt das Häkchen
  *nimmt an der Reihum-Verteilung teil* heraus und steht dann bei „reihum"
  nicht mehr im Plan
- **Aufgaben** → die Standardaufgaben ersetzen, Punkte ans Alter anpassen
- **Links** → was jeder oft braucht, anlegen und anpinnen
- **Ansicht anpassen** → Fenster in die Reihenfolge bringen, die zu euch passt

---

## Das Wandtablet im Flur

Ein Tablet, das fest an der Wand hängt, wird zum **Familiengerät**: dauerhaft
angemeldet, aber ohne persönliche Daten. Keine Rangliste, keine Einstellungen,
keine eigenen Links. Wer eine Aufgabe abhakt, tippt kurz auf sein Gesicht —
**ohne PIN**, und die Punkte landen trotzdem beim Richtigen.

Eingerichtet wird das **auf dem Tablet selbst**: dort als Administrator
anmelden, dann **Verwaltung → Wandgerät → Dieses Gerät als Wandgerät
einrichten**.

Beim Einrichten wirst du auf diesem Gerät **abgemeldet**. Das ist Absicht —
sonst liefe alles, was jemand im Flur abhakt, auf dein Konto. Die Einstellung
gilt nur für **dieses** Gerät; dein Handy bleibt unberührt.

Was danach anders ist:

- Nach fünf Minuten ohne Berührung wird aus dem Dashboard ein Bilderrahmen.
  Eine Berührung führt zurück in den Familien-Modus, nie in ein fremdes Konto.
- Musik hören und Dateien herunterladen gehen weiterhin — das sind
  Familiendinge, keine persönlichen.
- Wer das Tablet mit aufs Sofa nimmt, meldet sich oben rechts an und bekommt
  seine persönliche Ansicht. Nach dem Abmelden ist es wieder das Familiengerät.

Rückgängig geht es über **Verwaltung → Wandgerät → Wieder aufheben**.

---

## Wie es aussehen soll

Zwei Dinge lassen sich unabhängig voneinander einstellen, jede Person für
sich, unter **Einstellungen**:

- **Hell, dunkel oder wie das System** — die Farbstimmung.
- **Nachtlicht oder Glas** — die Oberfläche. *Nachtlicht* stellt die Fenster
  ohne Rahmen nebeneinander und trennt sie mit Haarlinien; *Glas* legt sie als
  milchige Scheiben übereinander. Beides funktioniert hell wie dunkel.

Unter **Ansicht anpassen** ordnet jeder die Fenster selbst an und blendet aus,
was er nicht braucht.

---

## Musik einrichten

Die Musik-Kachel spielt Dateien aus **einem** Ordner. Woher der kommt, ist dem
Dashboard gleich: ein Verzeichnis auf dem Server, eine eingebundene Festplatte
oder eine Freigabe vom NAS.

Das geht in zwei Schritten, weil zwei verschiedene Dinge dahinterstecken.

**Schritt 1 — welcher Ordner hereingereicht wird.** Das steht in der `.env`:

```bash
MUSIC_HOST_DIR=/media/festplatte/AUDIO
```

Danach `make up`. Der Ordner wird **schreibgeschützt** eingehängt — das
Dashboard soll abspielen, nicht löschen können.

Warum das nicht in der Weboberfläche geht: Das Backend läuft in einem
Container, und ein Container sieht nur, was in ihn eingehängt wurde. Was nicht
eingehängt ist, existiert für ihn nicht — daran ändert keine Einstellung in
einer Webseite etwas.

**Schritt 2 — welcher Teil davon gehört wird.** Das steht unter
**Verwaltung → Musik**. Dort lässt sich durch die Ordner blättern und einer
auswählen. Wer die ganze Platte einhängt, aber nur die Hörspiele im Dashboard
haben will, stellt das hier ein — ohne `.env`, ohne Neustart.

Beim Wechsel wird der alte Index verworfen und neu aufgebaut.

Was dann passiert:

- Im Hintergrund wird eingelesen. Der Start wartet nicht darauf. Bei
  zwanzigtausend Dateien auf einer USB-Platte dauert der erste Durchlauf
  einige Minuten; die Kachel zeigt, wie weit sie ist.
- Danach wird nur noch gelesen, was sich geändert hat. Alle sechs Stunden
  wird nachgesehen, und ein Administrator kann es über das Kreis-Symbol in der
  Kachel jederzeit anstossen.
- Erkannt werden `.mp3`, `.m4a`, `.m4b`, `.ogg`, `.opus`, `.flac`, `.wav`,
  `.aac` und `.wma`. Alles andere im Ordner wird übergangen.

**Der Benutzer, unter dem das Backend läuft, muss den Ordner lesen dürfen.**
Das ist die Kennung aus `PUID` in der `.env`. Nachsehen mit:

```bash
sudo -u "#$(grep ^PUID .env | cut -d= -f2)" ls /media/festplatte/AUDIO
```

Zwei Dinge, die keine Fehler sind:

- **Ton startet nie von allein.** Browser verbieten das. Nach einem Neustart
  des Wandtablets muss jemand einmal auf ▶ tippen.
- **Ist die Platte abgemeldet**, sagt die Kachel das und der Index bleibt
  stehen. Sobald sie wieder da ist, geht es weiter — ohne neu einzulesen.

Ganz abschalten lässt sich das Modul, indem `MUSIC_DIR` in der `.env` leer
bleibt. Dann wird auch nichts im Hintergrund eingelesen.

---

## Dateien zum Herunterladen

Die Kachel *Dateien* ist die Ablage für alles, was die ganze Familie braucht:
Bedienungsanleitungen, Elternbriefe, Formulare.

- Hochladen und löschen darf nur ein **Administrator**
- Herunterladen darf **jeder**, auch das Wandgerät im Flur
- Bis 100 MB je Datei, 300 MB je Vorgang
- Abgelegt unter `backend/data/files/` und **im Backup enthalten**

Wird der Platz auf dem Datenträger knapp, weist die Kachel darauf hin.

---

## Als App installieren

- **Android/Chrome:** Menü (⋮) → „App installieren"
- **iPhone/iPad:** Safari → Teilen → „Zum Home-Bildschirm"

> **Wichtig:** Der Offline-Modus und „App installieren" brauchen einen
> *secure context*. Über `http://` gibt es den nur auf `localhost`, **nicht**
> über eine LAN-Adresse. Also den HTTPS-Zugang auf Port **8443** benutzen.
>
> Das allein reicht aber nicht: Traefik bringt ein Platzhalter-Zertifikat mit,
> das nicht einmal eure Adresse enthält. Ein weggeklickter Zertifikatsfehler
> macht aus einer Seite **keine** vertrauenswürdige Herkunft — Chrome
> verweigert dann weiterhin Installation und Service Worker.
>
> Der Weg dahin steht unten unter *Ein eigenes Zertifikat*. Ohne das läuft
> das Dashboard ganz normal im Browser, nur eben ohne Installation und ohne
> Offline-Ansicht.

---

## Ein eigenes Zertifikat

Nötig, wenn ihr das Dashboard als **App installieren** oder den Offline-Modus
nutzen wollt. Zum reinen Aufrufen im Browser braucht es das nicht.

```bash
bash scripts/make-cert.sh 192.168.178.20
make up
```

Das Skript legt eine kleine **eigene Zertifizierungsstelle** an — eine Datei,
die sagt „diesen Zertifikaten glaube ich" — und stellt damit ein Zertifikat
für eure Adresse aus. Es sagt anschliessend selbst, wie die Stelle auf
Android, iPhone, Windows und Linux eingerichtet wird.

Das ist **einmal pro Gerät** zu tun. Danach:

- keine Zertifikatswarnung mehr
- „App installieren" erscheint
- Vollbild ohne Adressleiste
- Offline-Ansicht

Zwei Dinge dazu:

- **Auf dem iPhone sind es zwei Schritte.** Das Profil zu installieren reicht
  nicht; danach muss unter *Einstellungen → Allgemein → Info →
  Zertifikatsvertrauenseinstellungen* noch ein Schalter umgelegt werden. Ohne
  den zweiten Schritt bleibt die Warnung.
- **Der Schlüssel der Stelle bleibt im Haus.** `traefik/certs/familie-ca.key`
  ist von Git ausgenommen und gehört auf kein anderes Gerät. Wer ihn hat, kann
  Zertifikate ausstellen, denen eure Geräte glauben.

---

## Arbeitszeiten und Schulzeiten

Die Kachel *Arbeit & Schule* sagt, wer wann weg ist — und daraus, **ab wann
alle da sind**. Eingetragen wird unter **Arbeit & Schule** im Menü, auf zwei
Wegen:

**Fester Wochenplan** für alles, was jede Woche gleich ist. Ein Stundenplan
zum Beispiel: montags bis freitags 8:00 bis 13:00. Den fasst man ein- bis
zweimal im Jahr an.

**Nächste vier Wochen** für Zeiten, die jede Woche anders liegen. Vier Wochen
als Kalenderblatt, Uhrzeiten direkt in die Felder, ein Knopf übernimmt die
Woche darüber. Alles wird auf einmal gespeichert.

**Ein eingetragener Tag sticht den Wochenplan.** Ein Feiertag wird als *Frei*
eingetragen und hebt den Stundenplan für diesen einen Tag auf, ohne ihn zu
löschen. Ein leeres Feld heisst „nichts Besonderes" — dann gilt wieder der
Wochenplan.

Jeder pflegt seine eigenen Zeiten, ein Administrator die aller. Am Wandgerät
wird nur gelesen.

---

## Sicherung

Automatisch jede Nacht um 3 Uhr, sieben Tage Aufbewahrung, abgelegt unter
`backend/data/backup/`. Gesichert werden die Datenbank sowie die Ordner
`notes`, `ics`, `photos` und `files`.

**Musik ist bewusst nicht dabei.** Sie liegt ausserhalb des Datenordners, wird
nur schreibgeschützt eingehängt und kann Hunderte Gigabyte gross sein — die
gehört in deine übliche Datensicherung, nicht in die des Dashboards.

Manuell über **Verwaltung → Jetzt sichern** oder:

```bash
make backup      # Sicherung erstellen
make restore     # neueste Sicherung zurückspielen (fragt nach)
```

Eine Sicherung gelegentlich vom Gerät wegkopieren – SD-Karten halten nicht ewig:

```bash
rsync -avz benutzer@server:~/familien-dashboard/backend/data/backup/ ./sicherungen/
```

---

## Aktualisieren

```bash
git pull                 # oder: neues Archiv entpacken
make up
make verify
```

Deine Daten unter `backend/data/` bleiben unberührt; Schema-Änderungen laufen
beim Start automatisch durch. Läuft die installierte App gerade auf einem Gerät,
meldet sie sich von selbst mit „Neue Version verfügbar".

---

## Alle Befehle

```
make setup       Einrichten (einmalig)
make up          Starten
make down        Stoppen
make logs        Logs verfolgen
make verify      55 automatische Prüfungen
make check       Nur kompilieren und typprüfen, ohne zu starten

make dev         Entwicklungsmodus mit Hot-Reload → http://localhost:5173
make dev-down    Entwicklungsmodus stoppen

make backup      Sicherung erstellen
make restore     Sicherung zurückspielen

make clean       Container entfernen und aufräumen
```

Auf ein anderes Gerät ausrollen: siehe [DEPLOY.md](DEPLOY.md).

---

## Wenn es klemmt

| Problem | Lösung |
|---------|--------|
| `JWT_SECRET ist nicht gesetzt` | `make setup` ausführen |
| `port is already allocated` | `HTTP_PORT` in der `.env` auf einen freien Port ändern |
| Build bricht mit `killed` ab | Zu wenig Arbeitsspeicher, siehe unten |
| `permission denied` bei `backend/data` | `make init-data` |
| Seite nicht erreichbar | `docker compose ps` – laufen alle drei Container? Stimmt `BIND_ADDR`? |
| Wetter bleibt leer | Der Server braucht ausgehendes HTTPS zu `api.open-meteo.com` |
| Geräte alle rot | Adressen in **Verwaltung → Geräte** prüfen, *Verbindung testen* nutzen |
| Kein „App installieren" | Braucht HTTPS (Port 8443) oder `localhost` |
| PIN vergessen | Ein Administrator setzt sie unter **Verwaltung → Familienmitglieder** neu |
| Alle PINs vergessen | `make down`, `backend/data/db.sqlite*` löschen, `make up` – alles beginnt neu bei `1234` |

### Wenig Arbeitsspeicher

Der Frontend-Build braucht kurzzeitig rund 1 GB. Auf einem Gerät mit 1 GB RAM
vorher Auslagerungsspeicher einschalten:

```bash
sudo dphys-swapfile swapoff
sudo sed -i 's/^CONF_SWAPSIZE=.*/CONF_SWAPSIZE=1024/' /etc/dphys-swapfile
sudo dphys-swapfile setup
sudo dphys-swapfile swapon
```

Alternativ die Images auf einem stärkeren Rechner bauen und übertragen.

### Logs lesen

```bash
make logs                              # alles, fortlaufend
docker compose logs backend --tail=50  # nur das Backend
```

---

## Sicherheitshinweise

Das Dashboard ist für ein **vertrauenswürdiges Heimnetz** gebaut, nicht für das
offene Internet.

- **Keine Portweiterleitung einrichten.** Die Anmeldung ist eine vierstellige
  PIN – das reicht gegen versehentliche Zugriffe in der Familie, nicht gegen
  jemanden, der es ernsthaft versucht.
- Wer von unterwegs zugreifen will, nimmt ein VPN (WireGuard, Tailscale).
- Die `.env` bleibt geheim. Wer sie hat, kann Anmeldungen fälschen.
- Sicherungen enthalten alle Notizen, Fotos und Familiendateien im Klartext.
- Die Ablage *Dateien* nimmt jeden Dateityp an. Ausgeliefert wird sie
  ausschliesslich als Anhang und nie zur Anzeige im Browser — sonst könnte
  eine hochgeladene HTML-Datei unter der Adresse des Dashboards laufen.
  Hochladen darf trotzdem nur ein Administrator.

---

<div align="center">
<sub>

Familien Dashboard von Sebastian Blunk ·
[familienfabrik.at](https://familienfabrik.at) ·
[Spenden](https://paypal.me/bussdee)

</sub>
</div>
