import http from "k6/http";
import { check, sleep } from "k6";

const baseUrl = __ENV.ORDER_BASE_URL || "http://localhost:8080";
const path = __ENV.ORDER_PATH || "/orders";
const sku = __ENV.SKU || "sku-1";
const accountId = __ENV.ACCOUNT_ID || "acc-1";
const amount = Number(__ENV.AMOUNT || 100);
const qty = Number(__ENV.QTY || 1);

export const options = {
  stages: [
    { duration: "30s", target: 10 },
    { duration: "1m", target: 30 },
    { duration: "1m", target: 50 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.03"],
    http_req_duration: ["p(95)<1200", "p(99)<2500"],
  },
};

export default function () {
  const payload = JSON.stringify({
    customer: `ramp-user-${__VU}-${__ITER}`,
    amount,
    sku,
    qty,
    account_id: accountId,
  });

  const res = http.post(`${baseUrl}${path}`, payload, {
    headers: { "Content-Type": "application/json" },
    timeout: "15s",
  });

  check(res, {
    "status 201": (r) => r.status === 201,
  });

  sleep(0.1);
}
