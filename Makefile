K6 ?= k6
KCAT ?= /opt/homebrew/bin/kcat

.PHONY: up down monitoring-up monitoring-down load-prefill load-smoke load-ramp load-soak load-arrival load-smoke-with-prefill load-ramp-with-prefill load-soak-with-prefill load-arrival-with-prefill mocks

up:
	# Запуск инфраструктуры
	docker compose --env-file .env.infra -f docker-compose.infra.yaml up -d
	# Запуск мониторинга
	docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml up -d
	# Сборка и запуск сервисов
	docker compose -f order/docker-compose.yaml up -d --build
	docker compose -f inventory/docker-compose.yaml up -d --build
	docker compose -f payment/docker-compose.yaml up -d --build

down:
	docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml down
	docker compose -f payment/docker-compose.yaml down
	docker compose -f inventory/docker-compose.yaml down
	docker compose -f order/docker-compose.yaml down
	docker compose --env-file .env.infra -f docker-compose.infra.yaml down

monitoring-up:
	docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml up -d

monitoring-down:
	docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml down

load-prefill:
	KCAT=$(KCAT) ./loadtest/k6/scripts/prefill-happy-path.sh

load-smoke:
	$(K6) run loadtest/k6/scripts/orders-smoke.js

load-ramp:
	$(K6) run loadtest/k6/scripts/orders-ramp.js

load-soak:
	$(K6) run loadtest/k6/scripts/orders-soak.js

load-arrival:
	$(K6) run loadtest/k6/scripts/orders-arrival.js

load-smoke-with-prefill: load-prefill load-smoke

load-ramp-with-prefill: load-prefill load-ramp

load-soak-with-prefill: load-prefill load-soak

load-arrival-with-prefill: load-prefill load-arrival

mocks:
	@if command -v mockery >/dev/null 2>&1; then \
		cd order && mockery --config ../.mockery.yaml; \
	else \
		cd order && go run github.com/vektra/mockery/v2@latest --config ../.mockery.yaml; \
	fi