# Family Dashboard
# ⚠️  Der Deploy auf den Pi ist bewusst durch eine doppelte Bestätigung geschützt.

SHELL := /bin/bash
COMPOSE := docker compose
# Zielsystem für den Deploy. Überschreibbar, ohne die Datei zu ändern:
#   make deploy PI_HOST=pi@192.168.1.20
# Dauerhaft: PI_HOST in die .env schreiben und mit `export` bereitstellen.
PI_HOST ?= pi@raspberrypi.local
PI_PATH ?= ~/family-dashboard

.DEFAULT_GOAL := help
.PHONY: _rsync help setup init-data preflight check dev dev-down build up down restart logs ps \
        backup restore verify deploy _deploy_exec pi-init pi-verify pi-logs \
        pi-shell pi-status clean

help:
	@echo ""
	@echo "Familien Dashboard"
	@echo ""
	@echo "  Erste Schritte"
	@echo "    make setup       .env anlegen (JWT_SECRET wird erzeugt) + Datenverzeichnisse"
	@echo "    make preflight   Prüfen, ob dieses Gerät passt (ändert nichts)"
	@echo "    make check       Backend kompilieren + Frontend typprüfen (kein Docker-Start)"
	@echo ""
	@echo "  Entwickeln"
	@echo "    make dev         Vite HMR + Go Hot-Reload    -> http://localhost:5173"
	@echo "    make dev-down    Dev-Stack stoppen"
	@echo ""
	@echo "  Testen wie in Produktion (LAN)"
	@echo "    make up          Gebaute Images starten      -> http://localhost:8088"
	@echo "    make verify      Smoke-Test gegen den laufenden Stack"
	@echo "    make logs        Logs verfolgen"
	@echo "    make down        Stack stoppen"
	@echo ""
	@echo "  Daten"
	@echo "    make backup      Backup jetzt erstellen"
	@echo "    make restore     Neuestes Backup einspielen"
	@echo ""
	@echo "  Deploy auf den Pi (siehe DEPLOY.md)"
	@echo "    make pi-init     Erstinstallation: nur Code übertragen, nichts starten"
	@echo "    make deploy      Übertragen + starten (fragt zweimal nach)"
	@echo "    make pi-verify   Smoke-Test gegen den Pi"
	@echo "    make pi-status / pi-logs / pi-shell"
	@echo ""
	@echo "  make clean       Container + ungenutzte Images entfernen"
	@echo ""

# =============================================================================
# SETUP
# =============================================================================

setup: init-data
	@if [ ! -f .env ]; then \
		echo "Erzeuge .env mit frischem JWT_SECRET..."; \
		SECRET=$$(openssl rand -base64 48 | tr -d '\n'); \
		sed -e "s|^JWT_SECRET=.*|JWT_SECRET=$$SECRET|" \
		    -e "s|^PUID=.*|PUID=$$(id -u)|" \
		    -e "s|^PGID=.*|PGID=$$(id -g)|" .env.example > .env; \
		chmod 600 .env; \
		echo "✅ .env angelegt (JWT_SECRET erzeugt, Kennung $$(id -u):$$(id -g))"; \
	else \
		echo "ℹ️  .env existiert bereits – unverändert gelassen"; \
	fi
	@echo ""
	@echo "Nächster Schritt:  make dev   (oder: make up)"

init-data:
	@mkdir -p backend/data/notes backend/data/ics backend/data/photos backend/data/backup
	@bash scripts/init-data.sh

# =============================================================================
# QUALITÄTSPRÜFUNG (ohne laufenden Stack)
# =============================================================================

# Prüft das Gerät, bevor zum ersten Mal gestartet wird. Ändert nichts.
preflight:
	@bash scripts/preflight.sh

check:
	@echo "▶ Backend kompilieren + vet..."
	@docker run --rm -v "$(PWD)/backend":/src -w /src golang:1.23-alpine \
		sh -c "go build ./... && go vet ./..."
	@echo "✅ Backend OK"
	@echo "▶ Frontend typprüfen..."
	@docker run --rm -v "$(PWD)/frontend":/app -w /app node:20-alpine \
		sh -c "npm ci --silent && npm run check"
	@echo "✅ Frontend OK"

# =============================================================================
# ENTWICKLUNG
# =============================================================================

dev: init-data
	@$(COMPOSE) -f docker-compose.dev.yml up --build

dev-down:
	@$(COMPOSE) -f docker-compose.dev.yml down

# =============================================================================
# PRODUKTIONSNAHER STACK
# =============================================================================

build:
	@$(COMPOSE) build

