#!/usr/bin/env bash
set -euo pipefail

BROKER="${BROKER:-localhost:29092}"
TOPIC="${TOPIC:-saga.commands}"
SKU="${SKU:-sku-1}"
ACCOUNT_ID="${ACCOUNT_ID:-acc-1}"
STOCK_QTY="${STOCK_QTY:-200000}"
BALANCE="${BALANCE:-20000000}"
TS="$(date +%s)"

echo "{\"command_id\":\"cmd-set-stock-${TS}\",\"type\":\"set_stock\",\"sku\":\"${SKU}\",\"qty\":${STOCK_QTY}}" \
| kcat -P -b "${BROKER}" -t "${TOPIC}"

echo "{\"command_id\":\"cmd-set-balance-${TS}\",\"type\":\"set_balance\",\"account_id\":\"${ACCOUNT_ID}\",\"amount\":${BALANCE}}" \
| kcat -P -b "${BROKER}" -t "${TOPIC}"

echo "prefill sent: sku=${SKU}, stock=${STOCK_QTY}, account_id=${ACCOUNT_ID}, balance=${BALANCE}"
