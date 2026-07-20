#!/usr/bin/env bash
# Проверка инвариантов саги после нагрузочного прогона.
#
# Запускается НА VPS: базы по docs/deploy.md слушают только 127.0.0.1.
#   ssh user@VPS_IP 'cd saga && ./scripts/verify-consistency.sh'
#
# Нагрузочный тест сам по себе доказывает только то, что система быстрая.
# Что она при этом корректна — доказывает этот скрипт: k6 видит лишь HTTP 201,
# а разъехались ли три базы между собой, из ответов не видно.
#
# Выход: 0 — все инварианты держатся, 1 — есть нарушения.
set -uo pipefail

PG_ORDER="${PG_ORDER:-postgres-order}"
PG_INVENTORY="${PG_INVENTORY:-postgres-inventory}"
PG_PAYMENT="${PG_PAYMENT:-postgres-payment}"
STUCK_AFTER="${STUCK_AFTER:-60}" # секунд, после которых pending считается зависшим

failed=0

q() { # q <container> <db-user/db> <sql>
  docker exec "$1" psql -U "$2" -d "$2" -tAc "$3" 2>/dev/null | tr -d '[:space:]'
}

check() { # check <описание> <фактическое> <ожидаемое>
  if [ "$2" = "$3" ]; then
    printf 'OK    %-52s %s\n' "$1" "$2"
  else
    printf 'FAIL  %-52s got=%s want=%s\n' "$1" "$2" "$3"
    failed=1
  fi
}

echo "== Саги в полёте =="
pending_total=$(q "$PG_ORDER" order "select count(*) from orders where status='pending'")
echo "      заказов в pending: ${pending_total}"

# Зависшая сага: pending дольше разумного. Именно она не видна в rate-метриках,
# потому что не порождает событий.
stuck=$(q "$PG_ORDER" order \
  "select count(*) from orders where status='pending' and created_at < now() - interval '${STUCK_AFTER} seconds'")
check "нет саг, зависших дольше ${STUCK_AFTER}с" "$stuck" "0"

echo
echo "== Согласованность order.status и saga_state =="

# completed обязан означать обе успешные ветки.
bad_completed=$(q "$PG_ORDER" order \
  "select count(*) from orders o join saga_state s on s.order_id=o.id
   where o.status='completed' and not (s.inventory_status='reserved' and s.payment_status='charged')")
check "completed только при reserved+charged" "$bad_completed" "0"

# Деньги списаны, а заказ провалился и возврата не было — потерянные средства.
money_stuck=$(q "$PG_ORDER" order \
  "select count(*) from orders o join saga_state s on s.order_id=o.id
   where o.status='failed' and s.payment_status='charged'")
check "нет failed с незавершённым возвратом" "$money_stuck" "0"

# Товар зарезервирован, а заказ провалился и резерв не сняли — залипший сток.
stock_stuck=$(q "$PG_ORDER" order \
  "select count(*) from orders o join saga_state s on s.order_id=o.id
   where o.status='failed' and s.inventory_status='reserved'")
check "нет failed с неснятым резервом" "$stock_stuck" "0"

# Заказ без саги вообще: значит, запись заказа и старт саги разъехались.
orphans=$(q "$PG_ORDER" order \
  "select count(*) from orders o left join saga_state s on s.order_id=o.id where s.order_id is null")
check "нет заказов без saga_state" "$orphans" "0"

echo
echo "== Кросс-базовый инвариант: склад =="

# Сколько товара реально удерживает inventory.
held=$(q "$PG_INVENTORY" inventory \
  "select coalesce(sum(total_qty-available_qty),0) from inventory_stock")
# Сколько его должно удерживаться по мнению order: все саги в статусе reserved.
expected_held=$(q "$PG_ORDER" order \
  "select coalesce(sum(o.qty),0) from orders o join saga_state s on s.order_id=o.id
   where s.inventory_status='reserved'")
check "удержанный сток сходится с order" "$held" "$expected_held"

# CHECK-констрейнт не даст уйти в минус, но проверяем явно: если он сработал,
# сервис получит ошибку, и интереснее увидеть это здесь, чем в логах.
negative=$(q "$PG_INVENTORY" inventory \
  "select count(*) from inventory_stock where available_qty < 0 or available_qty > total_qty")
check "нет отрицательного/переполненного стока" "$negative" "0"

echo
echo "== Outbox =="
unsent=$(q "$PG_ORDER" order "select count(*) from outbox_messages where sent_at is null")
echo "      неотправленных сообщений: ${unsent}"
# После завершения прогона релей обязан разгрести очередь. Ненулевое значение
# спустя время означает, что релей не справляется или мёртв.
check "outbox разгребён" "$unsent" "0"

stuck_retries=$(q "$PG_ORDER" order "select count(*) from outbox_messages where attempts > 5 and sent_at is null")
check "нет сообщений с attempts>5" "$stuck_retries" "0"

echo
if [ "$failed" -eq 0 ]; then
  echo "ИТОГ: все инварианты держатся"
else
  echo "ИТОГ: обнаружены нарушения (см. FAIL выше)"
fi
exit "$failed"
