#!/usr/bin/env bash
# End-to-end x402 test on a dev node: verify -> settle -> receipt (consensus-safe, zero-fee)
set -euo pipefail

PORT=${PORT:-8571}
BIN="./Core-Blockchain/node_src/build/bin/geth"
LOG="/tmp/geth_x402_consensus.log"
PIDF="/tmp/geth_x402_consensus.pid"

echo "[x402] Starting dev node on port ${PORT} ..."
if [[ ! -x "$BIN" ]]; then
  echo "ERROR: geth binary not found at ${BIN}. Build it first: (cd Core-Blockchain/node_src && make geth)" >&2
  exit 1
fi

# Stop any previous instance on this PID file
if [[ -f "$PIDF" ]]; then
  if kill -0 "$(cat "$PIDF")" 2>/dev/null; then
    echo "[x402] Killing previous dev node PID $(cat "$PIDF")"
    kill "$(cat "$PIDF")" || true
    sleep 1
  fi
  rm -f "$PIDF"
fi

# Start dev node with x402 + personal APIs
nohup "$BIN" --dev \
  --http --http.addr 127.0.0.1 --http.port "$PORT" \
  --http.api eth,net,web3,personal,txpool,miner,debug,x402 \
  --allow-insecure-unlock \
  --ipcdisable --nodiscover --maxpeers 0 \
  --verbosity 3 >"$LOG" 2>&1 & echo $! >"$PIDF"

# Ensure cleanup on exit
cleanup() {
  if [[ -f "$PIDF" ]]; then
    if kill -0 "$(cat "$PIDF")" 2>/dev/null; then
      kill "$(cat "$PIDF")" || true
    fi
    rm -f "$PIDF"
  fi
}
trap cleanup EXIT

# Wait for RPC
echo -n "[x402] Waiting for RPC..."
for i in {1..30}; do
  if curl -s -H "Content-Type: application/json" -X POST --data '{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}' "http://127.0.0.1:${PORT}" >/dev/null; then
    echo " up"
    break
  fi
  echo -n "."
  sleep 1
done

rpc_call() {
  local DATA="$1"
  curl -s -H "Content-Type: application/json" -X POST --data "$DATA" "http://127.0.0.1:${PORT}"
}
json_result() { python3 -c 'import sys,json; j=json.load(sys.stdin); print(j.get("result",""))'; }
json_is_valid() { python3 -c 'import sys,json; j=json.load(sys.stdin); print(j.get("result",{}).get("isValid") is True)'; }
json_field() {
  local FIELD="$1"
  python3 - "$FIELD" <<'PY'
import sys,json
j=json.load(sys.stdin)
field=sys.argv[1]
r=j.get("result",{})
print(r.get(field,""))
PY
}

# Create new account (empty pass)
echo "[x402] Creating dev account..."
RES=$(rpc_call '{"jsonrpc":"2.0","method":"personal_newAccount","params":[""],"id":1}')
NEWACC=$(echo "$RES" | json_result)
if [[ -z "$NEWACC" ]]; then
  echo "ERROR: Failed to create account: $RES" >&2
  exit 1
fi
echo "[x402] New account: ${NEWACC}"

# Set etherbase to this account

# Unlock it for signing
echo "[x402] Unlocking account..."
rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"personal_unlockAccount\",\"params\":[\"${NEWACC}\",\"\",600],\"id\":1}" >/dev/null

# Get chainId (net_version as decimal string)
CHAINID=$(rpc_call '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' | json_result)
echo "[x402] ChainID: ${CHAINID}"

# Fund the new account to satisfy balance check
echo "[x402] Funding account for test..."
FUNDER=$(rpc_call '{"jsonrpc":"2.0","method":"eth_coinbase","params":[],"id":1}' | json_result)
if [[ -z "$FUNDER" ]]; then
  echo "ERROR: Could not determine a funding account" >&2
  exit 1
fi
# Unlock funder (dev account usually has empty password in dev mode)
rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"personal_unlockAccount\",\"params\":[\"${FUNDER}\",\"\",600],\"id\":1}" >/dev/null
# Ensure miner is running so funding tx gets mined
rpc_call '{"jsonrpc":"2.0","method":"miner_start","params":[1],"id":1}' >/dev/null

# Send 0.01 SPLD to the NEWACC so it has balance (more than required 0.001)
FUND_HEX=0x2386f26fc10000  # 0.01 * 1e18
SEND_TX=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"${FUNDER}\",\"to\":\"${NEWACC}\",\"value\":\"${FUND_HEX}\"}],\"id\":1}")
FUND_HASH=$(echo "$SEND_TX" | json_result)
echo "[x402] Funding tx: ${FUND_HASH}"
# Wait for fund receipt
for i in {1..100}; do
  RCP=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"${FUND_HASH}\"],\"id\":1}")
  HAS=$(echo "$RCP" | python3 - <<'PY'
import sys,json
s=sys.stdin.read().strip()
try:
    j=json.loads(s)
    print("1" if j.get("result") else "")
except Exception:
    print("")
PY
)
  if [[ "$HAS" == "1" ]]; then
    echo "[x402] Funding confirmed"
    # small delay to ensure state reflects the mined transfer
    sleep 0.5
    break
  fi
  echo -n "."
  sleep 0.2
done

echo "[x402] Proceeding to sign and verify..."
# Prepare payment: pay to self, 0.001 SPLD
AMT_DEC=1000000000000000            # 1e15 wei = 0.001
AMT_HEX=0x38d7ea4c68000
NOW=$(date +%s)
VALID_AFTER=$((NOW-10))
VALID_BEFORE=$((NOW+300))
NONCE=0x$(openssl rand -hex 32)

