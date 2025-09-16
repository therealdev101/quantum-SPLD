# Splendor x402 Native Payments Protocol – Developer Guide

This guide documents the x402 JSON‑RPC API implemented in your node, plus a simple HTTP middleware to add pay‑per‑request to any API. It reflects the current code and includes request/response schemas, signature spec, examples, and caveats.

Contents
- What x402 is
- Quickstart (start node and probe)
- API Reference
  - x402_supported
  - x402_verify
  - x402_settle
  - Revenue and settings: x402_getValidatorRevenue, x402_getRevenueStats, x402_getTopPerformingValidators, x402_setValidatorFeeShare, x402_setDistributionMode
- Signature specification (EIP‑191 + chainId; legacy fallback)
- Middleware usage (Express/Fastify)
- Errors, security, and anti‑replay
- GPU acceleration notes
- Production roadmap and caveats
- Troubleshooting

What x402 is
- A native micropayments protocol exposed via JSON‑RPC under the "x402" namespace on your node.
- HTTP‑friendly flow:
  1) Client calls an API endpoint without payment → receives 402 Payment Required with “accepts” requirements.
  2) Client signs a payment payload and retries with an X‑Payment header containing the base64 JSON payload.
  3) Server middleware calls node RPC x402_verify (precheck) then x402_settle (execute).
- Exact‑amount semantics (scheme "exact") with signature + time window + anti‑replay checks.
- Revenue split (in code): 90% developer (payTo), 5% validators, 5% protocol treasury (treasury configurable via env X402_TREASURY_ADDRESS).

Quickstart

1) Start a node with x402 RPC
- Helper scripts may expose RPC on port 80; examples below use 8545.

Example (dev mode, 8545):
```
./build/bin/geth --dev --http --http.addr 127.0.0.1 --http.port 8545 \
  --http.api db,eth,net,web3,txpool,miner,debug,x402 \
  --ipcdisable --nodiscover --maxpeers 0
```

2) Probe that x402 is enabled
```
curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
  http://127.0.0.1:8545
```

Expected:
```
{"jsonrpc":"2.0","id":1,"result":{"kinds":[{"scheme":"exact","network":"splendor"}]}}
```

API Reference

All methods are JSON‑RPC 2.0 over the node HTTP endpoint.

Types

PaymentRequirements (params[0] to verify/settle)
- scheme: "exact"
- network: "splendor"
- maxAmountRequired: hex string (wei), e.g. "0x38d7ea4c68000" for 0.001 SPLD if 1e18 wei per coin
- resource: string (e.g. "/api/weather")
- description: string
- mimeType: "application/json"
- payTo: 0x... (address to receive developer share)
- maxTimeoutSeconds: number (e.g., 300)
- asset: 0x0000000000000000000000000000000000000000 (native coin for now)

PaymentPayload (params[1] to verify/settle)
```
{
  "x402Version": 1,
  "scheme": "exact",
  "network": "splendor",
  "payload": {
    "from": "0xFromAddress",
    "to": "0xToAddress",
    "value": "0x...",          // wei
    "validAfter":  1694790000, // unix seconds
    "validBefore": 1694790300, // unix seconds
    "nonce": "0x...32bytes",
    "signature": "0x...65bytes"
  }
}
```

VerificationResponse
- isValid: bool
- invalidReason?: string
- payerAddress?: string

SettlementResponse
- success: bool
- error?: string
- txHash?: 0x...
- networkId?: "splendor"

1) x402_supported()
- Request:
```
{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}
```
- Response:
```
{"jsonrpc":"2.0","id":1,"result":{"kinds":[{"scheme":"exact","network":"splendor"}]}}
```

2) x402_verify(requirements, payload)
- Enforces:
  - scheme == "exact", network == "splendor"
  - validAfter <= now <= validBefore
  - signature valid (see Signature specification)
  - sender balance >= value
  - exact-amount: payload.payload.value == requirements.maxAmountRequired
  - recipient matches: payload.payload.to == requirements.payTo
  - in‑memory anti‑replay precheck: repeated (from, nonce) is rejected
- Example:
```
curl -s -X POST -H "Content-Type: application/json" \
  --data '{
    "jsonrpc":"2.0",
    "method":"x402_verify",
    "params":[
      {
        "scheme":"exact",
        "network":"splendor",
        "maxAmountRequired":"0x38d7ea4c68000",
        "resource":"/api/weather",
        "description":"API access payment",
        "mimeType":"application/json",
        "payTo":"0x6BED5A6606fF44f7d986caA160F14771f7f14f69",
        "maxTimeoutSeconds":300,
        "asset":"0x0000000000000000000000000000000000000000"
      },
      {
        "x402Version":1,
        "scheme":"exact",
        "network":"splendor",
        "payload":{
          "from":"0xFrom...",
          "to":"0x6BED5A6606fF44f7d986caA160F14771f7f14f69",
          "value":"0x38d7ea4c68000",
          "validAfter":  1694790000,
          "validBefore": 1694790300,
          "nonce":"0x...32bytes",
          "signature":"0x...65bytes"
        }
      }
    ],
    "id":1
  }' \
  http://127.0.0.1:8545
```
- Response:
```
{"jsonrpc":"2.0","id":1,"result":{"isValid":true,"payerAddress":"0xFrom..."}}
```

