import { sleep } from "k6";
import { checkOrderCreated, loadParams, postOrder } from "./lib/orders.js";

export const options = {
  stages: [
    { duration: __ENV.STAGE_1_DURATION || "30s", target: Number(__ENV.STAGE_1_TARGET || 10) },
    { duration: __ENV.STAGE_2_DURATION || "1m", target: Number(__ENV.STAGE_2_TARGET || 30) },
    { duration: __ENV.STAGE_3_DURATION || "1m", target: Number(__ENV.STAGE_3_TARGET || 50) },
    { duration: __ENV.STAGE_4_DURATION || "30s", target: Number(__ENV.STAGE_4_TARGET || 0) },
  ],
  thresholds: {
    http_req_failed: ["rate<0.03"],
    http_req_duration: ["p(95)<1200", "p(99)<2500"],
  },
  tags: { test_type: "ramp" },
};

export default function () {
  const params = loadParams();
  const res = postOrder(params, `ramp-user-${__VU}-${__ITER}`);
  checkOrderCreated(res);

  sleep(Number(__ENV.SLEEP_SECONDS || 0.1));
}
