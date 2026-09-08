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
tar -xzf familien-dashboard-pi-1.0.1.tar.gz
cd familien-dashboard-pi-1.0.1
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

Erwartet: **35 Prüfungen bestanden.**

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
- **Verwaltung → Geräte** → die Beispiele durch eure eigenen ersetzen und mit
  *Verbindung testen* prüfen
- **Aufgaben** → die Standardaufgaben ersetzen, Punkte ans Alter anpassen
- **Links** → was jeder oft braucht, anlegen und anpinnen
- **Ansicht anpassen** → Fenster in die Reihenfolge bringen, die zu euch passt

---

## Als App installieren

- **Android/Chrome:** Menü (⋮) → „App installieren"
- **iPhone/iPad:** Safari → Teilen → „Zum Home-Bildschirm"

> **Wichtig:** Der Offline-Modus und der Installations-Vorschlag brauchen einen
> *secure context*. Über `http://` funktioniert das nur auf `localhost`, **nicht**
> über eine LAN-IP. Für beides den HTTPS-Zugang auf Port **8443** benutzen und
> die Zertifikatswarnung einmalig pro Gerät bestätigen (selbstsigniert – im
> eigenen Heimnetz in Ordnung).
>
> Ohne HTTPS läuft die App ganz normal, nur ohne Offline-Cache.

---

## Sicherung

Automatisch jede Nacht um 3 Uhr, sieben Tage Aufbewahrung, abgelegt unter
`backend/data/backup/`. Manuell über **Verwaltung → Jetzt sichern** oder:

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
make verify      35 automatische Prüfungen
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
- Sicherungen enthalten alle Notizen und Fotos im Klartext.

---

<div align="center">
<sub>

Familien Dashboard von Sebastian Blunk ·
[familienfabrik.at](https://familienfabrik.at) ·
[Spenden](https://paypal.me/bussdee)

</sub>
</div>