up: init-data
	@$(COMPOSE) up -d --build
	@echo ""
	@echo "Dashboard:  http://localhost:$${HTTP_PORT:-8088}"
	@echo "Im LAN:     http://$$(hostname -I | awk '{print $$1}'):$${HTTP_PORT:-8088}"
	@echo ""
	@echo "Standard-PIN aller Benutzer: 1234  (bitte in den Einstellungen ändern)"

down:
	@$(COMPOSE) down

restart:
	@$(COMPOSE) restart

logs:
	@$(COMPOSE) logs -f --tail=100

ps:
	@$(COMPOSE) ps

verify:
	@bash scripts/smoke-test.sh

# =============================================================================
# DATEN
# =============================================================================

backup:
	@bash scripts/backup.sh

restore:
	@bash scripts/restore.sh

# =============================================================================
# DEPLOY AUF DEN PI – GESCHÜTZT DURCH DOPPELTE BESTÄTIGUNG
# =============================================================================

# Erstinstallation: überträgt nur den Code. Nichts wird gebaut oder gestartet,
# damit auf dem Pi in Ruhe "make setup" laufen kann.
pi-init:
	@echo "📦 Übertrage Code nach $(PI_HOST):$(PI_PATH) (ohne Start)..."
	@$(MAKE) --no-print-directory _rsync
	@echo ""
	@echo "✅ Code liegt auf dem Pi. Weiter mit:"
	@echo "   ssh $(PI_HOST)"
	@echo "   cd $(PI_PATH) && make setup"
	@echo "   nano .env      # BIND_ADDR auf die LAN-Adresse setzen"
	@echo ""
	@echo "   Danach von hier:  make deploy"

_rsync:
	@rsync -az --delete \
		--exclude '.git' \
		--exclude 'node_modules' \
		--exclude '.svelte-kit' \
		--exclude 'frontend/build' \
		--exclude '.gocache' \
		--exclude 'backend/tmp' \
		--exclude 'backend/data' \
		--exclude '.env' \
		--exclude '.local' \
		--exclude 'dist' \
		--exclude '*.log' \
		./ $(PI_HOST):$(PI_PATH)/

_deploy_exec:
	@echo "🚀 Übertrage Code auf $(PI_HOST)..."
	@$(MAKE) --no-print-directory _rsync
	@echo "🔧 Baue und starte auf dem Pi..."
	@ssh $(PI_HOST) "cd $(PI_PATH) && test -f .env || (echo '❌ .env fehlt auf dem Pi – dort einmalig \"make setup\" ausführen'; exit 1)"
	@ssh $(PI_HOST) "cd $(PI_PATH) && mkdir -p backend/data/{notes,ics,photos,backup} && docker compose up -d --build"
	@echo "✅ Deploy fertig — Dashboard auf dem Zielsystem, Port $${HTTP_PORT:-8088}"

deploy:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════════════╗"
	@echo "║  ⚠️   DEPLOY AUF $(PI_HOST)"
	@echo "║                                                                      ║"
	@echo "║    Bestehende Dienste auf dem Zielsystem bleiben unberührt.           ║"
	@echo "║  Das Dashboard belegt ausschließlich 8088 (HTTP) und 8443 (HTTPS).   ║"
	@echo "║                                                                      ║"
	@echo "║  VORAUSSETZUNG: lokal getestet mit 'make up' und 'make verify'.      ║"
	@echo "╚══════════════════════════════════════════════════════════════════════╝"
	@echo ""
	@read -p "❓ BESTÄTIGUNG 1/2: Lokale Version getestet und freigegeben? [ja/NEIN] " c1; \
	  if [ "$$c1" != "ja" ]; then echo "❌ Abgebrochen."; exit 1; fi
	@read -p "❓ BESTÄTIGUNG 2/2: Deploy auf den Pi JETZT durchführen? [ja/NEIN] " c2; \
	  if [ "$$c2" != "ja" ]; then echo "❌ Abgebrochen."; exit 1; fi
	@echo ""
	@$(MAKE) _deploy_exec

# Smoke-Test gegen das Zielsystem. Host aus PI_HOST ableiten (Teil nach dem @).
pi-verify:
	@HOST=$$(echo "$(PI_HOST)" | sed 's/.*@//'); \
	 BASE=http://$$HOST:$${HTTP_PORT:-8088} bash scripts/smoke-test.sh

pi-status:
	@ssh $(PI_HOST) "cd $(PI_PATH) && docker compose ps"

pi-logs:
	@ssh $(PI_HOST) "cd $(PI_PATH) && docker compose logs -f --tail=100"

pi-shell:
	@ssh -t $(PI_HOST) "cd $(PI_PATH) && docker compose exec backend sh"

# =============================================================================
# WARTUNG
# =============================================================================

clean:
	@$(COMPOSE) down --remove-orphans
	@$(COMPOSE) -f docker-compose.dev.yml down --remove-orphans 2>/dev/null || true
	@docker image prune -f
