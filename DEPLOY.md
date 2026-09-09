# Deploy auf einen anderen Rechner

Schritt-für-Schritt-Anleitung, um das Familien Dashboard von deinem
Arbeitsrechner auf das Zielgerät zu bringen — typischerweise einen Raspberry Pi,
ein NAS oder einen Mini-PC.

Für eine Installation **direkt auf** dem Zielgerät brauchst du diese Anleitung
nicht; dann reicht [INSTALL.md](INSTALL.md).

## Zielgerät festlegen

Alle `make`-Befehle hier sprechen `PI_HOST` an. Standard ist
`pi@raspberrypi.local`. Abweichend entweder pro Aufruf:

```bash
make deploy PI_HOST=pi@192.168.1.20
```

… oder dauerhaft am Anfang des `Makefile` eintragen. In den Beispielen unten
steht `$PI_HOST` stellvertretend für dein Zielgerät.

**Zeitbedarf:** rund 20 Minuten beim ersten Mal, danach etwa 3 Minuten pro
Aktualisierung.

---

## Was der Deploy anfasst — und was nicht

| | |
|---|---|
| **Belegt Ports** | 8088 (HTTP) und 8443 (HTTPS) |
| **Bleibt unberührt** | Alle anderen Dienste auf dem Zielgerät — Plex, Kavita, FileBrowser, SSH und so weiter |
| **Kontakt zu anderen Diensten** | ausschließlich lesende Health-Checks alle 30 Sekunden |
| **Wird übertragen** | Quellcode, Konfiguration, Skripte |
| **Wird NICHT übertragen** | `.env` und `backend/data/` — Datenbank, Notizen, Fotos und Backups auf dem Pi bleiben, wie sie sind |

Der Pi bekommt also seine **eigene Datenbank**. Die Aufgaben, Punkte, Fotos und
Einstellungen von diesem Testrechner wandern **nicht** automatisch mit (wie du
sie trotzdem mitnimmst, steht in Abschnitt 6).

---

## 1. Voraussetzungen prüfen

Auf **diesem** Rechner:

```bash
ssh $PI_HOST 'echo Verbindung ok && docker --version && docker compose version'
```

Erwartet: „Verbindung ok" plus zwei Versionsnummern.

**Wenn SSH nach einem Passwort fragt,** einmalig einen Schlüssel hinterlegen —
sonst fragt jeder Deploy dreimal nach:

```bash
ssh-keygen -t ed25519 -C "family-dashboard"    # nur falls noch kein Schlüssel da ist
ssh-copy-id $PI_HOST
```

**Wenn Docker fehlt,** auf dem Pi installieren:

```bash
ssh $PI_HOST
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
exit                                            # abmelden, damit die Gruppe greift
```

Danach neu anmelden und `docker ps` testen.

> **Hinweis zum Arbeitsspeicher:** Der Frontend-Build läuft auf dem Pi und
> braucht dabei kurzzeitig rund 1 GB. Auf einem Pi mit 1 GB RAM kann das
> scheitern — dann vorher Swap einschalten:
> `sudo dphys-swapfile swapoff && sudo sed -i 's/^CONF_SWAPSIZE=.*/CONF_SWAPSIZE=1024/' /etc/dphys-swapfile && sudo dphys-swapfile setup && sudo dphys-swapfile swapon`

---

## 1b. Vorabprüfung auf dem Zielgerät

Bevor irgendetwas gebaut wird: prüfen lassen, ob das Gerät passt. Das Skript
ändert nichts.

```bash
make pi-init                  # Code übertragen, ohne zu starten
ssh $PI_HOST 'cd ~/family-dashboard && bash scripts/preflight.sh'
```

Es meldet freie Ports, Namenskonflikte mit vorhandenen Containern, freien
Arbeitsspeicher inklusive Swap, Plattenplatz und ob das Wetter erreichbar ist.

> **Warum der Speicher zählt:** Der Frontend-Build braucht kurzzeitig rund 1 GB.
> Reicht der freie Speicher nicht, greift der OOM-Killer des Kernels — und der
> beendet nicht zwingend den Build, sondern den Prozess mit dem größten
> Speicherhunger. Auf einem Gerät, auf dem noch andere Dienste laufen, kann das
> einen davon treffen. Deshalb vorher prüfen.

---

## 2. Lokal freigeben

Der Deploy ist absichtlich gesperrt, bis die lokale Version läuft. Also zuerst:

```bash
make up
make verify
```

