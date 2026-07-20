// Поиск потолка. Гоняется ПЕРВЫМ, до всех остальных сценариев.
//
// На одном VPS живут три Go-сервиса, три Postgres, Kafka и весь стек
// мониторинга. С высокой вероятностью первым насытится CPU хоста, а не что-либо
// в дизайне саги. Пока потолок не найден, любые выводы вида «сага держит N rps»
// на самом деле рассказывают про размер VPS.
//
// Как читать результат: смотреть не на цифру rps из отчёта, а на два признака.
// Первый — предупреждение k6 про dropped iterations / insufficient VUs: значит
// не догнал генератор, и throughput в отчёте больше не про сервер. Второй —
// график CPU по контейнерам из cAdvisor: если общий CPU уткнулся в 100%,
// найден потолок железа.
//
// Дальше все содержательные сценарии гонять на 60–70% от найденного числа,
// чтобы мерить систему, а не тесноту хоста.
import { postCatalogOrder, checkOrderCreated, loadParams, probeBaseline } from "./lib/orders.js";

const STEP_DURATION = __ENV.STEP_DURATION || "1m";

export const options = {
  scenarios: {
    capacity: {
      executor: "ramping-arrival-rate",
      startRate: Number(__ENV.START_RATE || 25),
      timeUnit: "1s",
      preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 100),
      // Запас по VU намеренно большой: когда сервис начнёт деградировать и время
      // ответа вырастет, по закону Литтла на тот же rps понадобится кратно
      // больше VU. Упереться здесь в собственный генератор — худший исход.
      maxVUs: Number(__ENV.MAX_VUS || 1000),
      stages: [
        { duration: STEP_DURATION, target: Number(__ENV.STEP_1 || 50) },
        { duration: STEP_DURATION, target: Number(__ENV.STEP_2 || 100) },
        { duration: STEP_DURATION, target: Number(__ENV.STEP_3 || 200) },
        { duration: STEP_DURATION, target: Number(__ENV.STEP_4 || 400) },
        { duration: STEP_DURATION, target: Number(__ENV.STEP_5 || 800) },
        { duration: "30s", target: 0 },
      ],
      exec: "load",
    },
    baseline: {
      executor: "constant-arrival-rate",
      rate: Number(__ENV.BASELINE_RATE || 2),
      timeUnit: "1s",
      duration: __ENV.BASELINE_DURATION || "5m30s",
      preAllocatedVUs: 5,
      maxVUs: 20,
      exec: "baseline",
    },
  },
  // Порогов намеренно нет: цель прогона — найти, где всё сломается,
  // а не отчитаться, что не сломалось.
  tags: { test_type: "capacity" },
};

const params = loadParams();

export function load() {
  checkOrderCreated(postCatalogOrder(params, `capacity-${__VU}-${__ITER}`));
}

export function baseline() {
  probeBaseline(params);
}