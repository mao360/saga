.PHONY: up down mocks

up:
	# Запуск инфраструктуры
	docker compose --env-file .env.infra -f docker-compose.infra.yaml up -d
	# Сборка и запуск сервисов
	docker compose -f order/docker-compose.yaml up -d --build
	docker compose -f inventory/docker-compose.yaml up -d --build
	docker compose -f payment/docker-compose.yaml up -d --build

down:
	docker compose -f payment/docker-compose.yaml down
	docker compose -f inventory/docker-compose.yaml down
	docker compose -f order/docker-compose.yaml down
	docker compose -f docker-compose.infra.yaml down

mocks:
	@if command -v mockery >/dev/null 2>&1; then \
		cd order && mockery --config ../.mockery.yaml; \
	else \
		cd order && go run github.com/vektra/mockery/v2@latest --config ../.mockery.yaml; \
	fi