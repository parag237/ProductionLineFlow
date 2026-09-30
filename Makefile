.PHONY: help setup docker-start infra infra-down migrate db dev backend frontend logs

API_DIR := productionlineflow-api
FRONTEND_DIR := productionlineflow-frontend

help:
	@echo "Available targets:"
	@echo "  make setup     Install frontend dependencies"
	@echo "  make docker-start Check Docker and start Docker Desktop if needed"
	@echo "  make infra     Start local PostgreSQL and Redis with Docker Compose"
	@echo "  make infra-down Stop local PostgreSQL and Redis"
	@echo "  make migrate   Apply database migrations and development seed data"
	@echo "  make db        Open an interactive PostgreSQL shell"
	@echo "  make dev       Run backend and web frontend together"
	@echo "  make backend   Run the backend API"
	@echo "  make logs      Follow backend API logs"
	@echo "  make frontend Run the web frontend"

setup:
	cd $(FRONTEND_DIR) && pnpm install

docker-start:
	@if docker info >/dev/null 2>&1; then \
		echo "Docker is already running"; \
	else \
		if echo "$${DOCKER_HOST:-}" | grep -q '\.colima/'; then \
			if ! command -v colima >/dev/null 2>&1; then echo "Colima is selected but not installed"; exit 1; fi; \
			echo "Starting Colima"; \
			colima start || exit 1; \
		else \
			echo "Starting Docker Desktop"; \
			open -a Docker || exit 1; \
		fi; \
		attempt=0; \
		until docker info >/dev/null 2>&1; do \
			attempt=$$((attempt + 1)); \
			if [ $$attempt -ge 60 ]; then echo "Docker did not become ready within 120 seconds"; exit 1; fi; \
			sleep 2; \
		done; \
		echo "Docker is ready"; \
	fi

infra: docker-start
	docker compose -f $(API_DIR)/docker-compose.yml up -d --wait postgres redis

infra-down:
	docker compose -f $(API_DIR)/docker-compose.yml stop postgres redis

migrate: infra
	@set -eu; \
	compose='docker compose -f $(API_DIR)/docker-compose.yml'; \
	$$compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d warehouse_dev \
		-c 'CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())'; \
	baseline=$$($$compose exec -T postgres psql -U app -d warehouse_dev -tAc \
		"SELECT to_regclass('public.companies') IS NOT NULL AND to_regclass('public.platform_users') IS NOT NULL AND to_regclass('public.platform_refresh_tokens') IS NOT NULL AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='companies' AND column_name='activated_at')"); \
	tracked=$$($$compose exec -T postgres psql -U app -d warehouse_dev -tAc 'SELECT COUNT(*) FROM schema_migrations'); \
	if [ "$$baseline" = "t" ] && [ "$$tracked" = "0" ]; then \
		echo "Existing schema is complete; recording its migration baseline"; \
		$$compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d warehouse_dev \
			-c "INSERT INTO schema_migrations (version) VALUES ('000001_initial'), ('000002_platform_refresh_tokens'), ('000003_company_management'), ('000004_company_lifecycle_times') ON CONFLICT DO NOTHING"; \
	fi; \
	for migration in $(API_DIR)/db/migrations/*.up.sql; do \
		version=$$(basename "$$migration" .up.sql); \
		applied=$$($$compose exec -T postgres psql -U app -d warehouse_dev -tAc \
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = '$$version')"); \
		if [ "$$applied" = "t" ]; then \
			echo "Skipping applied migration $$version"; \
		else \
			echo "Applying migration $$version"; \
			{ printf 'BEGIN;\n'; cat "$$migration"; \
			  printf "\nINSERT INTO schema_migrations (version) VALUES ('%s');\nCOMMIT;\n" "$$version"; \
			} | $$compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d warehouse_dev; \
		fi; \
	done; \
	echo "Applying development seed data (platform account parag@platform.test)"; \
	$$compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d warehouse_dev \
		< $(API_DIR)/db/seeds/dev.sql

db: docker-start
	docker compose -f $(API_DIR)/docker-compose.yml up -d --wait postgres
	docker compose -f $(API_DIR)/docker-compose.yml exec postgres psql -U app -d warehouse_dev

backend:
	docker compose -f $(API_DIR)/docker-compose.yml up --build api

logs:
	docker compose -f $(API_DIR)/docker-compose.yml logs --follow --tail=100 api

frontend:
	cd $(FRONTEND_DIR) && pnpm --filter @warehouse/web dev

dev:
	@set -e; \
	(docker compose -f $(API_DIR)/docker-compose.yml up --build api) & api_pid=$$!; \
	(cd $(FRONTEND_DIR) && pnpm --filter @warehouse/web dev) & frontend_pid=$$!; \
	trap 'kill $$api_pid $$frontend_pid 2>/dev/null || true; wait 2>/dev/null || true' INT TERM EXIT; \
	wait $$api_pid