3) x402_settle(requirements, payload)
- Re-runs verification, atomically marks nonce used (in‑memory precheck), then submits a consensus-safe settlement.
- Consensus-safe settlement: The node builds and submits a typed transaction (TxTypeX402) to the txpool. The Congress engine executes settlement during block processing with durable on-chain anti‑replay and zero fees. The txHash returned is a real, mined transaction hash (check via eth_getTransactionReceipt).
- Example:
```
curl -s -X POST -H "Content-Type: application/json" \
  --data '{
    "jsonrpc":"2.0",
    "method":"x402_settle",
    "params":[ ...same as x402_verify... ],
    "id":1
  }' \
  http://127.0.0.1:8545
```
- Response:
```
{"jsonrpc":"2.0","id":1,"result":{"success":true,"txHash":"0x...","networkId":"splendor"}}
```

Revenue and settings

4) x402_getValidatorRevenue(validatorAddress)
- Returns the accumulated x402 revenue for a validator (in hex wei).
```
{"jsonrpc":"2.0","method":"x402_getValidatorRevenue","params":["0xValidator"],"id":1}
```

5) x402_getRevenueStats()
- Returns totals, averages, top validator, today’s stats.
```
{"jsonrpc":"2.0","method":"x402_getRevenueStats","params":[],"id":1}
```

6) x402_getTopPerformingValidators(limit)
- Returns validators ranked by AI performance score (from code’s in‑memory metrics).
```
{"jsonrpc":"2.0","method":"x402_getTopPerformingValidators","params":[10],"id":1}
```

7) x402_setValidatorFeeShare(percentage)
- Sets validator fee share (0..1). Default is 0.05 (5%).
```
{"jsonrpc":"2.0","method":"x402_setValidatorFeeShare","params":[0.05],"id":1}
```

8) x402_setDistributionMode(mode)
- Sets distribution mode: "proportional", "equal", or "performance".
```
{"jsonrpc":"2.0","method":"x402_setDistributionMode","params":["performance"],"id":1}
```

Signature specification

Preferred v2 (with chainId + EIP‑191 prefix):
- Message to sign (string):
```
x402-payment:{from}:{to}:{value}:{validAfter}:{validBefore}:{nonce}:{chainId}
```
- Hashing for signature: Ethereum Signed Message prefix (EIP‑191, accounts.TextHash)
- Verification tries v2 first.

Legacy v1 (fallback; no chainId):
- Message:
```
x402-payment:{from}:{to}:{value}:{validAfter}:{validBefore}:{nonce}
```
- Also hashed with EIP‑191 prefix.

Notes:
- signature must be 65 bytes (r,s,v). If v >= 27, it is normalized by subtracting 27.
- Recovered address must equal payload.payload.from.
- Use hex strings for addresses and fields as shown.

Middleware usage (Express/Fastify)

Install:
```
cp -r Core-Blockchain/x402-middleware ./x402-middleware
cd x402-middleware
npm install
```

Express:
```js
const express = require('express');
const { splendorX402Express } = require('./x402-middleware');

const app = express();
app.use('/api', splendorX402Express({
  payTo: '0xYourDeveloperWallet',
  rpcUrl: 'http://127.0.0.1:8545',
  pricing: {
    '/api/weather': '0.001',
    '/api/premium': '0.01',
    '/api/free': '0'
  },
  defaultPrice: '0.005'
}));

app.get('/api/weather', (req, res) => {
  res.json({ weather: 'Sunny 75°F', payment: req.x402 });
});

app.listen(3000, () => console.log('x402 test server on :3000'));
```

Header format:
- Server responds with 402 Payment Required body:
```
{
  "x402Version": 1,
  "accepts": [{
    "scheme":"exact",
    "network":"splendor",
    "maxAmountRequired":"0x...",
    "resource":"/api/weather",
    "description":"Payment required for /api/weather",
    "mimeType":"application/json",
    "payTo":"0xYourDeveloperWallet",
    "maxTimeoutSeconds":300,
    "asset":"0x0000000000000000000000000000000000000000"
  }]
}
```
- Client signs and sends:
  - X-Payment: base64(JSON of PaymentPayload object)
- Middleware then calls x402_verify → x402_settle and attaches req.x402 with details.

Errors, security, and anti‑replay
- invalidReason strings you may see from verify:
  - "Unsupported payment scheme" / "Unsupported network"
  - "Payment not yet valid" / "Payment expired"
  - "Invalid signature"
  - "Insufficient balance"
  - "Payment amount must equal required amount"
  - "Payment recipient mismatch"
  - "Payment nonce already used"
