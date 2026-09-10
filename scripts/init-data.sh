#!/usr/bin/env bash
# Legt die Datenverzeichnisse an und füllt sie beim ersten Mal mit Beispielen.
# Idempotent: vorhandene Dateien werden nie überschrieben.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA="$ROOT/backend/data"

mkdir -p "$DATA"/{notes,ics,photos,files,music,backup}
# Muss existieren, bevor Docker ihn beim Einhängen selbst anlegt — sonst
# gehört er root und das Zertifikatsskript kann nichts hineinschreiben.
mkdir -p "$ROOT/traefik/certs"

# Der Backend-Container läuft als UID 1000. Statt 777 wird nur so weit
# geöffnet, wie es dafür nötig ist.
chmod 755 "$DATA" "$DATA"/{notes,ics,photos,files,music,backup} 2>/dev/null || true

created=0

# Bewusst KEIN Beispiel-Kalender: Platzhalter-Termine, die niemand löschen
# kann, nerven mehr als sie helfen. Termine legt man im Dashboard an, oder man
# kopiert eine echte .ics-Datei nach backend/data/ics/.

if [ ! -f "$DATA/notes/willkommen.md" ]; then
  cat > "$DATA/notes/willkommen.md" <<'NOTE'
---
title: Willkommen! 👋
tags: [info, start]
pinned: true
---

# Willkommen im Familien Dashboard

Alles läuft lokal auf unserem eigenen Server – keine Cloud, keine Konten.

## Die Widgets
- **Wetter** – aktuelle Lage und Vorhersage
- **Kalender** – Termine direkt eintragen, oder eine `.ics`-Datei ablegen
- **Einkaufen** – gemeinsame Liste, synchronisiert sich live auf allen Geräten
- **Notizen** – Markdown, liegt als Datei in `backend/data/notes/`
- **Aufgaben** – wer macht was, mit Punkten, Leveln und Rangliste
- **Geräte** – ist Plex/Kavita/FileBrowser erreichbar?
- **Countdowns** – Geburtstage und Ferien aus dem Kalender
- **Fotos** – Bilder aus `backend/data/photos/`
- **Dateien** – Anleitungen und Formulare; Admin lädt hoch, alle laden herunter
- **Musik** – MP3s und Hörspiele aus einem eingehängten Ordner (`MUSIC_HOST_DIR`)
- **Arbeit & Schule** – wer wann weg ist, und ab wann alle da sind

## Wichtig zum Start
Alle starten mit der PIN **1234**. Bitte gleich unter
**Einstellungen → PIN ändern** eine eigene wählen.

## Als App aufs Handy — und warum das ein Zertifikat braucht

„App installieren", Vollbild ohne Adressleiste und die Offline-Ansicht gibt es
nur über **https**. Und nicht über irgendein https: Der Browser muss dem
Zertifikat auch **trauen**. Eine weggeklickte Warnung reicht ihm nicht — er
bietet die Installation dann weiterhin nicht an.

Mitgeliefert ist nur ein Platzhalter-Zertifikat, das nicht einmal eure Adresse
enthält. Ein eigenes ist in drei Schritten gemacht.

### 1. Erzeugen — einmal, auf dem Gerät, auf dem das Dashboard läuft

```
bash scripts/make-cert.sh 192.168.1.20
make up
```

Statt `192.168.1.20` die Adresse eintragen, die ihr im Browser eintippt. Das
Skript legt eine kleine eigene Zertifizierungsstelle an und stellt damit ein
Zertifikat für genau diese Adresse aus.

### 2. Verteilen — über diese Kachel hier

Legt die Datei `traefik/certs/familie-ca.crt` in die Kachel **Dateien**. Dann
lädt sie jeder aus dem Dashboard herunter, ohne Kabel und ohne Mail.

Beim allerersten Mal über **Port 8088**, nicht 8443 — der verschlüsselte
Zugang warnt ja noch, das soll die Datei gerade beheben.

> **Nur `familie-ca.crt` wird verteilt.** Sie enthält ein Zertifikat und
> keinen Schlüssel. Geheim ist `familie-ca.key`, und die bleibt auf dem
> Server. Wer sie hat, kann Zertifikate ausstellen, denen eure Geräte glauben.

### 3. Einrichten — einmal pro Gerät

- **Android:** Einstellungen → Sicherheit → Verschlüsselung & Anmeldedaten →
  Zertifikat installieren → CA-Zertifikat → Trotzdem installieren, dann die
  Datei aus *Downloads* wählen.
- **iPhone/iPad:** Datei öffnen → Profil installieren. **Danach zusätzlich**
  Einstellungen → Allgemein → Info → Zertifikatsvertrauenseinstellungen und
  dort den Schalter umlegen. Ohne den zweiten Schritt bleibt die Warnung.
- **Windows:** Doppelklick → Zertifikat installieren → Lokaler Computer →
  Vertrauenswürdige Stammzertifizierungsstellen.
- **Linux:** Datei nach `/usr/local/share/ca-certificates/` kopieren, dann
  `sudo update-ca-certificates`.
- **Firefox** hat einen eigenen Speicher: Einstellungen → Zertifikate →
  Zertifikate anzeigen → Importieren.

Danach das Dashboard über **https** und Port **8443** aufrufen. Die Warnung
bleibt weg, und im Browsermenü steht „App installieren".

- **iPhone/iPad:** Safari → Teilen → „Zum Home-Bildschirm"
- **Android:** Chrome → Menü → „App installieren"
NOTE
  echo "  + backend/data/notes/willkommen.md"
  created=1
fi

if [ ! -f "$DATA/notes/einkaufsideen.md" ]; then
  cat > "$DATA/notes/einkaufsideen.md" <<'NOTE'
---
title: Rezepte & Ideen 🍽️
tags: [einkauf, ideen]
pinned: false
---

## Lieblingsgerichte
- Spaghetti Bolognese
- Pizza selbst gemacht
- Pfannkuchen zum Frühstück
- Chili con Carne
- Ofengemüse mit Feta

## Immer im Haus
Äpfel · Bananen · Joghurt · Müsli · Nüsse · Käse

## Haushalt
Spülmittel · Klopapier · Mülltüten · Waschmittel
NOTE
  echo "  + backend/data/notes/einkaufsideen.md"
  created=1
fi

touch "$DATA/photos/.gitkeep"
touch "$DATA/files/.gitkeep"
# Der Musikordner ist nur der Platzhalter für die Einhängung. Wer eine
# Sammlung hat, trägt ihren Pfad in der .env unter MUSIC_HOST_DIR ein.
touch "$DATA/music/.gitkeep"

if [ "$created" -eq 1 ]; then
  echo "✅ Beispieldaten angelegt."
else
  echo "✅ Datenverzeichnisse vorhanden."
fi
