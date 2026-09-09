#!/usr/bin/env bash
# Legt die Datenverzeichnisse an und füllt sie beim ersten Mal mit Beispielen.
# Idempotent: vorhandene Dateien werden nie überschrieben.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA="$ROOT/backend/data"

mkdir -p "$DATA"/{notes,ics,photos,files,music,backup}

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

## Als App installieren
- **iPhone/iPad:** Safari → Teilen → „Zum Home-Bildschirm"
- **Android:** Chrome → Menü → „App installieren"

## Wichtig zum Start
Alle starten mit der PIN **1234**. Bitte gleich unter
**Einstellungen → PIN ändern** eine eigene wählen.
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
