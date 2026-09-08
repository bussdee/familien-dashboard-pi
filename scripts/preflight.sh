#!/usr/bin/env bash
#
# Vorabprüfung: Passt das Familien Dashboard auf dieses Gerät, ohne dort
# etwas anderes zu stören?
#
# Das Skript ändert NICHTS. Es schaut nur nach und meldet, was im Weg ist.
# Auf dem Zielgerät ausführen, bevor zum ersten Mal deployt wird:
#
#   bash scripts/preflight.sh
#
set -uo pipefail

HTTP_PORT="${HTTP_PORT:-8088}"
HTTPS_PORT="${HTTPS_PORT:-8443}"

rot="\033[31m"; gruen="\033[32m"; gelb="\033[33m"; grau="\033[90m"; aus="\033[0m"
fehler=0
warnungen=0

ok()      { printf "  ${gruen}✓${aus} %s\n" "$1"; }
warnung() { printf "  ${gelb}!${aus} %s\n" "$1"; warnungen=$((warnungen + 1)); }
problem() { printf "  ${rot}✗${aus} %s\n" "$1"; fehler=$((fehler + 1)); }
hinweis() { printf "    ${grau}%s${aus}\n" "$1"; }

echo ""
echo "Vorabprüfung für das Familien Dashboard"
echo "auf $(hostname) · $(uname -m) · $(date '+%d.%m.%Y %H:%M')"
echo ""

# ─────────────────────────────────────────────────────────── Docker
echo "Docker"
if ! command -v docker >/dev/null 2>&1; then
  problem "Docker ist nicht installiert"
  hinweis "curl -fsSL https://get.docker.com | sh"
elif ! docker ps >/dev/null 2>&1; then
  problem "Docker läuft, aber dein Benutzer darf es nicht bedienen"
  hinweis "sudo usermod -aG docker \$USER  — danach ab- und wieder anmelden"
else
  ok "Docker $(docker version --format '{{.Server.Version}}' 2>/dev/null)"
  if docker compose version >/dev/null 2>&1; then
    ok "Compose $(docker compose version --short 2>/dev/null)"
  else
    problem "Das Compose-Plugin fehlt"
    hinweis "sudo apt install docker-compose-plugin"
  fi
fi
echo ""

# ─────────────────────────────────────────────────────────── Ports
echo "Ports $HTTP_PORT und $HTTPS_PORT"

# Lauscht dort etwas? ss zeigt den Prozess nur als root, daher ohne Anspruch.
lauscht() {
  command -v ss >/dev/null 2>&1 &&
    ss -ltn 2>/dev/null | awk -v p=":$1\$" '$4 ~ p {found=1} END {exit !found}'
}

# Wer hält den Port – ein eigener Container aus einem früheren Deploy oder
# etwas Fremdes? Das ist der Unterschied zwischen "alles gut" und "Konflikt".
haelt_container() {
  docker ps --format '{{.Names}}\t{{.Ports}}' 2>/dev/null |
    awk -v p=":$1->" '$0 ~ p {print $1; exit}'
}

for port in "$HTTP_PORT" "$HTTPS_PORT"; do
  if ! lauscht "$port"; then
    ok "Port $port ist frei"
    continue
  fi

  wer="$(haelt_container "$port")"
  case "$wer" in
    family-*)
      ok "Port $port hält dein eigener Container $wer — wird beim Deploy ersetzt"
      ;;
    "")
      problem "Port $port ist belegt (kein Docker-Container — ein Dienst auf dem Host)"
      hinweis "Wer es ist: sudo ss -ltnp | grep :$port"
      hinweis "Sonst in der .env einen freien Port eintragen (HTTP_PORT / HTTPS_PORT)"
      ;;
    *)
      problem "Port $port ist von Container \"$wer\" belegt"
      hinweis "In der .env einen freien Port eintragen (HTTP_PORT / HTTPS_PORT)"
      ;;
  esac
done
echo ""

