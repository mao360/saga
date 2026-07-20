#!/usr/bin/env bash
# Засев каталога: STOCK_QTY на каждый из SKU_COUNT товаров и BALANCE на каждый
# из ACCOUNT_COUNT счетов.
#
# ВАЖНО: запускается НА VPS, а не с ноутбука. Kafka по docs/deploy.md слушает
# только 127.0.0.1 и наружу не публикуется, поэтому kcat с локальной машины
# до брокера не достучится:
#
#   ssh user@VPS_IP 'cd saga && ./loadtest/k6/scripts/prefill.sh'
#
# Альтернатива для итераций — SSH-туннель на 29092 и BROKER=localhost:29092.
set -euo pipefail

# Продюсер по умолчанию — консольный producer внутри контейнера kafka.
# kcat с VPS не работает: localhost там резолвится в IPv6 ::1, а брокер по
# ADMIN_BIND слушает 127.0.0.1, плюс kcat на хосте может быть не установлен.
# Изнутри контейнера идём на localhost:9092 и обе проблемы отпадают.
# PRODUCER=kcat вернёт старое поведение (актуально для локального dev).
PRODUCER="${PRODUCER:-docker}"
TOPIC="${TOPIC:-saga.commands}"
KAFKA_CONTAINER="${KAFKA_CONTAINER:-kafka}"
BROKER="${BROKER:-localhost:29092}"

produce() {
  case "${PRODUCER}" in
    docker)
      docker exec -i "${KAFKA_CONTAINER}" \
        /opt/kafka/bin/kafka-console-producer.sh \
        --bootstrap-server localhost:9092 --topic "${TOPIC}"
      ;;
    kcat)
      local kcat="${KCAT:-}"
      if [ -z "${kcat}" ]; then
        for c in kcat /opt/homebrew/bin/kcat /usr/local/bin/kcat; do
          if command -v "$c" >/dev/null 2>&1 || [ -x "$c" ]; then kcat="$c"; break; fi
        done
      fi
      [ -n "${kcat}" ] || { echo "kcat not found. Set KCAT=/full/path/to/kcat" >&2; exit 1; }
      "${kcat}" -P -b "${BROKER}" -t "${TOPIC}"
      ;;
    *)
      echo "unknown PRODUCER=${PRODUCER} (ожидается docker или kcat)" >&2
      exit 1
      ;;
  esac
}

SKU_COUNT="${SKU_COUNT:-200}"
ACCOUNT_COUNT="${ACCOUNT_COUNT:-200}"
STOCK_QTY="${STOCK_QTY:-200000}"
BALANCE="${BALANCE:-20000000}"
TS="$(date +%s)"

# command_id включает TS, иначе повторный засев будет отброшен дедупликацией
# по processed_commands в inventory/payment.
{
  for i in $(seq 1 "${SKU_COUNT}"); do
    printf '{"command_id":"cmd-set-stock-%s-%s","type":"set_stock","sku":"sku-%s","qty":%s}\n' \
      "${TS}" "${i}" "${i}" "${STOCK_QTY}"
  done
  for i in $(seq 1 "${ACCOUNT_COUNT}"); do
    printf '{"command_id":"cmd-set-balance-%s-%s","type":"set_balance","account_id":"acc-%s","amount":%s}\n' \
      "${TS}" "${i}" "${i}" "${BALANCE}"
  done
} | produce

echo "prefill sent: ${SKU_COUNT} sku x ${STOCK_QTY}, ${ACCOUNT_COUNT} accounts x ${BALANCE}"
echo "note: команды применяются асинхронно, дай сервисам пару секунд до старта нагрузки"