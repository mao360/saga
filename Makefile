K6 ?= k6
KCAT ?= /opt/homebrew/bin/kcat

.PHONY: up down monitoring-up monitoring-down load-prefill load-smoke load-ramp load-soak load-arrival load-capacity load-mixed load-hotkey vps-prefill vps-verify vps-chaos load-smoke-with-prefill load-ramp-with-prefill load-soak-with-prefill load-arrival-with-prefill mocks

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

# --- Прогоны против VPS ---------------------------------------------------
# Скрипты гоняются локально, приложение и вся инфраструктура — на VPS.
# Настройки берутся из loadtest/k6/.env.loadtest (ORDER_BASE_URL и прочее).
#
# Порядок первого захода:
#   1. make vps-prefill    — засеять каталог (по SSH, kafka наружу не смотрит)
#   2. make load-capacity  — найти потолок, дальше гонять на 60-70% от него
#   3. make load-mixed     — смешанный бизнес-профиль
#   4. make vps-verify     — проверить инварианты по трём базам

K6_ENV ?= loadtest/k6/.env.loadtest
VPS_SSH ?= $(error задай VPS_SSH, например VPS_SSH=vpsadmin@195.133.5.172)
VPS_SSH_PORT ?= 2222
VPS_DIR ?= saga
SSH = ssh -p $(VPS_SSH_PORT) $(VPS_SSH)

# У k6 нет --env-file, поэтому переменные экспортируются в окружение: k6 их
# подхватывает через --include-system-env-vars (включён по умолчанию).
# ulimit: дефолт macOS — 256 дескрипторов, при сотнях VU с keep-alive упрёшься
# в него молча, в виде необъяснимых ошибок соединения.
define k6_run
	@test -f $(K6_ENV) || { echo "нет $(K6_ENV) — cp loadtest/k6/.env.loadtest.example $(K6_ENV) и укажи ORDER_BASE_URL"; exit 1; }
	@set -a; . ./$(K6_ENV); set +a; ulimit -n 65536; $(K6) run $(1)
endef

load-capacity:
	$(call k6_run,loadtest/k6/scripts/orders-capacity.js)

load-mixed:
	$(call k6_run,loadtest/k6/scripts/orders-mixed.js)

load-hotkey:
	$(call k6_run,loadtest/k6/scripts/orders-hotkey.js)

vps-prefill:
	$(SSH) 'cd $(VPS_DIR) && ./loadtest/k6/scripts/prefill.sh'

vps-verify:
	$(SSH) 'cd $(VPS_DIR) && ./scripts/verify-consistency.sh'

# Пример: make vps-chaos CHAOS="kill-payment 30"
vps-chaos:
	$(SSH) 'cd $(VPS_DIR) && ./scripts/chaos.sh $(CHAOS)'

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