# ─────────────────────────────────── Namenskonflikte mit vorhandenen Containern
echo "Vorhandene Container und Netzwerke"
if docker ps >/dev/null 2>&1; then
  for name in family-traefik family-backend family-frontend; do
    if docker ps -a --format '{{.Names}}' | grep -qx "$name"; then
      # Ein Container aus einem früheren Deploy ist in Ordnung.
      if docker inspect "$name" --format '{{index .Config.Labels "com.docker.compose.project"}}' 2>/dev/null \
         | grep -qiE 'family|dashboard'; then
        ok "$name existiert bereits (früherer Deploy) — wird ersetzt"
      else
        problem "Ein fremder Container heißt bereits $name"
        hinweis "container_name in docker-compose.yml ändern"
      fi
    else
      ok "Name $name ist frei"
    fi
  done

  if docker network ls --format '{{.Name}}' | grep -qx "family-net"; then
    ok "Netzwerk family-net existiert bereits — wird wiederverwendet"
  else
    ok "Netzwerkname family-net ist frei"
  fi

  laufend="$(docker ps --format '{{.Names}}' | grep -vE '^family-' | tr '\n' ' ')"
  if [ -n "$laufend" ]; then
    printf "  ${grau}Läuft hier sonst noch: %s${aus}\n" "$laufend"
    hinweis "Diese Container werden nicht angefasst."
  fi
fi
echo ""

# ─────────────────────────────────────────────────────────── Arbeitsspeicher
echo "Arbeitsspeicher"
if [ -r /proc/meminfo ]; then
  gesamt_kb=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
  frei_kb=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)
  swap_kb=$(awk '/^SwapTotal:/{print $2}' /proc/meminfo)
  gesamt=$((gesamt_kb / 1024)); frei=$((frei_kb / 1024)); swap=$((swap_kb / 1024))

  printf "  ${grau}%s MB gesamt, %s MB verfügbar, %s MB Swap${aus}\n" "$gesamt" "$frei" "$swap"

  # Der Frontend-Build braucht kurzzeitig rund 1 GB. Reicht das nicht, greift
  # der OOM-Killer — und der trifft womöglich einen anderen Dienst, nicht den
  # Build. Deshalb ist das hier ein echtes Problem und keine Kleinigkeit.
  if [ $((frei + swap)) -lt 1200 ]; then
    problem "Zu wenig Speicher für den Build ($((frei + swap)) MB verfügbar + Swap)"
    hinweis "Der Build braucht ~1 GB. Ohne Reserve kann der OOM-Killer"
    hinweis "einen ANDEREN Dienst beenden — etwa Plex."
    hinweis "Lösung: Swap einschalten oder woanders bauen (siehe DEPLOY.md)"
  elif [ $((frei + swap)) -lt 2000 ]; then
    warnung "Knapp: $((frei + swap)) MB verfügbar inklusive Swap"
    hinweis "Es geht wahrscheinlich, aber sicherer mit mehr Swap."
  else
    ok "Genug Speicher für den Build"
  fi
fi
echo ""

# ─────────────────────────────────────────────────────────── Speicherplatz
echo "Speicherplatz"
docker_dir="$(docker info --format '{{.DockerRootDir}}' 2>/dev/null || echo /var/lib/docker)"
frei_mb="$(df -Pm "$docker_dir" 2>/dev/null | awk 'NR==2{print $4}')"
if [ -n "$frei_mb" ]; then
  printf "  ${grau}%s MB frei unter %s${aus}\n" "$frei_mb" "$docker_dir"
  if [ "$frei_mb" -lt 3000 ]; then
    problem "Zu wenig Platz — Bauen braucht rund 3 GB Puffer"
    hinweis "Läuft die Platte voll, leiden ALLE Container, nicht nur dieser."
    hinweis "Aufräumen: docker image prune -f"
  elif [ "$frei_mb" -lt 6000 ]; then
    warnung "Es passt, aber viel Reserve bleibt nicht"
  else
    ok "Genug Platz"
  fi
fi
echo ""

# ─────────────────────────────────────────────────────────── Internet (Wetter)
echo "Internet (nur fürs Wetter)"
if curl -fsS --max-time 8 -o /dev/null "https://api.open-meteo.com/v1/forecast?latitude=48&longitude=16&current=temperature_2m" 2>/dev/null; then
  ok "api.open-meteo.com ist erreichbar"
else
  warnung "api.open-meteo.com nicht erreichbar — alles außer dem Wetter läuft trotzdem"
fi
echo ""

# ─────────────────────────────────────────────────────────── Zusammenfassung
echo "─────────────────────────────────────────────"
if [ "$fehler" -gt 0 ]; then
  printf "${rot}%d Problem(e) gefunden.${aus} Bitte erst beheben.\n" "$fehler"
  [ "$warnungen" -gt 0 ] && printf "Dazu %d Hinweis(e).\n" "$warnungen"
  echo ""
  exit 1
fi

if [ "$warnungen" -gt 0 ]; then
  printf "${gelb}Bereit, mit %d Hinweis(en).${aus}\n" "$warnungen"
else
  printf "${gruen}Alles bereit.${aus}\n"
fi
echo ""
echo "Weiter mit:  make setup && make up"
echo ""
