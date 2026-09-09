#!/usr/bin/env bash
#
# Baut ein Release-Archiv aus dem aktuellen Stand.
#
# Das Archiv enthält ausschließlich Quellcode, Konfigurationsvorlagen und
# Dokumentation. Persönliche Daten bleiben garantiert draußen: keine .env,
# keine Datenbank, keine Notizen, keine Fotos, keine Backups.
#
#   bash scripts/make-release.sh            # Version aus VERSION
#   bash scripts/make-release.sh 1.2.0      # Version vorgeben
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-$(cat VERSION 2>/dev/null || echo 1.0.0)}"
NAME="familien-dashboard-pi-${VERSION}"
OUT="$ROOT/dist"
STAGE="$OUT/$NAME"

echo "▶ Baue Release $VERSION"

rm -rf "$STAGE"
mkdir -p "$STAGE"

# Nur diese Pfade wandern ins Archiv. Eine Positivliste statt einer
# Ausschlussliste: was neu dazukommt, landet nicht versehentlich im Release.
INCLUDE=(
  backend/main.go
  backend/go.mod
  backend/go.sum
  backend/config.yaml
  backend/Dockerfile
  backend/Dockerfile.dev
  backend/.air.toml
  backend/.dockerignore
  backend/internal
  frontend/src
  frontend/static
  frontend/package.json
  frontend/package-lock.json
  frontend/svelte.config.js
  frontend/vite.config.ts
  frontend/tailwind.config.js
  frontend/postcss.config.js
  frontend/tsconfig.json
  frontend/nginx.conf
  frontend/Dockerfile
  frontend/Dockerfile.dev
  frontend/.dockerignore
  traefik
  scripts
  docker-compose.yml
  docker-compose.dev.yml
  Makefile
  .env.example
  .gitignore
  LICENSE
  README.md
  INSTALL.md
  DEPLOY.md
  CHANGELOG.md
  SECURITY.md
  CONTRIBUTING.md
  docs
)

for path in "${INCLUDE[@]}"; do
  if [ ! -e "$path" ]; then
    echo "  ⚠️  fehlt, wird übersprungen: $path"
    continue
  fi
  mkdir -p "$STAGE/$(dirname "$path")"
  cp -r "$path" "$STAGE/$(dirname "$path")/"
done

# Leere Datenverzeichnisse anlegen, damit der erste Start funktioniert
mkdir -p "$STAGE"/backend/data/{notes,ics,photos,files,music,backup}
for dir in "$STAGE"/backend/data "$STAGE"/backend/data/*/; do
  touch "$dir/.gitkeep"
done

echo "$VERSION" > "$STAGE/VERSION"

# ── Sicherheitsnetz ──────────────────────────────────────────────────────────
# Lieber hier abbrechen als ein Archiv mit fremden Geheimnissen ausliefern.
echo "▶ Prüfe auf persönliche Daten..."
PROBLEME=0

if find "$STAGE" -name '.env' -not -name '.env.example' | grep -q .; then
  echo "  ❌ .env im Archiv gefunden"; PROBLEME=1
fi
if find "$STAGE" -name '*.sqlite*' | grep -q .; then
  echo "  ❌ Datenbank im Archiv gefunden"; PROBLEME=1
fi
if find "$STAGE/backend/data" -type f ! -name '.gitkeep' | grep -q .; then
  echo "  ❌ Dateien unter backend/data gefunden:"
  find "$STAGE/backend/data" -type f ! -name '.gitkeep' | sed 's/^/     /'
  PROBLEME=1
fi
if find "$STAGE" -type d -name node_modules | grep -q .; then
  echo "  ❌ node_modules im Archiv gefunden"; PROBLEME=1
fi

# Private IPv4-Adressen, die auf ein konkretes Heimnetz hindeuten. Die
# Beispieladressen im 192.168.1.x-Bereich sind bewusst erlaubt.
LEAKS="$(grep -rIn --exclude-dir=node_modules \
  -E '\b(192\.168\.(?!1\.)[0-9]+\.[0-9]+|10\.[0-9]+\.[0-9]+\.[0-9]+)\b' -P "$STAGE" 2>/dev/null || true)"
if [ -n "$LEAKS" ]; then
  echo "  ⚠️  Möglicherweise private Adressen (bitte prüfen):"
  echo "$LEAKS" | sed 's|'"$STAGE"'/|     |' | head -10
fi

if [ "$PROBLEME" -ne 0 ]; then
  echo ""
  echo "❌ Abgebrochen. Das Archiv wurde NICHT erstellt."
  exit 1
fi
echo "  ✅ Keine persönlichen Daten gefunden"

# ── Packen ───────────────────────────────────────────────────────────────────
cd "$OUT"
tar -czf "${NAME}.tar.gz" "$NAME"
if command -v zip >/dev/null 2>&1; then
  zip -qr "${NAME}.zip" "$NAME"
fi

# Prüfsummen für die Download-Seite
sha256sum "${NAME}.tar.gz" > "${NAME}.tar.gz.sha256"
[ -f "${NAME}.zip" ] && sha256sum "${NAME}.zip" > "${NAME}.zip.sha256"

rm -rf "$STAGE"

echo ""
echo "✅ Fertig:"
ls -lh "$OUT"/${NAME}.* | awk '{print "   " $9 "  (" $5 ")"}'
echo ""
echo "SHA-256:"
cat "$OUT/${NAME}.tar.gz.sha256" | awk '{print "   " $1}'
