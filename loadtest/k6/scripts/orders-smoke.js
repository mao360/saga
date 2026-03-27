import http from "k6/http";
import { check, sleep } from "k6";

const baseUrl = __ENV.ORDER_BASE_URL || "http://localhost:8080";
const path = __ENV.ORDER_PATH || "/orders";
const sku = __ENV.SKU || "sku-1";
const accountId = __ENV.ACCOUNT_ID || "acc-1";
const amount = Number(__ENV.AMOUNT || 100);
const qty = Number(__ENV.QTY || 1);

export const options = {
  vus: 5,
  duration: "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<800"],
  },
};

export default function () {
  const payload = JSON.stringify({
    customer: `load-user-${__VU}-${__ITER}`,
    amount,
    sku,
    qty,
    account_id: accountId,
  });

  const res = http.post(`${baseUrl}${path}`, payload, {
    headers: { "Content-Type": "application/json" },
    timeout: "10s",
  });

  check(res, {
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

  sleep(0.2);
}