Erwartet: **40 Prüfungen bestanden.** Wenn hier etwas rot ist, wird nicht
deployt — der Fehler wäre auf dem Pi derselbe.

Zusätzlich einmal von Hand durchklicken: anmelden, eine Aufgabe abhaken, einen
Termin eintragen, ein Foto hochladen. Was hier nicht geht, geht dort auch nicht.

---

## 3. Einmalige Einrichtung auf dem Pi

Nur beim **allerersten** Mal nötig.

```bash
ssh $PI_HOST 'mkdir -p ~/family-dashboard'
```

> Der Zielpfad steht im `Makefile` als `PI_PATH` und ist `~/family-dashboard`.
> Auch den kannst du überschreiben: `make deploy PI_PATH=/opt/dashboard`.

Jetzt zurück auf **diesen** Rechner und den Code einmal hochkopieren:

```bash
make pi-init
```

`pi-init` überträgt ausschließlich den Code — es baut und startet nichts. Genau
das ist hier gewollt, denn die `.env` fehlt auf dem Pi ja noch.

Wieder auf dem Pi:

```bash
cd ~/family-dashboard
make setup
```

`make setup` legt die `.env` an und erzeugt darin ein zufälliges `JWT_SECRET`.
Anschließend die Netzwerk-Bindung eintragen:

```bash
nano .env
```

Diese Zeile ändern:

```env
BIND_ADDR=<ZIEL-IP>
```

Damit lauscht das Dashboard **nur** auf der LAN-Adresse des Pi statt auf allen
Netzwerkschnittstellen. Mit `Strg+O`, `Enter`, `Strg+X` speichern und schließen.

Optional im selben Schritt anpassen, falls gewünscht:

```env
WEATHER_TZ=Europe/Berlin
TZ=Europe/Berlin
```

> Das `JWT_SECRET` auf dem Pi ist bewusst ein **anderes** als hier. Es wird nie
> übertragen. Ändert man es später, müssen sich alle neu anmelden — sonst nichts.

---

## 4. Deploy auslösen

Zurück auf **diesem** Rechner:

```bash
make deploy
```

Es kommen zwei Rückfragen. Beide brauchen exakt `ja`, alles andere bricht ab:

```
❓ BESTÄTIGUNG 1/2: Lokale Version getestet und freigegeben? [ja/NEIN]
❓ BESTÄTIGUNG 2/2: Deploy auf den Pi JETZT durchführen? [ja/NEIN]
```

Danach passiert Folgendes:

1. **rsync** überträgt den Code (ohne `.env`, ohne `backend/data`)
2. Auf dem Pi werden die Datenverzeichnisse angelegt, falls sie fehlen
3. `docker compose up -d --build` baut die Images und startet den Stack

Der **erste** Build dauert auf einem Pi 4 rund 10–15 Minuten (Go-Backend und
Frontend-Bundle). Spätere Deploys sind deutlich schneller, weil Docker die
Zwischenschichten wiederverwendet.

Am Ende erscheint:

```
✅ Deploy fertig: http://<ZIEL-IP>:8088
```

---

## 5. Nach dem Deploy prüfen

```bash
make pi-status          # laufen alle drei Container und sind sie "healthy"?
make pi-verify          # Smoke-Test gegen den Pi
```

Erwartet: dreimal `healthy` und wieder **40 Prüfungen bestanden**.

Dann im Browser `http://<ZIEL-IP>:8088` öffnen.

### Die ersten drei Handgriffe

1. **Als Papa anmelden** (PIN `1234`)
2. **Einstellungen → PIN ändern** — und dasselbe für Mama und Kind
3. **Wetter** antippen und den Ort einstellen

Solange jemand noch die Standard-PIN hat, zeigt das Dashboard oben einen
gelben Hinweis. Der verschwindet von selbst, sobald alle gewechselt haben.

### Danach einrichten

- **Verwaltung → Geräte**: Adressen an den Pi anpassen und mit *Verbindung
  testen* prüfen
- **Aufgaben**: die Standard-Aufgaben durch eure echten ersetzen, Punkte ans
  Alter der Kinder anpassen
- **Links**: was jeder oft braucht, anlegen und anpinnen
- **Fotos**: über den Foto-Rahmen hochladen
- **Kalender**: Termine eintragen oder eine `.ics` nach
  `~/family-dashboard/backend/data/ics/` kopieren

---

## 6. Daten von diesem Rechner mitnehmen (optional)

