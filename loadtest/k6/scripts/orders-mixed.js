// Смешанный бизнес-профиль: так выглядит реальный трафик магазина.
//
// Happy path — не единственный путь в проде. Часть заказов упирается в
// отсутствие товара, часть — в нехватку денег, часть приходит дважды из-за
// ретрая клиента после таймаута. Каждый из этих случаев запускает в саге свою
// ветку: компенсацию (release_inventory / refund_payment) или дедупликацию по
// processed_commands. Компенсация — это лишний раунд по Kafka, то есть профиль
// нагрузки на брокер у неё принципиально другой, чем у happy path.
//
// Все четыре потока идут одновременно, потому что в проде они и идут
// одновременно, и интересно именно их взаимное влияние.
import { postCatalogOrder, postOrder, checkOrderCreated, loadParams, probeBaseline } from "./lib/orders.js";
import { pickSku, pickAccount } from "./lib/catalog.js";

const RATE = Number(__ENV.RATE || 60);
const DURATION = __ENV.DURATION || "5m";

function scenario(exec, share, extra = {}) {
  return {
    executor: "constant-arrival-rate",
    rate: Math.max(1, Math.round(RATE * share)),
    timeUnit: "1s",
    duration: DURATION,
    preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || 50),
    maxVUs: Number(__ENV.MAX_VUS || 300),
    exec,
    ...extra,
  };
}

export const options = {
  scenarios: {
    happy: scenario("happyPath", Number(__ENV.SHARE_HAPPY || 0.8)),
    out_of_stock: scenario("outOfStock", Number(__ENV.SHARE_OUT_OF_STOCK || 0.05)),
    no_funds: scenario("insufficientFunds", Number(__ENV.SHARE_NO_FUNDS || 0.1)),
    duplicates: scenario("duplicateSubmit", Number(__ENV.SHARE_DUPLICATE || 0.05)),
    // Контрольная линия идёт всё время прогона с низкой постоянной скоростью.
    baseline: {
      executor: "constant-arrival-rate",
      rate: Number(__ENV.BASELINE_RATE || 2),
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: 5,
      maxVUs: 20,
      exec: "baseline",
    },
  },
  thresholds: {
    // Отказ саги по бизнес-причине — это всё равно HTTP 201: заказ принят,
    // а отклонение случится позже и асинхронно. Поэтому порог по http_req_failed
    // остаётся строгим даже там, где половина саг заведомо упадёт.
    "http_req_failed{endpoint:orders}": ["rate<0.03"],
    "http_req_duration{endpoint:orders}": ["p(95)<1500", "p(99)<3000"],
    baseline_healthz_duration: ["p(95)<1000"],
    order_accepted: ["rate>0.97"],
  },
  tags: { test_type: "mixed" },
};

const params = loadParams();

export function happyPath() {
  checkOrderCreated(postCatalogOrder(params, `mixed-happy-${__VU}-${__ITER}`));
}

// Заведомо больше, чем лежит на складе: сага дойдёт до inventory_rejected
// и, если оплата успела пройти, потянет за собой refund_payment.
export function outOfStock() {
  checkOrderCreated(
    postCatalogOrder(params, `mixed-oos-${__VU}-${__ITER}`, {
      qty: Number(__ENV.OUT_OF_STOCK_QTY || 1000000),
      tags: { flow: "out_of_stock" },
    }),
  );
}

// Сумма заведомо больше любого засеянного баланса: payment_rejected,
// а зарезервированный товар должен вернуться через release_inventory.
export function insufficientFunds() {
  checkOrderCreated(
    postCatalogOrder(params, `mixed-nofunds-${__VU}-${__ITER}`, {
      amount: Number(__ENV.NO_FUNDS_AMOUNT || 1000000000),
      tags: { flow: "insufficient_funds" },
    }),
  );
}

// Двойная отправка: клиент словил таймаут и повторил запрос, либо пользователь
// дважды нажал кнопку. Важно понимать, что сейчас это НЕ тест идемпотентности —
// у POST /orders нет ключа идемпотентности, поэтому оба запроса создадут два
// независимых заказа и дважды спишут деньги. processed_commands в inventory и
// payment ключуется по command_id и защищает только от повторной доставки
// команды из Kafka, до HTTP-слоя он не достаёт.
//
// Сценарий оставлен намеренно: он показывает, во что обходится дубль по складу
// и балансу, и служит нагрузочной базой на случай, если ключ идемпотентности
// будет добавлен. Тело обоих запросов одинаковое — как у настоящего ретрая.
export function duplicateSubmit() {
  const customer = `mixed-dup-${__VU}-${__ITER}`;
  const body = {
    sku: pickSku(params.catalog, params.cdf),
    accountId: pickAccount(params.catalog),
    tags: { flow: "duplicate" },
  };
  checkOrderCreated(postOrder(params, customer, body));
  checkOrderCreated(postOrder(params, customer, body));
}

export function baseline() {
  probeBaseline(params);
}