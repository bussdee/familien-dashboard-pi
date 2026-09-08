#!/usr/bin/env bash
# Spielt das neueste Backup zurück. Fragt vorher nach, weil dabei der
# aktuelle Stand überschrieben wird.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA="$ROOT/backend/data"
BACKUP="$DATA/backup"

LATEST_DB="$(ls -t "$BACKUP"/db_*.sqlite 2>/dev/null | head -1 || true)"
LATEST_FILES="$(ls -t "$BACKUP"/files_*.tar.gz 2>/dev/null | head -1 || true)"

if [ -z "$LATEST_DB" ] && [ -z "$LATEST_FILES" ]; then
  echo "❌ Keine Backups in $BACKUP gefunden." >&2
  exit 1
fi

echo "Wiederherstellen:"
[ -n "$LATEST_DB" ]    && echo "  Datenbank: $(basename "$LATEST_DB")"
[ -n "$LATEST_FILES" ] && echo "  Dateien:   $(basename "$LATEST_FILES")"
echo ""
echo "⚠️  Der aktuelle Stand wird überschrieben."
read -r -p "Fortfahren? [ja/NEIN] " confirm
[ "$confirm" = "ja" ] || { echo "Abgebrochen."; exit 1; }

if docker ps --format '{{.Names}}' | grep -qx 'family-backend'; then
  echo "▶ Stoppe Backend..."
  docker stop family-backend >/dev/null
  RESTART=1
fi

if [ -n "$LATEST_DB" ]; then
  # Alte WAL/SHM entfernen, sonst mischt SQLite alten und neuen Stand.
  rm -f "$DATA/db.sqlite" "$DATA/db.sqlite-wal" "$DATA/db.sqlite-shm"
  cp "$LATEST_DB" "$DATA/db.sqlite"
  echo "  ✓ Datenbank zurückgespielt"
fi

if [ -n "$LATEST_FILES" ]; then
  tar -xzf "$LATEST_FILES" -C "$DATA"
  echo "  ✓ Dateien zurückgespielt"
fi

if [ "${RESTART:-0}" = "1" ]; then
  echo "▶ Starte Backend..."
  docker start family-backend >/dev/null
fi

echo "✅ Wiederherstellung abgeschlossen."
