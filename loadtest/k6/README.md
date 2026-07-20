# Нагрузочное тестирование k6

## 1. Предусловия

- Поднять стек приложения: `make up`
- Скопировать шаблон env мониторинга: `cp .env.monitoring.example .env.monitoring`
- (Опционально) Скопировать шаблон env для нагрузки: `cp loadtest/k6/.env.loadtest.example loadtest/k6/.env.loadtest`
- Убедиться, что для happy-path засеяны склад и баланс:
  - `sku=sku-1`
  - `account_id=acc-1`
- Поднять стек мониторинга:
  - `docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml up -d`
  - Дашборды Grafana описаны как код в `monitoring/grafana/` и подгружаются автоматически.
  - Данные Grafana (история и т.п.) лежат в Docker-томе `${GRAFANA_DATA_VOLUME:-grafana_data}`.

## 2. Smoke-тест

Короткий базовый прогон:

```bash
k6 run loadtest/k6/scripts/orders-smoke.js
```

## 3. Ramp-тест

Небольшая рампа нагрузки:

```bash
k6 run loadtest/k6/scripts/orders-ramp.js
```

## 4. Soak-тест

Длительная стабильная нагрузка:

```bash
k6 run loadtest/k6/scripts/orders-soak.js
```

## 5. Arrival-rate тест

Профиль с целевым RPS:

```bash
k6 run loadtest/k6/scripts/orders-arrival.js
```

## 6. Шорткаты в Makefile

```bash
make load-smoke-with-prefill
make load-ramp-with-prefill
make load-soak-with-prefill
make load-arrival-with-prefill
```

Если пути к `k6` или `kcat` отличаются:

```bash
K6=/path/to/k6 KCAT=/path/to/kcat make load-ramp-with-prefill
```

## 7. Переопределение через переменные окружения

Можно переопределить значения по умолчанию:

```bash
ORDER_BASE_URL=http://localhost:8080 \
ORDER_PATH=/orders \
SKU=sku-1 \
ACCOUNT_ID=acc-1 \
AMOUNT=100 \
QTY=1 \
k6 run loadtest/k6/scripts/orders-smoke.js
```

Примеры типовой настройки:

```bash
VUS=20 DURATION=2m k6 run loadtest/k6/scripts/orders-soak.js
STAGE_1_TARGET=20 STAGE_2_TARGET=60 STAGE_3_TARGET=90 k6 run loadtest/k6/scripts/orders-ramp.js
START_RATE=30 STAGE_1_RATE=70 STAGE_2_RATE=120 STAGE_3_RATE=120 k6 run loadtest/k6/scripts/orders-arrival.js
```

## 8. Где смотреть метрики

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (admin/admin)
- Эндпоинт метрик cAdvisor: `http://localhost:8088/metrics`
- Эндпоинт метрик Kafka exporter: `http://localhost:9308/metrics`

## 9. Примечания

- Это нагрузка только по happy-path.
- Если склада/баланса перестаёт хватать, будут ответы не-201.
- Пополняйте `inventory` и `payment` перед длительными прогонами.
```