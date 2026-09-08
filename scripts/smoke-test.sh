#!/usr/bin/env bash
# Smoke-Test gegen den laufenden Stack (make up).
# Prüft die Kette Traefik -> nginx -> Go-Backend inklusive Login und WebSocket.
set -uo pipefail

BASE="${BASE:-http://localhost:${HTTP_PORT:-8088}}"
JAR="$(mktemp)"
trap 'rm -f "$JAR"' EXIT

pass=0
fail=0

check() {
  local label="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    printf '  \033[32m✓\033[0m %-46s %s\n' "$label" "$actual"
    pass=$((pass + 1))
  else
    printf '  \033[31m✗\033[0m %-46s %s (erwartet %s)\n' "$label" "$actual" "$expected"
    fail=$((fail + 1))
  fi
}

code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

echo ""
echo "Smoke-Test gegen $BASE"
echo ""

echo "Erreichbarkeit"
check "SPA wird ausgeliefert"          200 "$(code "$BASE/")"
check "SPA-Fallback für /settings"     200 "$(code "$BASE/settings")"
check "SPA-Fallback für /rangliste"    200 "$(code "$BASE/rangliste")"
check "SPA-Fallback für /links"        200 "$(code "$BASE/links")"
check "SPA-Fallback für /ansicht"      200 "$(code "$BASE/ansicht")"
check "SPA-Fallback für /wetter"       200 "$(code "$BASE/wetter")"
check "Manifest"                       200 "$(code "$BASE/manifest.json")"
check "Service Worker"                 200 "$(code "$BASE/service-worker.js")"
check "App-Icon"                       200 "$(code "$BASE/icons/icon-192.png")"
check "Backend-Health über /api"       200 "$(code "$BASE/api/health")"

echo ""
echo "Authentifizierung"
check "Benutzerliste ist öffentlich"   200 "$(code "$BASE/api/auth/users")"
check "Geschützt ohne Login (401)"     401 "$(code "$BASE/api/shopping")"
check "Falsche PIN wird abgewiesen"    401 \
  "$(code -X POST -H 'Content-Type: application/json' -d '{"user_id":1,"pin":"9999"}' "$BASE/api/auth/login")"

# Pick an admin: the later checks need admin-only endpoints. A greedy sed here
# used to grab the LAST id in the array, which was a member.
USER_ID="$(curl -s "$BASE/api/auth/users" | python3 -c '
import json, sys
users = json.load(sys.stdin)
admins = [u["id"] for u in users if u["role"] == "admin"]
print((admins or [u["id"] for u in users] or [1])[0])
' 2>/dev/null || echo 1)"
LOGIN="$(code -c "$JAR" -X POST -H 'Content-Type: application/json' \
  -d "{\"user_id\":${USER_ID:-1},\"pin\":\"1234\"}" "$BASE/api/auth/login")"
check "Login mit Standard-PIN"         200 "$LOGIN"

if [ "$LOGIN" != "200" ]; then
  echo ""
  echo "  ⚠️  Login fehlgeschlagen – die PIN wurde vermutlich schon geändert."
  echo "     Die folgenden Prüfungen werden übersprungen."
else
  echo ""
  echo "Daten-Endpunkte (angemeldet)"
  for ep in me:/api/auth/me weather:/api/weather calendar:/api/calendar \
            shopping:/api/shopping notes:/api/notes chores:/api/chores \
            scoreboard:/api/scoreboard history:/api/scoreboard/history \
            links:/api/links layout:/api/preferences/dashboard.layout \
            reward:/api/shopping/reward location:/api/weather/location \
            devices:/api/devices photos:/api/photos; do
    name="${ep%%:*}"; path="${ep#*:}"
    status="$(code -b "$JAR" "$BASE$path")"
    # Wetter darf 503 sein, wenn der Server (noch) kein Internet hatte.
    if [ "$name" = "weather" ] && [ "$status" = "503" ]; then
      printf '  \033[33m!\033[0m %-46s 503 (kein Internet – zulässig)\n' "$path"
    else
      check "$path" 200 "$status"
    fi
  done

  echo ""
  echo "Schreiben"
  CREATE="$(curl -s -b "$JAR" -X POST -H 'Content-Type: application/json' \
    -d '{"name":"Smoke-Test","quantity":"1"}' "$BASE/api/shopping")"
  ITEM_ID="$(printf '%s' "$CREATE" | sed -n 's/.*"id":\([0-9]*\).*/\1/p' | head -1)"
  check "Einkaufs-Eintrag anlegen"     201 \
    "$(code -b "$JAR" -X POST -H 'Content-Type: application/json' -d '{"name":"Smoke-Test-2"}' "$BASE/api/shopping")"
  if [ -n "$ITEM_ID" ]; then
    check "Eintrag wieder löschen"     204 "$(code -b "$JAR" -X DELETE "$BASE/api/shopping/$ITEM_ID")"
  fi
  # Aufräumen: jeden Eintrag entfernen, den dieser Test angelegt hat.
  removed=0
  while read -r id; do
    [ -n "$id" ] || continue
    curl -s -o /dev/null -b "$JAR" -X DELETE "$BASE/api/shopping/$id"
    removed=$((removed + 1))
  done < <(curl -s -b "$JAR" "$BASE/api/shopping" \
    | python3 -c 'import json,sys
for item in json.load(sys.stdin):
    if item["name"].startswith("Smoke-Test"):
        print(item["id"])' 2>/dev/null)
  printf '  \033[32m✓\033[0m %-46s %s\n' "Testdaten wieder entfernt" "$removed"
  pass=$((pass + 1))

  echo ""
  echo "Adminbereich"
  check "Geräteliste"                  200 "$(code -b "$JAR" "$BASE/api/admin/devices")"
  check "Gerät testen (ungültige URL)" 400 \
    "$(code -b "$JAR" -X POST -H 'Content-Type: application/json' \
       -d '{"name":"T","type":"http","url":"kaputt"}' "$BASE/api/admin/devices/test")"

  echo ""
  echo "WebSocket"
  WS="$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" \
    -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
    -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: c21va2V0ZXN0MTIzNDU2Nw==' \
    -H "Origin: $BASE" "$BASE/api/shopping/ws")"
  check "Handshake vom eigenen Origin"  101 "$WS"
  WS_EVIL="$(curl -s -o /dev/null -w '%{http_code}' -b "$JAR" \
    -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
    -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: c21va2V0ZXN0MTIzNDU2Nw==' \
    -H 'Origin: http://angreifer.example' "$BASE/api/shopping/ws")"
  check "Fremder Origin wird blockiert" 403 "$WS_EVIL"
fi

echo ""
if [ "$fail" -eq 0 ]; then
  printf '\033[32m✅ %d Prüfungen bestanden.\033[0m\n\n' "$pass"
  exit 0
fi
printf '\033[31m❌ %d von %d Prüfungen fehlgeschlagen.\033[0m\n\n' "$fail" "$((pass + fail))"
exit 1
