# k6 load testing

## 1. Preconditions

- Start app stack: `make up`
- Copy monitoring env template: `cp .env.monitoring.example .env.monitoring`
- (Optional) Copy loadtest env template: `cp loadtest/k6/.env.loadtest.example loadtest/k6/.env.loadtest`
- Ensure stock and balance are prefilled for happy path:
  - `sku=sku-1`
  - `account_id=acc-1`
- Start monitoring stack:
  - `docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml up -d`
  - Grafana dashboards are persisted in Docker volume `${GRAFANA_DATA_VOLUME:-grafana_data}`.
  - Do not use `docker compose down -v` for monitoring unless you want to reset dashboards.

## 2. Smoke test

Run a short baseline test:

```bash
k6 run loadtest/k6/scripts/orders-smoke.js
```

## 3. Ramp test

Run a small ramp:

```bash
k6 run loadtest/k6/scripts/orders-ramp.js
```

## 4. Soak test

Run longer stable load:

```bash
k6 run loadtest/k6/scripts/orders-soak.js
```

## 5. Arrival-rate test

Run target RPS profile:

```bash
k6 run loadtest/k6/scripts/orders-arrival.js
```

## 6. Makefile shortcuts

```bash
make load-smoke-with-prefill
make load-ramp-with-prefill
make load-soak-with-prefill
make load-arrival-with-prefill
```

If `k6` or `kcat` path differs:

```bash
K6=/path/to/k6 KCAT=/path/to/kcat make load-ramp-with-prefill
```

## 7. Environment overrides

You can override defaults:

```bash
ORDER_BASE_URL=http://localhost:8080 \
ORDER_PATH=/orders \
SKU=sku-1 \
ACCOUNT_ID=acc-1 \
AMOUNT=100 \
QTY=1 \
k6 run loadtest/k6/scripts/orders-smoke.js
```

Common tuning examples:

```bash
VUS=20 DURATION=2m k6 run loadtest/k6/scripts/orders-soak.js
STAGE_1_TARGET=20 STAGE_2_TARGET=60 STAGE_3_TARGET=90 k6 run loadtest/k6/scripts/orders-ramp.js
START_RATE=30 STAGE_1_RATE=70 STAGE_2_RATE=120 STAGE_3_RATE=120 k6 run loadtest/k6/scripts/orders-arrival.js
```

## 8. Where to смотреть metrics

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (admin/admin)
- cAdvisor metrics endpoint: `http://localhost:8088/metrics`
- Kafka exporter metrics endpoint: `http://localhost:9308/metrics`

## 9. Notes

- This is happy-path load only.
- If stock/balance becomes insufficient, you will see non-201 responses.
- Refill `inventory` and `payment` before long tests.
