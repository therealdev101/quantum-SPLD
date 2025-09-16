# Splendor Blockchain V4 — Unified Quantum + x402 + GPU High‑Performance EVM

[![License: SBSAL](https://img.shields.io/badge/License-SBSAL-red.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![Network Status](https://img.shields.io/badge/Mainnet-Live-brightgreen.svg)](https://mainnet-rpc.splendor.org/)
[![AI Powered](https://img.shields.io/badge/AI-TinyLlama_1.1B-purple.svg)](docs/GETTING_STARTED.md)
[![GPU Accelerated](https://img.shields.io/badge/GPU-RTX_4000_SFF_Ada-orange.svg)](docs/GETTING_STARTED.md)

A unified, production‑grade EVM blockchain that combines:
- Quantum‑resistant signatures (ML‑DSA/Dilithium via liboqs)
- Native x402 HTTP‑native micropayments protocol
- CUDA‑accelerated hybrid CPU/GPU pipeline capable of verified multi‑million TPS

Refer to the consolidated technical spec:
- docs/SPLENDOR_UNIFIED_QUANTUM_X402_GPU_TPS_CONSENSUS.md

## Overview

Splendor Blockchain V4 integrates post‑quantum cryptography, native x402 micropayments, and GPU‑accelerated execution in a single client. The system has verified performance peaks of 2.35M TPS on the documented target hardware, with complete on‑chain proof artifacts and screenshots included in this repository.

### Key Capabilities

- Quantum Resistance (PQ/ML‑DSA/FIPS‑204):
  - ML‑DSA‑44/65/87 (mapped to liboqs Dilithium2/3/5) with CGO integration
  - Dynamic sizing via runtime queries; precompile and consensus hooks
- Native x402 Payments:
  - HTTP‑native 402 semantics (x402_supported/x402_verify/x402_settle)
  - Ready‑to‑use middleware (Core‑Blockchain/x402‑middleware)
- GPU Acceleration (CUDA/OpenCL):
  - Hybrid CPU/GPU pipeline with thresholds, worker parallelism, and pipelining
  - GPU RPC namespace for runtime introspection and health reporting
- AI‑Powered Optimization (TinyLlama 1.1B via vLLM):
  - Optional local AI service for load balancing and throughput tuning
- Full EVM Compatibility:
  - Operates with standard Ethereum tooling and client libraries

## Performance & Evidence

- Verified peak: 2.35M TPS  
- Sustained: 824K+ TPS  
- Block time: ~1 second

Artifacts in repository:
- Screenshots (images/):
  - 2.35mTPS.jpeg (peak)
  - 824kTPS.jpeg, 400kTPS.jpeg, 250kTPS.jpeg, 200kTPS.jpeg, 100kTPS.jpeg, tpsreport1.jpeg
- On‑chain proofs (proofs/):
  - Headers/ (multiple heights, including 20980–21019)
  - Full Blocks/block-21018.json
  - dump-specific-blocks.sh (script to re‑extract/verify)

## Unified Architecture (Quantum + x402 + GPU)

See the full spec document (highly recommended):
- docs/SPLENDOR_UNIFIED_QUANTUM_X402_GPU_TPS_CONSENSUS.md

Highlights:
- PQ/ML‑DSA:
  - crypto/mldsa (CGO: mldsa_cgo.go, common: mldsa_common.go, fallback: mldsa.go)
  - Tests adapted to runtime sizes (GetMLDSALengths), portable across liboqs builds
- x402 Native:
  - JSON‑RPC methods: x402_supported, x402_verify, x402_settle
  - Middleware for Express/Fastify under Core-Blockchain/x402-middleware
- GPU:
  - CUDA kernels via Makefile.cuda; linked into geth
  - GPU RPC namespace enabled at runtime for stats and telemetry

## Quick Start (Automated)

The following scripts integrate GPU and PQ build/run in sync.

1) One‑time setup (root)
```bash
sudo bash Core-Blockchain/node-setup.sh --rpc --validator 0 --nopk
```
What it does:
- Installs/updates NVIDIA drivers, CUDA (runfile if needed), OpenCL
- Builds liboqs (ML‑DSA) and compiles geth with CUDA + PQ linkage
- Writes .env with GPU defaults (ENABLE_GPU=true)
- Optionally installs vLLM/TinyLlama for AI load balancing
- Adds x402 configuration and test utilities

2) Start node(s)
```bash
cd Core-Blockchain
./node-start.sh --rpc
```
What it does:
- Exports CUDA env if ENABLE_GPU=true
- Starts RPC node(s) in tmux with:
  ```
  --http.api db,eth,net,web3,personal,txpool,miner,debug,x402,gpu
  ```
- Starts sync-helper and prints status

3) Live verification (port 80)
- x402 API:
```bash
curl -s -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
  http://127.0.0.1:80
```
- GPU stats:
```bash
curl -s -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"gpu_getGPUStats","params":[],"id":2}' \
  http://127.0.0.1:80
```
Expected GPU results include: type=CUDA, deviceCount≥1, available=true, miner.gpuEnabled=true.

## Technical Architecture

### Quantum Resistance (ML‑DSA/FIPS‑204)
- liboqs v0.8.0 built via Makefile.pq (CMake/Ninja)
- CGO bindings (crypto/mldsa/mldsa_cgo.go) map ML‑DSA‑44/65/87 to Dilithium2/3/5
- Precompile path validates ML‑DSA parameters and verifies signatures
- Consensus hooks include ML‑DSA awareness where applicable

### Native x402 Micropayments
- JSON‑RPC:
  - x402_supported: list of supported schemes/networks
  - x402_verify: validates payment
  - x402_settle: executes payment
- Middleware (x402-middleware):
  - Easily add payments to HTTP endpoints (Express/Fastify)
  - Example + test harness included

### GPU Hybrid Processing
- CUDA kernels compiled with Makefile.cuda (arch auto‑detected in setup)
- Runtime GPU initialization and hybrid scheduler
- Workers and thresholds configurable via .env:
  - GPU_THRESHOLD, GPU_*_WORKERS, GPU_MAX_BATCH_SIZE, THROUGHPUT_TARGET
- GPU RPC namespace supports health/config/TPS introspection

## Target Hardware Profile

- GPU: NVIDIA RTX 4000 SFF Ada (20GB VRAM)
- CPU: 16+ cores (32+ threads) @ 3.0+ GHz
- RAM: 64GB+
- Storage: NVMe SSD (2TB+, ~7GB/s)
- Network: Gigabit+

For 2.35M TPS envelope:
- Ensure GPU drivers are active (nvidia-smi shows device)
- CUDA toolkit available (nvcc, LD_LIBRARY_PATH)
- Disable dev flags in production (no vmdebug/pprof)
- Scale client connections; use persistent RPC sessions

## Developer Notes

- PQ tests:
```bash
cd Core-Blockchain/node_src
make -f Makefile.pq pq-test   # mldsa package tests (PASS)
```
- Middleware (x402):
```bash
cd Core-Blockchain/x402-middleware
npm install
# See README in middleware and root x402 test scripts
```

## Documentation Index

- Unified Architecture (this repo’s authoritative spec)
  - docs/SPLENDOR_UNIFIED_QUANTUM_X402_GPU_TPS_CONSENSUS.md
- Getting Started
  - docs/GETTING_STARTED.md
- Validator Guide
  - docs/VALIDATOR_GUIDE.md
- API Reference
  - docs/API_REFERENCE.md
- Smart Contracts
  - docs/SMART_CONTRACTS.md
- Security
  - docs/SECURITY.md
- Additional Documents (migrated)
  - docs/AI_SYSTEM_COMPREHENSIVE_DOCUMENTATION.md
  - docs/CHANGELOG.md
  - docs/CHANGES.md
  - docs/DEPLOYMENT_GUIDE.md
  - docs/PARALLEL_PROCESSING_SUMMARY.md
  - docs/PROJECT_STRUCTURE.md
  - docs/QUANTUM_RESISTANCE_DEPLOYMENT_GUIDE.md
  - docs/QUANTUM_RESISTANCE_IMPLEMENTATION_PLAN.md
  - docs/QUANTUM_RESISTANCE_IMPLEMENTATION_SUMMARY.md
  - docs/SECURITY_CONCERNS_ANALYSIS.md
  - docs/SPLENDOR_X402_REVOLUTIONARY_ANALYSIS.md

## License

This project is licensed under the **Splendor Blockchain Source‑Available License (SBSAL) v1.0** — see [LICENSE](LICENSE).

**Permitted:**
- Security auditing, research, education
- Connecting to the official Splendor network
- Personal non‑commercial modifications
- Contributions to the official repo

**Prohibited without written permission:**
- Forks for competing networks
- Commercial use or resale
- Operating separate networks
- Removing Splendor branding

Commercial inquiries: legal@splendor.org

© 2025 Splendor Labs S.A. All rights reserved.

---

Built with AI by the Splendor Team — advancing blockchain through AI, PQ, and GPU acceleration.
