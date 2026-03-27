# k6 load testing

## 1. Preconditions

- Start app stack: `make up`
- Copy monitoring env template: `cp .env.monitoring.example .env.monitoring`
- Ensure stock and balance are prefilled for happy path:
  - `sku=sku-1`
  - `account_id=acc-1`
- Start monitoring stack:
  - `docker compose --env-file .env.monitoring -f docker-compose.monitoring.yaml up -d`

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

## 4. Environment overrides

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

## 5. Where to смотреть metrics

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (admin/admin)
- cAdvisor metrics endpoint: `http://localhost:8088/metrics`
- Kafka exporter metrics endpoint: `http://localhost:9308/metrics`

## 6. Notes

- This is happy-path load only.
- If stock/balance becomes insufficient, you will see non-201 responses.
- Refill `inventory` and `payment` before long tests.
