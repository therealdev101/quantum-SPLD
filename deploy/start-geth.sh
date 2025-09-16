#!/usr/bin/env bash
set -euo pipefail

DATADIR=${DATADIR:-/data}
GENESIS_PATH=${GENESIS_PATH:-/genesis/genesis.json}
NETWORK_ID=${NETWORK_ID:-1337}
HTTP_ADDR=${HTTP_ADDR:-0.0.0.0}
HTTP_PORT=${HTTP_PORT:-8545}
HTTP_APIS=${HTTP_APIS:-eth,net,web3,x402}
WS_ENABLE=${WS_ENABLE:-false}
WS_ADDR=${WS_ADDR:-0.0.0.0}
WS_PORT=${WS_PORT:-8546}
EXTRA_OPTS=${EXTRA_OPTS:-}

# Strict signature verification for production (can be overridden)
export X402_STRICT_VERIFY=${X402_STRICT_VERIFY:-1}

mkdir -p "${DATADIR}"

# Initialize if needed
if [ ! -d "${DATADIR}/geth/chaindata" ]; then
  if [ ! -f "${GENESIS_PATH}" ]; then
    echo "ERROR: Genesis file not found at ${GENESIS_PATH}" >&2
    exit 1
  fi
  echo "[start-geth] Initializing datadir with genesis: ${GENESIS_PATH}"
  geth --datadir "${DATADIR}" init "${GENESIS_PATH}"
fi

# Build base command
CMD=(geth
  --datadir "${DATADIR}"
  --networkid "${NETWORK_ID}"
  --http
  --http.addr "${HTTP_ADDR}"
  --http.port "${HTTP_PORT}"
  --http.api "${HTTP_APIS}"
  --authrpc.addr 0.0.0.0
  --authrpc.port 8551
  --authrpc.vhosts "*"
  --ipcdisable
  --nousb
  --maxpeers 50
)

# WebSocket (optional)
if [ "${WS_ENABLE}" = "true" ]; then
  CMD+=(--ws --ws.addr "${WS_ADDR}" --ws.port "${WS_PORT}" --ws.api "${HTTP_APIS}")
fi

# Never enable personal or --allow-insecure-unlock in production
# No --dev, no mining here (RPC node)

# Append any extra opts from env
if [ -n "${EXTRA_OPTS}" ]; then
  # shellcheck disable=SC2206
  EXTRA_ARR=(${EXTRA_OPTS})
  CMD+=("${EXTRA_ARR[@]}")
fi

echo "[start-geth] Starting geth with strict verify=${X402_STRICT_VERIFY}, http.api=${HTTP_APIS}, networkid=${NETWORK_ID}"
exec "${CMD[@]}"
