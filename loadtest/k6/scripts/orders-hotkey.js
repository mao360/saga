// Флеш-сейл: заданная доля трафика бьёт в один SKU.
//
// Смысл — сравнить с реалистичным Zipf-профилем на той же скорости. Разница в
// throughput и в p95 саги показывает цену row-level lock на строке склада:
// заказы на один товар сериализуются в inventory, сколько бы VU ни было.
// Это же и тест на гонку за последним товаром — если STOCK_QTY меньше, чем
// придёт заказов, часть саг обязана честно упасть в failed через компенсацию,
// а склад не имеет права уйти в минус (проверяется verify-скриптом на VPS).
import { postCatalogOrder, checkOrderCreated, loadParams, probeBaseline } from "./lib/orders.js";

export const options = {
  scenarios: {
    flash_sale: {
      executor: "constant-arrival-rate",
      rate: Number(__ENV.RATE || 100),
      timeUnit: "1s",
      duration: __ENV.DURATION || "3m",
      preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 50),
      maxVUs: Number(__ENV.MAX_VUS || 300),
      exec: "flashSale",
    },
    baseline: {
      executor: "constant-arrival-rate",
      rate: Number(__ENV.BASELINE_RATE || 2),
      timeUnit: "1s",
      duration: __ENV.DURATION || "3m",
      preAllocatedVUs: 5,
      maxVUs: 20,
      exec: "baseline",
    },
  },
  thresholds: {
    "http_req_failed{endpoint:orders}": ["rate<0.05"],
    "http_req_duration{endpoint:orders}": ["p(95)<2000"],
    baseline_healthz_duration: ["p(95)<1000"],
  },
  tags: { test_type: "hotkey" },
};

// HOT_SKU и HOT_SHARE читает сам каталог; по умолчанию для этого сценария
// весь трафик идёт в один товар.
const params = loadParams();

export function flashSale() {
  checkOrderCreated(postCatalogOrder(params, `hotkey-${__VU}-${__ITER}`));
}

export function baseline() {
  probeBaseline(params);
}