- Nonce tracking is currently in‑memory (demo). For production, enforce replay protection on‑chain (see roadmap).

GPU acceleration notes
- GPU acceleration does not change API surface; it speeds internal batch processing.
- Build configuration:
  - CPU-only (default): builds and runs everywhere.
  - GPU: build with cgo and tag "gpu" and ensure native libs present.
    - CUDA (optional): CUDA toolkit + project-local libcuda_kernels
    - OpenCL (optional): OpenCL ICD runtime
- At runtime, logs indicate:
  - "CUDA GPU acceleration enabled" or
  - "OpenCL GPU acceleration enabled", else CPU fallback.

Implementation details (consensus-safe, zero-fee)
- Typed transaction: x402 settlement uses an EIP‑2718 typed tx (TxTypeX402) carrying the signed payment payload. The tx is added to the txpool and mined normally.
- Engine execution: The Congress PoSA engine executes the settlement during block processing (ApplySysTx), verifying signature (EIP‑191 + chainId), exact‑amount, recipient, time window, and durable anti‑replay.
- Durable anti‑replay: (from, nonce) is recorded on-chain under a reserved registry address (0x…0402); duplicates are rejected by all nodes.
- Zero‑fee policy: No validator or protocol fee is taken; receiver gets exactly the amount; no additive credits.
- Receipts and indexing: x402 settlements have canonical tx hashes and receipts; use eth_getTransactionReceipt to confirm inclusion.

Troubleshooting

- “method not found” on x402_supported
  - Ensure your node is started with --http.api ... x402 and that backend.go registers the x402 namespace (Service: NewX402API(s)).
  - Restart the node after rebuilding.

- “bind: address already in use” when starting node
  - Another instance is already bound to the HTTP port (e.g., 8545/80). Stop it or choose a different --http.port.

- invalidReason: "Invalid signature"
  - Ensure client signs the exact message:
    - Preferred v2 (with chainId): `x402-payment:{from}:{to}:{value}:{validAfter}:{validBefore}:{nonce}:{chainId}`
    - Hash with EIP‑191 (Ethereum Signed Message) prefix before signing.
  - Make sure signature is 65 bytes and v is normalized if needed (v ∈ {27,28} → subtract 27).

- invalidReason: "Payment amount must equal required amount"
  - Scheme "exact" enforces equality: payload.value must equal requirements.maxAmountRequired exactly.
  - If using the middleware, confirm dollarToWei conversion matches the intended price and rounding.

- invalidReason: "Payment nonce already used"
  - Use a unique nonce per payment attempt; current anti‑replay is in‑memory (demo). For a fresh test, restart node or change nonce.

- x402_supported works but GPU not enabled
  - GPU acceleration is optional and does not affect API responses.
  - To enable: build with CGO and `-tags gpu`, install CUDA/OpenCL as applicable. Logs will show “CUDA GPU acceleration enabled” or “OpenCL GPU acceleration enabled”.

Appendix: Example payload and X‑Payment header

- Example PaymentPayload (JSON):
```
{
  "x402Version": 1,
  "scheme": "exact",
  "network": "splendor",
  "payload": {
    "from": "0xA1...B2",
    "to":   "0x6BED5A6606fF44f7d986caA160F14771f7f14f69",
    "value": "0x38d7ea4c68000",
    "validAfter":  1694790000,
    "validBefore": 1694790300,
    "nonce": "0x5a8b2f2b4c8edc1e5a8b2f2b4c8edc1e5a8b2f2b4c8edc1e5a8b2f2b4c8edc1e",
    "signature": "0x...65bytes..."
  }
}
```

- Base64 encode this JSON and send in the request header:
  - `X-Payment: eyJ4NDAyVmVyc2lvbiI6MS4uLn0=` (example)

- Minimal cURL (after you have X‑Payment):
```
curl -H "X-Payment: $(cat payment.json | base64 -w0)" http://localhost:3000/api/weather
```

Fastify usage (optional)
```js
const fastify = require('fastify')({ logger: true });
const { splendorX402Fastify } = require('./x402-middleware');

fastify.register(splendorX402Fastify({
  payTo: '0xYourDeveloperWallet',
  rpcUrl: 'http://127.0.0.1:8545',
  pricing: { '/api/*': '0.001' }
}));

fastify.get('/api/hello', async (request, reply) => {
  return { hello: 'world', payment: request.x402 };
});

fastify.listen({ port: 3000 });
```

This guide is kept code-accurate. If you change the API structs or signature logic in `Core-Blockchain/node_src/eth/api_x402.go`, please update:
- Signature specification (v2 string and hashing)
- Request/Response examples
- Error reasons and validation rules
- Economic splits and treasury address notes
