#!/usr/bin/env bash
set -euo pipefail

# Simple ML-DSA precompile (0x0100) sanity check via eth_call.
# Sends a minimal header-only payload (alg=0x65, zero lengths) which will return false (32 zero bytes)
# but confirms the precompile is callable and wired.

RPC_URL="${RPC_URL:-http://127.0.0.1:80}"
TO_ADDR="0x0000000000000000000000000000000000000100"
DATA="0x650000000000000000000000"  # 0x65 + 3x uint32(0)

read -r -d '' PAYLOAD <<EOF
{
  "jsonrpc": "2.0",
  "method": "eth_call",
  "params": [
    {"to": "${TO_ADDR}", "data": "${DATA}"},
    "latest"
  ],
  "id": 1
}
EOF

echo "Calling ML-DSA precompile at ${TO_ADDR} ..."
resp=$(curl -s -H 'Content-Type: application/json' --data "${PAYLOAD}" "${RPC_URL}")
echo "Response: ${resp}"
echo
echo "Expected: result = 0x00..00 (32 bytes false), confirming precompile is wired."

