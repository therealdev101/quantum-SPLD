# Splendor x402 – Production Deployment Guide

This guide covers launching a production-ready RPC node and a validator, with strict x402 signature verification enabled and no dev-mode unlocks. It also includes a minimal offline signer and a smoke test.

Prereqs
- Docker and docker compose v2
- A genesis.json (you can start from Core-Blockchain/genesis.json and adapt)
- A unique NETWORK_ID for your chain
- A validator key managed by a hardened signer (clef/HSM) – never unlock accounts over HTTP in production
- Reverse proxy/TLS termination for your exposed RPC endpoint (recommended)

What you get
- Strict x402 signature verification toggle (env X402_STRICT_VERIFY=1)
- Production Dockerfile to build/run geth binary (CPU-only)
- Hardened start script for RPC node
- docker-compose templates: “rpc” and “validator + clef”
- Offline canonical x402 signer tool (tools/x402sign)
- Unit tests for strict signature path

1) Build production images
From the repo root:
  docker compose -f deploy/docker-compose.yml build

This builds a CPU-only geth binary in a multi-stage image and uses it for both “rpc” and “validator” services.

2) Prepare genesis and network settings
- Replace NETWORK_ID in deploy/docker-compose.yml with your chain id.
- Mount your genesis.json where the compose expects it (../Core-Blockchain/genesis.json).
- Validate EVM fork blocks and TxTypeX402 decoding are enabled at genesis as per your chain’s config.

3) Launch the RPC node (strict verify ON)
Start the RPC node:
  docker compose -f deploy/docker-compose.yml up -d rpc

Defaults:
- X402_STRICT_VERIFY=1
- http.api=eth,net,web3,x402
- No personal, no insecure unlocks, IPC disabled, RPC on 8545
- Datadir initialized from GENESIS_PATH on first boot

Check logs:
  docker logs -f splendor-rpc

4) Launch the validator with an external signer (template)
Important: Replace the validator etherbase and secure your signer.

Start clef and validator:
  docker compose -f deploy/docker-compose.yml up -d clef validator

Notes:
- clef here runs with --stdio-ui for template purposes only. In production remove that, add a rules file and configure operator policies.
- Mount a secure keystore directory (or integrate with an HSM-backed signer).
- Adjust EXTRA_OPTS in validator service:
  EXTRA_OPTS=--signer=/clef/clef.ipc --mine --miner.etherbase=0xYourValidatorAddress

5) Strict signature verification – canonical message
Strict mode requires this exact message format:
  x402-payment:{from}:{to}:{value}:{validAfter}:{validBefore}:{nonce}:{chainId}

- from/to: checksum addresses (0x…)
- value: hex string (0x…)
- validAfter/validBefore: unix seconds (uint64)
- nonce: 0x-prefixed 32-byte hex
- chainId: uint

Hashing/signing:
- EIP-191 prefix (Ethereum Signed Message): accounts.TextHash(payload)
- Recover and compare to from; signature length must be 65 bytes and v normalized (27/28 accepted)

6) Offline signer (tools/x402sign)
Build:
  cd tools/x402sign
  go build -o x402sign

Usage:
- Print the signer address from a hex private key:
  ./x402sign -key 0x<privhex> addr
- Sign a canonical x402 message string:
  ./x402sign -key 0x<privhex> sign "x402-payment:0xFrom:0xTo:0xValueHex:ValidAfter:ValidBefore:0xNonce:ChainId"

Output is a 0x-prefixed 65-byte signature (r||s||v) with v ∈ {27,28}.

7) Smoke test: verify → settle on RPC
Assuming RPC at http://localhost:8545 and an account 0xFrom funded with native coin:

- Build canonical message text (replace placeholders):
  MSG="x402-payment:0xFrom:0xTo:0x38d7ea4c68000:$(date +%s -d '-10 sec'):$(date +%s -d '+5 min'):0x<32-byte-nonce-hex>:1337"

- Sign with x402sign:
  SIG=$(./x402sign -key 0x<priv> sign "$MSG")

- Compose PaymentRequirements and PaymentPayload JSON with the same fields:
  Requirements:
    {
      "scheme":"exact","network":"splendor",
      "maxAmountRequired":"0x38d7ea4c68000","resource":"/api/test",
      "description":"Test payment","mimeType":"application/json",
      "payTo":"0xTo","maxTimeoutSeconds":300,
      "asset":"0x0000000000000000000000000000000000000000"
    }
  Payload:
    {
      "x402Version":1,"scheme":"exact","network":"splendor",
      "payload":{
        "from":"0xFrom","to":"0xTo","value":"0x38d7ea4c68000",
        "validAfter":<valAfter>,"validBefore":<valBefore>,
        "nonce":"0x<32-byte-hex>","signature":"0x<65-byte-sig>"
      }
    }

- Call RPC:
  curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"x402_verify","params":[REQUIREMENTS_JSON,PAYLOAD_JSON],"id":1}' \
    http://localhost:8545

  Expect: {"result":{"isValid":true,...}}

- Settle:
  curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"x402_settle","params":[REQUIREMENTS_JSON,PAYLOAD_JSON],"id":1}' \
    http://localhost:8545

  Expect: {"result":{"success":true,"txHash":"0x..."}}

- Confirm receipt:
  curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt","params":["0x..."],"id":1}' \
    http://localhost:8545

8) Production hardening checklist
- RPC exposure: Put behind TLS and auth; restrict CORS/vhosts to your domains; rate-limit x402 endpoints.
- Remove dev modules: No personal API, no insecure unlocks; IPC disabled for RPC node.
- Validators: External signer only (clef/HSM). Strong rulesets; no unattended approvals.
- Economics: If you later enable fees, set X402_TREASURY_ADDRESS and adjust the revenue path; otherwise leave zero-fee policy (100% to payTo).
- Anti-replay: Ensure the consensus (ApplySysTx) path records (from,nonce) on-chain. Confirm duplicate rejection after reorgs in staging.
- Monitoring: Export x402 metrics and log settlements; alert on spikes in invalidReason, failed settlements, or replay attempts.

9) Switching strict mode
- Enabled by default via X402_STRICT_VERIFY=1. To relax (staging interop), set X402_STRICT_VERIFY=0 on the RPC node.
- Production should keep strict mode ON and require canonical client signing behavior.

10) Validator/Signer notes
- Update validator etherbase and connect to a secure signer (clef or HSM adapter).
- Do not expose validator RPC publicly.
- Tune p2p, discovery, and peering according to your network’s topology.

If you need a one-command staging bring-up:
  docker compose -f deploy/docker-compose.yml up -d

Then point a client to the RPC endpoint and follow section 7 to run a canonical payment through verify → settle → receipt.
