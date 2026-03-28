#!/usr/bin/env bash
set -euo pipefail

KCAT="${KCAT:-}"
if [ -z "${KCAT}" ]; then
  if command -v kcat >/dev/null 2>&1; then
    KCAT="$(command -v kcat)"
  elif [ -x "/opt/homebrew/bin/kcat" ]; then
    KCAT="/opt/homebrew/bin/kcat"
  elif [ -x "/usr/local/bin/kcat" ]; then
    KCAT="/usr/local/bin/kcat"
  else
    echo "kcat not found. Set KCAT=/full/path/to/kcat"
    exit 1
  fi
fi

BROKER="${BROKER:-localhost:29092}"
TOPIC="${TOPIC:-saga.commands}"
SKU="${SKU:-sku-1}"
ACCOUNT_ID="${ACCOUNT_ID:-acc-1}"
STOCK_QTY="${STOCK_QTY:-200000}"
BALANCE="${BALANCE:-20000000}"
TS="$(date +%s)"

echo "{\"command_id\":\"cmd-set-stock-${TS}\",\"type\":\"set_stock\",\"sku\":\"${SKU}\",\"qty\":${STOCK_QTY}}" \
| "${KCAT}" -P -b "${BROKER}" -t "${TOPIC}"

echo "{\"command_id\":\"cmd-set-balance-${TS}\",\"type\":\"set_balance\",\"account_id\":\"${ACCOUNT_ID}\",\"amount\":${BALANCE}}" \
| "${KCAT}" -P -b "${BROKER}" -t "${TOPIC}"

echo "prefill sent: sku=${SKU}, stock=${STOCK_QTY}, account_id=${ACCOUNT_ID}, balance=${BALANCE}"
