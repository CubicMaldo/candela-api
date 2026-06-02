# Makefile — Comandos del proyecto Nomad API
# Uso: make <comando>

.PHONY: help up down logs ps build migrate-up migrate-down \
        migrate-create shell-api shell-db tidy test

# ── Variables ────────────────────────────────────────────
COMPOSE = docker compose
MIGRATE = $(COMPOSE) exec api go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
          -path /app/db/migrations \
          -database "$$DATABASE_URL"

# ── Ayuda ────────────────────────────────────────────────
help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Docker Compose ───────────────────────────────────────
up: ## Levanta todos los servicios
	$(COMPOSE) up -d
	@echo ""
	@echo "✅ Servicios corriendo:"
	@echo "   API     → http://localhost:8080"
	@echo "   Adminer → http://localhost:8081"
	@echo "   DB      → localhost:5432"
	@echo ""

down: ## Detiene y elimina los contenedores (datos persisten en volumes)
	$(COMPOSE) down

down-v: ## Detiene y elimina contenedores Y volumes (borra datos)
	$(COMPOSE) down -v

logs: ## Muestra logs de todos los servicios (Ctrl+C para salir)
	$(COMPOSE) logs -f

logs-api: ## Muestra solo logs de la API
	$(COMPOSE) logs -f api

ps: ## Estado de los contenedores
	$(COMPOSE) ps

build: ## Reconstruye la imagen del API (tras cambiar Dockerfile.dev)
	$(COMPOSE) build api

# ── Migraciones ──────────────────────────────────────────
migrate-up: ## Aplica todas las migraciones pendientes
	$(COMPOSE) exec api sh -c 'migrate -path /app/db/migrations -database "$$DATABASE_URL" up'

migrate-down: ## Revierte la última migración
	$(COMPOSE) exec api sh -c 'migrate -path /app/db/migrations -database "$$DATABASE_URL" down 1'

migrate-drop: ## ⚠️ Elimina TODAS las tablas (usa solo en dev)
	$(COMPOSE) exec api sh -c 'migrate -path /app/db/migrations -database "$$DATABASE_URL" drop -f'

migrate-create: ## Crea nuevos archivos de migración. Uso: make migrate-create name=add_notifications
	@read -p "Nombre de la migración: " name; \
	$(COMPOSE) exec api sh -c "migrate create -ext sql -dir /app/db/migrations -seq $$name"

# ── Shells ───────────────────────────────────────────────
shell-api: ## Abre shell dentro del contenedor API
	$(COMPOSE) exec api sh

shell-db: ## Abre psql dentro del contenedor PostgreSQL
	$(COMPOSE) exec postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB

redis-cli: ## Abre redis-cli
	$(COMPOSE) exec redis redis-cli -a $$REDIS_PASSWORD

# ── Go ───────────────────────────────────────────────────
tidy: ## go mod tidy dentro del contenedor
	$(COMPOSE) exec api go mod tidy

test: ## Corre tests
	$(COMPOSE) exec api go test ./...

# ── Setup inicial ────────────────────────────────────────
setup: ## ⭐ Primer uso: copia .env y levanta servicios
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "📝 .env creado desde .env.example — edita los valores antes de continuar"; \
	else \
		echo "ℹ️  .env ya existe, no se sobreescribe"; \
	fi
	@echo ""
	@echo "Próximo paso: edita .env y luego ejecuta: make up"
