.PHONY: up down

up:
	# Запуск инфраструктуры
	docker compose --env-file .env.infra -f docker-compose.infra.yaml up -d
	# Сборка и запуск сервисов
	docker compose -f order/docker-compose.yaml up -d --build
	docker compose -f inventory/docker-compose.yaml up -d --build

down:
	docker compose -f inventory/docker-compose.yaml down
	docker compose -f order/docker-compose.yaml down
	docker compose -f docker-compose.infra.yaml down