import http from "k6/http";
import { check } from "k6";

export function loadParams() {
  return {
    baseUrl: __ENV.ORDER_BASE_URL || "http://localhost:8080",
    path: __ENV.ORDER_PATH || "/orders",
    sku: __ENV.SKU || "sku-1",
    accountId: __ENV.ACCOUNT_ID || "acc-1",
    amount: Number(__ENV.AMOUNT || 100),
    qty: Number(__ENV.QTY || 1),
  };
}

export function postOrder(params, customer) {
  const payload = JSON.stringify({
    customer,
    amount: params.amount,
    sku: params.sku,
    qty: params.qty,
    account_id: params.accountId,
  });

  return http.post(`${params.baseUrl}${params.path}`, payload, {
    headers: { "Content-Type": "application/json" },
    timeout: __ENV.REQUEST_TIMEOUT || "15s",
  });
}

export function checkOrderCreated(res) {
  return check(res, {
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
}
