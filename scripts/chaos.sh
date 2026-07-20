#!/usr/bin/env bash
# Инъекция отказов под нагрузкой. Запускается НА VPS параллельно с k6:
#
#   ssh user@VPS_IP 'cd saga && ./scripts/chaos.sh kill-payment 30'
#
# Смысл не в том, чтобы система не сломалась, а в том, чтобы она восстановилась
# сама. Метрика результата — не «упало / не упало», а recovery time: за сколько
# order_saga_pending_orders вернулся к норме и разгрёбся лаг консьюмеров.
# Смотреть на дашборде Saga Overview, ряд Saga duration / Pending sagas.
#
# После каждого прогона обязателен ./scripts/verify-consistency.sh: отказ в
# середине саги — ровно тот случай, когда компенсация может не доехать.
set -euo pipefail

usage() {
  cat <<'USAGE'
Использование: ./scripts/chaos.sh <сценарий> [длительность_сек]

Сценарии:
  pause-payment [сек]    Заморозить payment (SIGSTOP). Контейнер жив, но не отвечает —
                         имитация зависшего процесса, худший случай для таймаутов.
  kill-payment [сек]     Убить payment и поднять обратно. Проверяет, доедут ли
                         команды из Kafka после возвращения консьюмера.
  kill-inventory [сек]   То же для inventory.
  restart-kafka          Перезапустить брокер. Продюсеры и консьюмеры обязаны
                         переподключиться, outbox — дослать накопленное.
  pause-order-db [сек]   Заморозить Postgres заказа: outbox-релей упирается в БД,
                         HTTP-приём заказов встаёт.
  status                 Показать текущее состояние контейнеров.
USAGE
}

DURATION="${2:-30}"

wait_and_report() { # wait_and_report <контейнер> <действие>
  echo "[chaos] $2 -> ${1}, держим ${DURATION}с"
  sleep "$DURATION"
}

case "${1:-}" in
  pause-payment)
    docker pause payment
    wait_and_report payment "заморозка"
    docker unpause payment
    echo "[chaos] payment разморожен"
    ;;
  kill-payment)
    docker stop payment
    wait_and_report payment "остановка"
    docker start payment
    echo "[chaos] payment поднят"
    ;;
  kill-inventory)
    docker stop inventory
    wait_and_report inventory "остановка"
    docker start inventory
    echo "[chaos] inventory поднят"
    ;;
  restart-kafka)
    echo "[chaos] перезапуск брокера"
    docker restart kafka
    echo "[chaos] kafka перезапущена; следи за лагом консьюмеров и outbox"
    ;;
  pause-order-db)
    docker pause postgres-order
    wait_and_report postgres-order "заморозка"
    docker unpause postgres-order
    echo "[chaos] postgres-order разморожен"
    ;;
  status)
    docker ps --filter name=order --filter name=inventory --filter name=payment \
      --filter name=kafka --filter name=postgres \
      --format 'table {{.Names}}\t{{.Status}}'
    exit 0
    ;;
  *)
    usage
    exit 1
    ;;
esac

echo
echo "[chaos] дальше: дождись возврата pending к норме на дашборде, затем"
echo "[chaos] ./scripts/verify-consistency.sh"