Nur nötig, wenn die hier angelegten Aufgaben, Notizen und Fotos auf den Pi
sollen. **Beide Stacks müssen dafür gestoppt sein**, sonst kopiert man einen
halb geschriebenen Datenbankstand.

```bash
# Hier: Stack stoppen und ein sauberes Backup ziehen
make down
make backup

# Auf dem Pi: Stack stoppen
ssh $PI_HOST 'cd ~/family-dashboard && docker compose down'

# Daten übertragen
rsync -avz backend/data/ $PI_HOST:~/family-dashboard/backend/data/

# Beide wieder starten
ssh $PI_HOST 'cd ~/family-dashboard && docker compose up -d'
make up
```

> Die Datei `db.sqlite-wal` gehört zwingend mit übertragen — sie enthält die
> zuletzt geschriebenen Änderungen. `rsync` auf das ganze Verzeichnis nimmt sie
> automatisch mit.

Danach gelten auf dem Pi die **PINs von diesem Rechner**, nicht mehr `1234`.

---

## 7. Aktualisieren

Für jede spätere Änderung reicht:

```bash
make up && make verify     # lokal testen
make deploy                # zwei Mal "ja"
```

Läuft auf einem Gerät gerade die installierte App, meldet sie sich nach dem
Deploy von selbst mit **„Neue Version verfügbar"** und lädt auf Knopfdruck neu.

---

## 8. Wenn etwas schiefgeht

### Zurück auf die vorherige Version

Der Deploy überschreibt nur den Code, nie die Daten. Ein Rückschritt ist daher
harmlos: die vorherige Fassung wieder ausspielen und erneut deployen.

### Logs ansehen

```bash
make pi-logs                                        # alle Container, fortlaufend
ssh $PI_HOST 'cd ~/family-dashboard && docker compose logs backend --tail=100'
```

### Häufige Fehler

| Meldung | Ursache und Lösung |
|---------|--------------------|
| `.env fehlt auf dem Pi` | Abschnitt 3 nachholen: auf dem Pi `make setup` |
| `JWT_SECRET ist nicht gesetzt` | In der `.env` auf dem Pi steht noch der Platzhalter |
| `port is already allocated` | 8088 oder 8443 ist belegt. In der `.env` auf dem Pi `HTTP_PORT` ändern |
| Build bricht mit `killed` ab | Zu wenig Arbeitsspeicher — Swap einschalten (Abschnitt 1) |
| `permission denied` auf `backend/data` | Auf dem Pi: `cd ~/family-dashboard && make init-data` |
| Dashboard nicht erreichbar | `make pi-status` prüfen; steht `BIND_ADDR` richtig in der `.env`? |
| Wetter bleibt leer | Der Pi braucht ausgehendes HTTPS zu `api.open-meteo.com` |
| Geräte alle rot | Adressen in **Verwaltung → Geräte** prüfen, *Verbindung testen* benutzen |

### Notbremse

```bash
ssh $PI_HOST 'cd ~/family-dashboard && docker compose down'
```

Stoppt nur das Dashboard. Plex, Kavita und FileBrowser laufen davon unberührt
weiter — sie teilen sich weder Container noch Netzwerk mit dem Dashboard.

---

## 9. Betrieb

**Backups** laufen automatisch jede Nacht um 3 Uhr, sieben Tage Aufbewahrung,
abgelegt unter `~/family-dashboard/backend/data/backup/`. Manuell geht es über
**Verwaltung → Jetzt sichern**.

Ein Backup gelegentlich vom Pi wegkopieren — eine SD-Karte hält nicht ewig:

```bash
rsync -avz $PI_HOST:~/family-dashboard/backend/data/backup/ ./pi-backups/
```

**Neustart nach Stromausfall** passiert von selbst: alle Container laufen mit
`restart: unless-stopped`.

**HTTPS und PWA:** Offline-Betrieb und „App installieren" brauchen einen
sicheren Kontext. Über `http://<ZIEL-IP>:8088` läuft die App normal, aber
ohne Offline-Cache und ohne Installations-Vorschlag. Wer das möchte, nutzt
`https://<ZIEL-IP>:8443` und bestätigt die Zertifikatswarnung einmalig pro
Gerät (selbstsigniert, das ist im eigenen LAN in Ordnung).

---

<div align="center">
<sub>

Familien Dashboard von Sebastian Blunk ·
[familienfabrik.at](https://familienfabrik.at) ·
[Spenden](https://paypal.me/bussdee)

</sub>
</div>