# Build v2 message string for signature (MUST match server verify format: value as hex string)
MSG="x402-payment:${NEWACC}:${NEWACC}:${AMT_HEX}:${VALID_AFTER}:${VALID_BEFORE}:${NONCE}:${CHAINID}"
# Convert message to hex for personal_sign (argv-based to avoid stdin redirection issues)
MSGHEX=0x$(python3 -c 'import binascii,sys; print(binascii.hexlify(sys.argv[1].encode()).decode())' "$MSG")

# Sign message with EIP-191 (personal_sign). Try variants if needed.
RESP=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"personal_sign\",\"params\":[\"${MSGHEX}\",\"${NEWACC}\",\"\"],\"id\":1}")
SIG=$(echo "$RESP" | json_result)
if [[ -z "$SIG" || "$SIG" == "null" ]]; then
  RESP=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"personal_sign\",\"params\":[\"${MSGHEX}\",\"${NEWACC}\"],\"id\":1}")
  SIG=$(echo "$RESP" | json_result)
fi
# Some clients/servers use reversed params for personal_sign: [address, data]
if [[ -z "$SIG" || "$SIG" == "null" ]]; then
  RESP=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"personal_sign\",\"params\":[\"${NEWACC}\",\"${MSGHEX}\"],\"id\":1}")
  SIG=$(echo "$RESP" | json_result)
fi
# Fallback to eth_sign (raw keccak without EIP-191)
if [[ -z "$SIG" || "$SIG" == "null" ]]; then
  RESP=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sign\",\"params\":[\"${NEWACC}\",\"${MSGHEX}\"],\"id\":1}")
  SIG=$(echo "$RESP" | json_result)
fi
if [[ -z "$SIG" || "$SIG" == "null" ]]; then
  echo "ERROR: Failed to sign x402 message. Last response: $RESP" >&2
  exit 1
fi
# Debug: show message and signature summaries
echo "[x402] DEBUG MSG: $MSG"
echo "[x402] DEBUG MSGHEX length: ${#MSGHEX}"
echo "[x402] DEBUG SIG length: $(echo -n "$SIG" | wc -c)"
echo "[x402] DEBUG SIG (prefix): $(echo -n "$SIG" | cut -c1-12)..."

# Recover signer with personal_ecRecover to validate client-side
ECREC_REQ=$(cat <<JSON
{"jsonrpc":"2.0","method":"personal_ecRecover","params":["${MSG}","${SIG}"],"id":1}
JSON
)
ECREC=$(rpc_call "$ECREC_REQ")
ECREC_ADDR=$(echo "$ECREC" | json_result)
echo "[x402] DEBUG ecRecover address: ${ECREC_ADDR}"
echo "[x402] DEBUG expected (from):   ${NEWACC}"

# Build PaymentRequirements and PaymentPayload
REQ=$(cat <<JSON
{
  "scheme":"exact",
  "network":"splendor",
  "maxAmountRequired":"${AMT_HEX}",
  "resource":"/api/test",
  "description":"Test payment",
  "mimeType":"application/json",
  "payTo":"${NEWACC}",
  "maxTimeoutSeconds":300,
  "asset":"0x0000000000000000000000000000000000000000"
}
JSON
)
PAY=$(cat <<JSON
{
  "x402Version":1,
  "scheme":"exact",
  "network":"splendor",
  "payload":{
    "from":"${NEWACC}",
    "to":"${NEWACC}",
    "value":"${AMT_HEX}",
    "validAfter":${VALID_AFTER},
    "validBefore":${VALID_BEFORE},
    "nonce":"${NONCE}",
    "signature":"${SIG}"
  }
}
JSON
)

echo "[x402] x402_supported"
rpc_call '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}'

echo "[x402] x402_verify"
VERIFY_REQ=$(cat <<JSON
{"jsonrpc":"2.0","method":"x402_verify","params":[${REQ},${PAY}],"id":1}
JSON
)
VERIFY=$(rpc_call "$VERIFY_REQ")
echo "$VERIFY"

OK=$(echo "$VERIFY" | json_is_valid)
if [[ "$OK" != "True" ]]; then
  echo "ERROR: Verify failed" >&2
  exit 1
fi

echo "[x402] x402_settle"
SETTLE_REQ=$(cat <<JSON
{"jsonrpc":"2.0","method":"x402_settle","params":[${REQ},${PAY}],"id":1}
JSON
)
SETTLE=$(rpc_call "$SETTLE_REQ")
echo "$SETTLE"

TXH=$(echo "$SETTLE" | json_field "txHash")
if [[ -z "$TXH" ]]; then
  echo "ERROR: Settle did not return txHash" >&2
  tail -n 200 "$LOG" | sed -n '/X402:/,$p' | tail -n 200 || true
  exit 1
fi
echo "[x402] txHash: ${TXH}"

# Poll for receipt
echo -n "[x402] Waiting for receipt"
for i in {1..100}; do
  RCP=$(rpc_call "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"${TXH}\"],\"id\":1}")
  STATUS=$(echo "$RCP" | python3 - <<'PY'
import sys,json
s=sys.stdin.read().strip()
try:
    j=json.loads(s)
    r=j.get("result")
    print(r.get("status") if r else "")
except Exception:
    print("")
PY
)
  if [[ -n "$STATUS" ]]; then
    echo " mined"
    echo "$RCP" | python3 -m json.tool
    break
  fi
  echo -n "."
  sleep 1
done

echo "[x402] Done. Log tail:"
tail -n 100 "$LOG" | sed -n '/X402:/,$p' | tail -n 100 || true
