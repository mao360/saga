import { checkOrderCreated, loadParams, postOrder } from "./lib/orders.js";

export const options = {
  scenarios: {
    arrival_rate: {
      executor: "ramping-arrival-rate",
      startRate: Number(__ENV.START_RATE || 20),
      timeUnit: "1s",
      preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 50),
      maxVUs: Number(__ENV.MAX_VUS || 200),
      stages: [
        { duration: __ENV.STAGE_1_DURATION || "1m", target: Number(__ENV.STAGE_1_RATE || 50) },
        { duration: __ENV.STAGE_2_DURATION || "1m", target: Number(__ENV.STAGE_2_RATE || 100) },
        { duration: __ENV.STAGE_3_DURATION || "1m", target: Number(__ENV.STAGE_3_RATE || 100) },
        { duration: __ENV.STAGE_4_DURATION || "30s", target: Number(__ENV.STAGE_4_RATE || 0) },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<1500", "p(99)<3000"],
  },
  tags: { test_type: "arrival" },
};

export default function () {
  const params = loadParams();
  const res = postOrder(params, `arrival-user-${__VU}-${__ITER}`);
  checkOrderCreated(res);
}
