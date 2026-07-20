import http from "k6/http";
import { check } from "k6";
import { Trend, Rate } from "k6/metrics";
import { loadCatalog, buildZipfCdf, pickSku, pickAccount } from "./catalog.js";

// Контрольная линия. /healthz не трогает ни БД, ни Kafka, поэтому его латентность —
// это чистая цена сети до VPS плюс accept в Go. Разница между ним и /orders под
// той же нагрузкой и есть настоящая работа приложения. Без этой линии на прогоне
// с ноутбука не отличить деградацию сервиса от дрожания домашнего канала.
export const baselineLatency = new Trend("baseline_healthz_duration", true);
export const orderAccepted = new Rate("order_accepted");

export function loadParams() {
  const catalog = loadCatalog();
  return {
    baseUrl: __ENV.ORDER_BASE_URL || "http://localhost:8080",
    path: __ENV.ORDER_PATH || "/orders",
    healthPath: __ENV.HEALTH_PATH || "/healthz",
    // Фиксированные sku/account оставлены для обратной совместимости со старыми
    // сценариями (smoke/ramp/soak/arrival) — они гоняют один ключ намеренно.
    sku: __ENV.SKU || "sku-1",
    accountId: __ENV.ACCOUNT_ID || "acc-1",
    amount: Number(__ENV.AMOUNT || 100),
    qty: Number(__ENV.QTY || 1),
    catalog,
    cdf: buildZipfCdf(catalog.skuCount, catalog.zipfS),
  };
}

// Заказ по каталогу: SKU по Zipf, счёт равномерно.
export function postCatalogOrder(params, customer, overrides = {}) {
  return postOrder(params, customer, {
    sku: pickSku(params.catalog, params.cdf),
    accountId: pickAccount(params.catalog),
    ...overrides,
  });
}

export function postOrder(params, customer, overrides = {}) {
  const payload = JSON.stringify({
    customer,
    amount: overrides.amount !== undefined ? overrides.amount : params.amount,
    sku: overrides.sku || params.sku,
    qty: overrides.qty !== undefined ? overrides.qty : params.qty,
    account_id: overrides.accountId || params.accountId,
  });

  return http.post(`${params.baseUrl}${params.path}`, payload, {
    headers: { "Content-Type": "application/json" },
    timeout: __ENV.REQUEST_TIMEOUT || "15s",
    tags: { endpoint: "orders", ...(overrides.tags || {}) },
  });
}

export function probeBaseline(params) {
  const res = http.get(`${params.baseUrl}${params.healthPath}`, {
    timeout: __ENV.REQUEST_TIMEOUT || "15s",
    tags: { endpoint: "healthz" },
  });
  baselineLatency.add(res.timings.duration);
  return res;
}

// 201 означает только «заказ принят и лёг в outbox». Чем кончилась сага —
// completed или failed — здесь не видно и видно быть не может: она ещё не
// начиналась. Исход снимается на сервере метрикой order_saga_duration_seconds.
export function checkOrderCreated(res) {
  const ok = check(res, {
    "status is 201": (r) => r.status === 201,
    "response has order id": (r) => {
      if (!r.body) return false;
      try {
        const obj = JSON.parse(r.body);
        return typeof obj.id === "string" && obj.id.length > 0;
      } catch (_) {
        return false;
      }
    },
  });
  orderAccepted.add(ok);
  return ok;
}