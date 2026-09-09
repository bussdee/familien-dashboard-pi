<div align="center">

# 🏠 Familien Dashboard

**Ein gemeinsamer Bildschirm für den Familienalltag — auf einem Gerät bei dir zu Hause.**

Wer bringt heute den Müll raus? Was fehlt beim Einkaufen? Wann ist der Elternabend?

[Was ist das?](#was-ist-das) · [Bilder](#so-sieht-es-aus) · [Installation](#installation) · [Erste Schritte](#erste-schritte-nach-der-installation) · [Ehrlich gesagt](#ehrlich-gesagt-die-grenzen)

[![CI](https://github.com/bussdee/familien-dashboard-pi/actions/workflows/ci.yml/badge.svg)](https://github.com/bussdee/familien-dashboard-pi/actions/workflows/ci.yml) ![Lizenz](https://img.shields.io/badge/Lizenz-MIT-blue) ![Nur LAN](https://img.shields.io/badge/Nur%20f%C3%BCrs-Heimnetz-orange) ![Ohne Cloud](https://img.shields.io/badge/Cloud-nein%20danke-green)

<br>

<img src="docs/screenshots/Familien_Modus.png" alt="Das Dashboard im Familien-Modus: Aufgaben, Einkaufsliste, Kalender, geteilte Links, Notizen, Foto-Rahmen und Geräte-Status">

<sub>Das Wandtablet im Familien-Modus — begrüßt niemanden persönlich, zeigt keine Rangliste.</sub>

</div>

---

## Was ist das?

Ein Tablet an der Küchenwand. Ein Lesezeichen auf jedem Handy in der Familie.
Darauf steht, was heute ansteht — und wer dran ist.

Das Familien Dashboard ist **eine Entlastung für Eltern**, keine weitere App,
die verwaltet werden will. Es nimmt die Fragen ab, die sonst zehnmal am Tag
gestellt werden, und macht aus den ewigen Aufgaben ein kleines Spiel, bei dem
Kinder mitmachen wollen.

Es läuft auf einem Gerät bei dir zu Hause: ein Raspberry Pi, ein alter Laptop,
ein NAS, dein normaler PC. **Kein Google-Konto. Kein Abo. Keine Daten, die das
Haus verlassen.**

### 🔒 Das Wichtigste zuerst: Es ist nur fürs Heimnetz

Dieses Projekt ist bewusst **einfach** gehalten, nicht **sicher** im Sinne einer
Anwendung, die im Internet steht:

- Die Anmeldung ist eine **vierstellige PIN**. Sie verhindert, dass das Kind aus
  Versehen als Papa Punkte bucht. Sie hält niemanden auf, der es ernsthaft
  versucht.
- Es gibt **keine Verschlüsselung der Inhalte**, keine Zwei-Faktor-Anmeldung,
  keine Zugriffsprotokolle.
- **Stelle es nicht ins Internet.** Keine Portweiterleitung im Router. Wer von
  unterwegs zugreifen will, nimmt ein VPN (WireGuard oder Tailscale).

Das ist eine bewusste Entscheidung: Ein Familien-Dashboard soll man mit einem
Tippen bedienen, nicht mit einem Passwortmanager. Diese Einfachheit gilt nur,
solange es im eigenen, vertrauenswürdigen Netz bleibt.

---

## Was es kann

| | |
|---|---|
| ⭐ **Aufgaben** | Müll, Geschirrspüler, Katzenklo. Jede Aufgabe hat ein Intervall und einen Punktwert. Zuständig ist wahlweise *reihum*, eine feste Person, *alle* oder *wer mag*. Einmal erledigt ist erledigt — ein zweites Abhaken am selben Tag gibt es nicht. |
| 🏆 **Punkte & Level** | Wer abhakt, bekommt Punkte. Level von *Neuling* bis *Legende*, Serien, elf Abzeichen, Siegertreppchen. Verklickt? Ein Elternteil nimmt es zurück — die Aufgabe wird dabei wieder fällig. |
| 🛒 **Einkaufsliste** | Gemeinsam und **in Echtzeit**: Was im Laden abgehakt wird, verschwindet sofort auf allen Geräten. Punkte gibt es auch dafür. |
| 📅 **Kalender** | Termine direkt eintragen, einmalig oder wiederkehrend. Oder eine `.ics` aus Apple, Google oder Outlook ablegen. |
| 📝 **Notizen** | Markdown, zweiseitig mit echten Dateien synchronisiert. Bleiben lesbar, auch ohne dieses Programm. |
| 🔗 **Links** | Lesezeichen mit Kategorien, privat oder mit der Familie geteilt. Angepinnte erscheinen auf der Startseite. |
| 🌧️ **Wetter mit Regenzeiten** | Nicht „98 % Regen", sondern **„Regen ab 18:30"**. Dazu der Temperaturverlauf der nächsten Stunden als Kurve. Wo Open-Meteo Viertelstundenwerte liefert (Mitteleuropa, DWD-Modell), wird es entsprechend genau. |
| 🖼️ **Foto-Rahmen & Diashow** | Bilder hochladen, wechseln automatisch. Ein Knopf schaltet auf **Vollbild**: nur Foto, Uhrzeit, Wetter und was heute noch ansteht. |
| 🏠 **Familien-Modus fürs Wandtablet** | Ein Tablet im Flur wird zum Familiengerät: dauerhaft angemeldet, aber ohne persönliche Daten. Wer abhakt, tippt kurz auf sein Gesicht — **keine PIN**, und die Punkte landen beim Richtigen. Wer es mit aufs Sofa nimmt, meldet sich an und ist danach wieder im Familien-Modus. |
| 📱 **Geräte-Status** | Läuft Plex, Kavita, der NAS? Kachel antippen öffnet die Oberfläche. |
| 🎛️ **Pro Person** | Jeder ordnet sich die Fenster selbst an und blendet aus, was er nicht braucht. Zwei Oberflächen zur Wahl — *Nachtlicht* (offen, mit feinen Linien) und *Glas* (Fenster als milchige Scheiben) — jeweils hell oder dunkel. |
| 📲 **Wie eine App** | Zum Startbildschirm hinzufügen: eigenes Symbol, keine Browserleiste, Offline-Ansicht. |

---

## So sieht es aus

### Der Familien-Modus

Das Wandtablet gehört niemandem — es gehört allen. Hakt jemand eine Aufgabe
ab, fragt es kurz nach. **Ein Tipp aufs eigene Gesicht, keine PIN.** Die Punkte
landen bei dem, der die Arbeit gemacht hat, und nicht bei dem, der sich zuletzt
angemeldet hatte.

![Der Dialog „Wer war das?" mit den drei Familienmitgliedern zur Auswahl](docs/screenshots/WerWarDas.png)

### Der Rest

<table>
<tr>
<td width="50%">

**Anmeldung**

Antippen, PIN, fertig. Keine E-Mail, kein Konto.

<img src="docs/screenshots/LogIn.png" alt="Anmeldebildschirm mit den drei Familienmitgliedern">

</td>
<td width="50%">

**Angemeldet: die persönliche Ansicht**

Mit Namen, Punktestand und Level. Am Wandgerät fehlt genau das.

<img src="docs/screenshots/Familien_Dashboard.png" alt="Die Übersicht als angemeldete Person, mit Begrüßung und Punkteband">

</td>
</tr>
<tr>
<td width="50%">

**Rangliste**

Siegertreppchen, Level, Abzeichen und wer zuletzt was erledigt hat.

<img src="docs/screenshots/Rangliste.png" alt="Rangliste mit Siegertreppchen, Punkten, Abzeichen und Verlauf">

</td>
<td width="50%">

**Einstellungen**

Name, Avatar, Farbe und PIN ändert jeder selbst. Dazu hell oder dunkel und
die Wahl zwischen den Oberflächen *Nachtlicht* und *Glas*.

<img src="docs/screenshots/Einstellungen.png" alt="Einstellungen mit Profil, Avatarauswahl, Darstellung, Oberfläche und PIN-Änderung">

</td>
</tr>
</table>

**Auf einem schmaleren Bildschirm** rücken die Fenster in drei Spalten
zusammen, auf dem Handy untereinander.

![Die Übersicht in drei Spalten](docs/screenshots/Dashboard_Papa.png)

---

## Installation

Du brauchst **kein Vorwissen**. Wenn du ein Terminal öffnen und drei Zeilen
eintippen kannst, reicht das. Wähle unten dein Gerät.

### Was in jedem Fall gebraucht wird

- Ein Gerät, das durchlaufen kann (oder wenigstens dann läuft, wenn ihr es nutzt)
- **2 GB Arbeitsspeicher** empfohlen — mit 1 GB geht es auch, siehe [Wenig Speicher](#wenig-arbeitsspeicher)
- Rund 2 GB Platz auf der Festplatte
- **Docker** — die Installation dafür steht bei jeder Variante dabei

---

<details open>
<summary><h3>🥧 Raspberry Pi (empfohlen)</h3></summary>

Ein Pi 4 oder 5 mit Raspberry Pi OS (64-Bit). Der Pi 3 geht auch, dauert aber.

**1. Docker installieren**

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

Danach **einmal ab- und wieder anmelden** (`exit`, dann neu verbinden), sonst
darf dein Benutzer Docker noch nicht bedienen.

**2. Dashboard holen und starten**

```bash
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup
make up
```

**3. Öffnen**

Die Adresse des Pi herausfinden:

```bash
hostname -I
```

Dann im Browser: `http://<diese-Adresse>:8088`

> Der erste Start baut die Anwendung und dauert **10–20 Minuten**. Danach
> startet sie in Sekunden. Hab Geduld, das ist einmalig.

</details>

<details>
<summary><h3>🐧 Linux-PC oder Server</h3></summary>

Ubuntu, Debian, Fedora, Mint — alles recht.

**1. Docker installieren**

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

Ab- und wieder anmelden, dann prüfen:

```bash
docker ps
```

**2. Dashboard holen und starten**

```bash
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup
make up
```

**3. Öffnen**

Auf dem Rechner selbst: `http://localhost:8088`
Von anderen Geräten: `http://<IP-des-Rechners>:8088` (`hostname -I` zeigt sie an)

</details>

<details>
<summary><h3>🪟 Windows-PC</h3></summary>

Funktioniert über Docker Desktop. Windows 10 oder 11.

**1. Docker Desktop installieren**

[Docker Desktop herunterladen](https://www.docker.com/products/docker-desktop/)
und installieren. Bei der Frage nach dem Backend **WSL 2** wählen (Standard).
Danach den Rechner neu starten und Docker Desktop einmal öffnen — es muss
laufen, bevor es weitergeht.

**2. Ubuntu-Terminal öffnen**

PowerShell als Administrator starten und einmalig:

```powershell
wsl --install
```

Neu starten, Benutzernamen und Passwort für Ubuntu vergeben. Ab jetzt findest du
**„Ubuntu"** im Startmenü — dort geht es weiter.

**3. Dashboard holen und starten**

Im Ubuntu-Fenster:

```bash
sudo apt update && sudo apt install -y git make
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup
make up
```

**4. Öffnen**

Auf dem PC selbst: `http://localhost:8088`

Von Handy oder Tablet aus brauchst du die IP-Adresse des PCs. In der
Eingabeaufforderung (nicht in Ubuntu):

```powershell
ipconfig
```

Die Zeile „IPv4-Adresse" bei deinem WLAN- oder LAN-Adapter — dann
`http://<diese-Adresse>:8088`.

> **Hinweis:** Ein Windows-PC wird meist nicht durchlaufen. Das Dashboard ist
> dann nur erreichbar, wenn der Rechner an ist. Für den Dauerbetrieb ist ein
> Raspberry Pi die bessere Wahl.

</details>

<details>
<summary><h3>🍎 Mac</h3></summary>

**1. Docker Desktop installieren**

[Docker Desktop für Mac](https://www.docker.com/products/docker-desktop/) — beim
Apple-Chip (M1 bis M4) die **Apple-Silicon**-Fassung wählen. Öffnen und laufen
lassen.

**2. Dashboard holen und starten**

Terminal öffnen (⌘ + Leertaste, „Terminal"):

```bash
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup
make up
```

**3. Öffnen**

`http://localhost:8088` — oder von anderen Geräten aus mit der IP des Macs
(Systemeinstellungen → Netzwerk).

</details>

<details>
<summary><h3>📦 NAS (Synology, QNAP, Unraid)</h3></summary>

Sofern dein NAS Docker **und** SSH-Zugang bietet.

**Synology:** Im Paket-Zentrum „Container Manager" (früher „Docker")
installieren. Unter Systemsteuerung → Terminal & SNMP den SSH-Dienst
aktivieren.

Dann per SSH verbinden und wie unter Linux vorgehen:

```bash
git clone https://github.com/bussdee/familien-dashboard-pi.git
cd familien-dashboard-pi
make setup
make up
```

> Falls `make` fehlt, geht es auch direkt:
> ```bash
> bash scripts/init-data.sh
> cp .env.example .env
> # In der .env ein eigenes JWT_SECRET eintragen:
> #   openssl rand -base64 48
> docker compose up -d --build
> ```
>
> Achte darauf, dass die Ports **8088** und **8443** auf dem NAS frei sind.
> Sonst in der `.env` andere eintragen.

</details>

<details>
<summary><h3>📥 Ohne Git (ZIP-Datei)</h3></summary>

Wenn du kein Git benutzen willst:

1. Oben rechts auf **Code → Download ZIP** klicken (oder ein Release-Archiv laden)
2. Entpacken
3. Terminal im entpackten Ordner öffnen und:

```bash
make setup
make up
```

</details>

---

## Erste Schritte nach der Installation

**Prüfen, ob alles läuft:**

```bash
make verify
```

Erwartet: **40 Prüfungen bestanden.** Wenn hier etwas rot ist, hilft
[INSTALL.md](INSTALL.md) weiter.

**Im Browser öffnen** und anmelden. Es sind drei Benutzer angelegt — **Papa**,
**Mama**, **Kind** — alle mit der PIN `1234`.

Dann in dieser Reihenfolge:

1. **PIN ändern.** Einstellungen → PIN ändern, für jede Person einzeln.
   Solange jemand noch `1234` hat, weist das Dashboard oben darauf hin.
2. **Profil anpassen.** Einstellungen → Mein Profil: Name, Avatar, Farbe.
3. **Personen ergänzen.** Verwaltung → Familienmitglieder: weitere anlegen,
   ungenutzte löschen.
4. **Wetterort setzen.** Auf das Wetter tippen, Ort suchen, auswählen.
5. **Aufgaben anpassen.** Die Beispiele ersetzen. Punkte ans Alter anpassen —
   was ein Sechsjähriger schafft, ist anders bewertet als das Bad putzen.
6. **Geräte eintragen** (optional). Verwaltung → Geräte, mit *Verbindung testen*.

### Ein Tablet an die Wand hängen

Wenn ein Tablet fest im Flur oder in der Küche hängt, gehört es niemandem —
es gehört allen. Genau dafür gibt es den **Familien-Modus**:

Auf diesem Tablet einmal als Administrator anmelden, dann **Verwaltung →
Wandgerät → Dieses Gerät als Wandgerät einrichten**. Das Tablet meldet dich
dabei ab und ist ab sofort das Familiengerät — mehr ist nicht zu tun.

Ab jetzt ist das Tablet dauerhaft bereit und zeigt Aufgaben, Einkaufsliste,
Termine, Wetter und die geteilten Links — aber nichts Persönliches: keine
Rangliste, keine Einstellungen, keine privaten Lesezeichen. Hakt jemand eine
Aufgabe ab, fragt es kurz **„Wer war das?"** und zeigt die Gesichter. Ein
Tipp genügt, die Punkte landen beim Richtigen.

Nach fünf Minuten ohne Berührung wird daraus von selbst ein Bilderrahmen.
Und wer das Tablet mit aufs Sofa nimmt, meldet sich oben rechts an — nach dem
Abmelden ist es wieder das Familiengerät.

Der Schalter gilt nur für **dieses eine Gerät**. Handys bleiben davon
unberührt.

**Aufs Handy holen:** Im Browser „Zum Home-Bildschirm hinzufügen" (iPhone:
Safari → Teilen) beziehungsweise „App installieren" (Android: Chrome-Menü).

---

## Ehrlich gesagt: die Grenzen

Damit du nicht enttäuscht wirst:

- **Nur auf Deutsch.** Eine Übersetzungsebene gibt es noch nicht.
- **Keine ernsthafte Zugangssicherung.** Vierstellige PIN, sonst nichts. Siehe
  ganz oben.
- **Am Wandgerät gibt es gar keine PIN.** Wer das Tablet im Flur in der Hand
  hält, kann Aufgaben für jedes Familienmitglied abhaken und damit Punkte
  buchen. Das ist Absicht: Ein Kind soll im Vorbeigehen abhaken können, ohne
  sich anzumelden. Alles, was wehtut — Punkte korrigieren, Benutzer verwalten,
  PIN ändern — bleibt hinter der Anmeldung. Wer das nicht will, richtet den
  Familien-Modus einfach nicht ein.
- **Offline-Modus und App-Installation brauchen HTTPS.** Über `http://` und eine
  LAN-Adresse verweigern Browser den Service Worker. Ein HTTPS-Zugang liegt auf
  Port **8443** bereit, mit selbstsigniertem Zertifikat — die Warnung muss man
  einmal pro Gerät bestätigen. Ohne HTTPS läuft alles ganz normal, nur ohne
  Offline-Ansicht.
- **Wetter braucht Internet.** Ohne Verbindung zeigt es den letzten Stand und
  sagt dazu, dass er alt ist. Alles andere läuft weiter.
- **Der erste Start dauert 10–20 Minuten.** Einmalig.
- **Fenster sortiert man mit Pfeiltasten**, nicht per Ziehen.
- **HEIC-Fotos vom iPhone** werden beim direkten Upload nicht unterstützt (beim
  Teilen wandelt iOS meist automatisch in JPG um).

---

## Alle Befehle

```
make setup       Einrichten (einmalig)
make preflight   Prüfen, ob das Gerät passt (ändert nichts)
make up          Starten                 make down     Stoppen
make logs        Logs ansehen            make verify   40 Prüfungen
make backup      Sichern                 make restore  Zurückspielen
make dev         Entwicklungsmodus mit Hot-Reload
make clean       Aufräumen
```

Auf ein anderes Gerät ausrollen: [DEPLOY.md](DEPLOY.md).

### Wenn es klemmt

| Problem | Lösung |
|---------|--------|
| `JWT_SECRET ist nicht gesetzt` | `make setup` ausführen |
| `port is already allocated` | `HTTP_PORT` in der `.env` auf einen freien Port ändern |
| `unable to open database file` | `PUID`/`PGID` in der `.env` stimmen nicht mit dem Besitzer von `backend/data` überein. Eigene Kennung mit `id -u` und `id -g` ermitteln und eintragen. |
| Build bricht mit `killed` ab | Zu wenig Arbeitsspeicher, siehe unten |
| Seite nicht erreichbar | `docker compose ps` — laufen alle drei Container? |
| PIN vergessen | Ein Administrator setzt sie in der Verwaltung neu |
| Alle PINs vergessen | `make down`, `backend/data/db.sqlite*` löschen, `make up` — Notizen und Fotos bleiben erhalten |

#### Wenig Arbeitsspeicher

Auf einem Gerät mit 1 GB vorher Auslagerungsspeicher einschalten:

```bash
sudo dphys-swapfile swapoff
sudo sed -i 's/^CONF_SWAPSIZE=.*/CONF_SWAPSIZE=1024/' /etc/dphys-swapfile
sudo dphys-swapfile setup && sudo dphys-swapfile swapon
```

Mehr Hilfe in [INSTALL.md](INSTALL.md).

---

## Sicherung

Läuft automatisch jede Nacht um 3 Uhr, sieben Tage Aufbewahrung, unter
`backend/data/backup/`. Manuell über **Verwaltung → Jetzt sichern** oder
`make backup`.

Kopiere gelegentlich eine Sicherung vom Gerät weg — SD-Karten halten nicht ewig.

---

## Technisches

Go-Backend mit SQLite (rein in Go, ohne CGO — dadurch baut ARM64 ohne
Cross-Toolchain), SvelteKit-Frontend als Single-Page-App, Traefik als Reverse
Proxy. **Drei Container, rund 150 MB Arbeitsspeicher im Betrieb.**

Was trotz der Einfachheit sauber gemacht ist: PIN-Anmeldung mit **argon2id**,
Sperre nach fünf Fehlversuchen, JWT im HttpOnly-Cookie, WebSocket nur vom
eigenen Origin, Container als non-root mit read-only Dateisystem, Traefik ohne
Docker-Socket, Upload-Prüfung nach Dateiinhalt statt nach Endung, Notizen durch
DOMPurify. Das Backend startet nicht ohne gesetztes `JWT_SECRET`.

```
familien-dashboard/
├── backend/internal/     auth · calendar · chores · points · shopping
│                         notes · links · devices · photos · weather
│                         backup · config · store
├── frontend/src/         SvelteKit (Routen, Widgets, Stores)
├── traefik/              Reverse Proxy
├── scripts/              Einrichtung, Sicherung, Tests, Release
└── backend/data/         ← eure Daten, nie im Repository
```

Vor einem Pull Request bitte:

```bash
make check              # kompilieren + typprüfen
make up && make verify
```

Das Ganze ausführlicher — Umgebung, Stil, was nie ins Repository gehört —
steht in [CONTRIBUTING.md](CONTRIBUTING.md). Wer eine Sicherheitslücke
gefunden hat, findet den Weg dorthin in [SECURITY.md](SECURITY.md).

---

## Lizenz

[MIT](LICENSE) — nutzen, ändern, weitergeben, auch gewerblich. Ohne Gewähr.

---

<div align="center">

Gebaut von **Sebastian Blunk** für die eigene Familie — und für alle, die es
gebrauchen können.

[familienfabrik.at](https://familienfabrik.at) · [☕ Spenden](https://paypal.me/bussdee)

<sub>Wenn dir das Projekt Zeit spart, freue ich mich über einen Kaffee.</sub>

</div>
