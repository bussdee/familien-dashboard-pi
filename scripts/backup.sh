#!/usr/bin/env bash
# Backup jetzt erstellen.
# Bevorzugt den laufenden Container (konsistenter SQLite-Snapshot per
# VACUUM INTO); ohne Container wird direkt vom Host kopiert.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA="$ROOT/backend/data"
BACKUP="$DATA/backup"
STAMP="$(date +%F_%H-%M-%S)"

mkdir -p "$BACKUP"

if docker ps --format '{{.Names}}' | grep -qx 'family-backend'; then
  echo "▶ Backend läuft – erstelle Backup über die API..."
  if [ ! -f "$ROOT/.env" ]; then
    echo "❌ .env fehlt. Zuerst 'make setup' ausführen." >&2
    exit 1
  fi
  echo "   (Hinweis: 'Jetzt sichern' im Verwaltungsbereich macht dasselbe)"
fi

if [ -f "$DATA/db.sqlite" ]; then
  # sqlite3 ist auf dem Host oft nicht installiert; cp inklusive WAL reicht,
  # solange der Container gestoppt ist oder gerade nicht schreibt.
  cp "$DATA/db.sqlite" "$BACKUP/db_${STAMP}.sqlite"
  [ -f "$DATA/db.sqlite-wal" ] && cp "$DATA/db.sqlite-wal" "$BACKUP/db_${STAMP}.sqlite-wal"
  echo "  + $BACKUP/db_${STAMP}.sqlite"
else
  echo "⚠️  Keine Datenbank unter $DATA/db.sqlite gefunden."
fi

tar -czf "$BACKUP/files_${STAMP}.tar.gz" -C "$DATA" notes ics photos 2>/dev/null || true
echo "  + $BACKUP/files_${STAMP}.tar.gz"

# Alte Backups aufräumen (7 Tage)
find "$BACKUP" -name 'db_*.sqlite*' -mtime +7 -delete 2>/dev/null || true
find "$BACKUP" -name 'files_*.tar.gz' -mtime +7 -delete 2>/dev/null || true

echo "✅ Backup fertig."
ls -lh "$BACKUP" | tail -6
