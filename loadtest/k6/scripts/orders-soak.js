import { sleep } from "k6";
import { checkOrderCreated, loadParams, postOrder } from "./lib/orders.js";

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || "10m",
  thresholds: {
    http_req_failed: ["rate<0.03"],
    http_req_duration: ["p(95)<1200", "p(99)<2500"],
  },
  tags: { test_type: "soak" },
};

export default function () {
  const params = loadParams();
  const res = postOrder(params, `soak-user-${__VU}-${__ITER}`);
  checkOrderCreated(res);
  sleep(Number(__ENV.SLEEP_SECONDS || 0.2));
}
