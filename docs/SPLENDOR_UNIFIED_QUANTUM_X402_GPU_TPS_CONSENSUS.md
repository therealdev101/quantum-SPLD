Title: Splendor Unified Architecture: Quantum-Resistant Signatures, Native x402 Payments, and GPU-Accelerated TPS

Version: 1.0
Status: Adopted
Last Updated: 2025-09-15

1) Executive Summary
This document consolidates the complete Splendor architecture into a single, authoritative specification. It covers:
- Quantum Resistance: ML-DSA (Dilithium) signatures via liboqs, fully integrated into the client with CGO support and dynamic sizing.
- Native x402 Payments: First-class micropayments protocol embedded in the blockchain node, with HTTP-native APIs and middleware support.
- GPU Acceleration: CUDA-based hybrid processing pipeline and GPU RPC introspection, delivering multi-million TPS on suitable hardware.

The repository contains explicit evidence of high throughput:
- Images/2.35mTPS.jpeg: peak run highlight
- Proofs/Headers and Full Blocks: serialized headers and full block bodies for verification
- Proof tooling via proofs/dump-specific-blocks.sh

2) Components Overview
2.1 Quantum-Resistant Signatures (PQ/ML-DSA)
- Algorithm: ML-DSA (FIPS 204), implemented via liboqs
- Variants:
  - ML-DSA-44 (Dilithium2 mapping)
  - ML-DSA-65 (Dilithium3 mapping)
  - ML-DSA-87 (Dilithium5 mapping)
- Integration:
  - CGO-based wrapper in crypto/mldsa with algorithm name mapping (ML-DSA-44/65/87 -> Dilithium2/3/5)
  - Fallback to static sizes if runtime queries are unavailable (e.g., static lib)
  - Unit tests use dynamic lengths via GetMLDSALengths to remain portable across liboqs builds

2.2 Native x402 Payments
- HTTP-native micropayments protocol embedded in the node
- JSON-RPC surface (examples):
  - x402_supported: Enumerates supported networks/schemes
  - x402_verify: Validates a payment without executing
  - x402_settle: Executes/settles a payment
- Middleware (x402-middleware):
  - Express.js and Fastify support for automatic 402 behavior
  - Integration examples and test harness under Core-Blockchain/x402-middleware

2.3 GPU Acceleration (CUDA/OpenCL)
- CUDA kernels compiled and linked into the client; GPU processor initialized at runtime
- Hybrid CPU/GPU pipeline with adaptive thresholds, worker parallelism, and pipelining
- GPU RPC: Runtime introspection via gpu RPC namespace (see Section 6)
- Environment-based tuning through .env: batching thresholds, worker counts, utilization targets

3) Repository Layout (Major Artifacts)
- Core-Blockchain/
  - node_src/ … main client source tree (geth)
  - node-setup.sh … end-to-end setup (drivers/CUDA/OpenCL/liboqs/build)
  - node-start.sh … end-to-end startup (tmux, env, sync-helper; GPU/x402 RPC exposed)
  - x402-middleware/ … HTTP middleware package for x402 integrations
- docs/ … (this document + migrated legacy docs)
- images/ … throughput snapshots (2.35mTPS.jpeg, others)
- proofs/ … serialized evidence (headers, blocks, dump tools)

4) Build and Install Flow (Automated)
Step 1: Setup (root privileges)
- Installs/updates NVIDIA driver, CUDA (runfile where needed), and OpenCL
- Builds liboqs (v0.8.0 by default) with Ninja/CMake; installs into repo-local path
- Exports space-safe headers/libs: /tmp/splendor_liboqs/include and /tmp/splendor_liboqs/lib
- Compiles CUDA kernels; links CUDA and liboqs into geth

Command:
sudo bash Core-Blockchain/node-setup.sh --rpc --validator 0 --nopk

Notes:
- The script configures persistent CUDA environment in /etc/profile and user bashrc
- It updates Makefile.cuda CUDA_ARCH based on GPU detection (Ada/Ampere/Turing/Professional)
- Adds .env with GPU defaults and high-TPS knobs (see Section 7)

Step 2: Start the node(s)
cd Core-Blockchain
./node-start.sh --rpc

Notes:
- Exports CUDA env if ENABLE_GPU=true in .env
- Starts nodes in tmux; includes GPU RPC namespace and x402 in --http.api
- Boot metrics include x402 and vLLM optional components

5) Quantum-Resistant Integration (Details)
5.1 Code Structure
- crypto/mldsa/
  - mldsa_cgo.go: CGO bindings to liboqs. Maps Splendor algorithm names (ML-DSA-44/65/87) to liboqs (Dilithium2/3/5).
  - mldsa_common.go: (cgo,!no_liboqs) constants, dynamic parameter map, and validation helpers.
  - mldsa.go: Fallback (no cgo or no_liboqs) with explicit error semantics and static parameters.
  - mldsa_test.go: Tests use GetMLDSALengths dynamically; integration test passes when liboqs is present.

5.2 Build Flags
- Makefile.pq:
  - Builds liboqs into Core-Blockchain/node_src/crypto/liboqs/install
  - Creates /tmp/splendor_liboqs symlink to avoid space-in-path issues
  - CGO flags:
    - CGO_CFLAGS: -I/tmp/splendor_liboqs/include
    - CGO_LDFLAGS: -L/tmp/splendor_liboqs/lib -loqs -lcrypto -lssl -ldl -lpthread

5.3 Runtime (Consensus Hooks and Precompiles)
- Precompile: ML-DSA verify precompile exposed at fixed address (see core/vm/contracts_pq.go)
- Consensus: pqconsensus and clique_pq include ML-DSA-aware paths (algorithm selection/verification)

6) GPU Acceleration (Details)
6.1 Runtime API Surface
- GPU RPC namespace is enabled at startup:
  --http.api db,eth,net,web3,personal,txpool,miner,debug,x402,gpu
- Example calls:
  - gpu_getGPUStats:
    Returns health: deviceCount, available flag, per-queue sizes, miner.gpuEnabled flag, and utilization
  - gpu_getGPUConfig:
    Reports configured thresholds, batching, and worker counts
  - gpu_getGPUHealth / gpu_getTPSMonitoring:
    Convenience introspection endpoints for perf testing and telemetry

6.2 Initialization and Hybrid Pipeline
- GPU initialized during startNode() via initializeGPUAcceleration()
- Hybrid config:
  - EnableGPU=true
  - GPUThreshold: offload minimum batch size to GPU (e.g., 500–1000)
  - Load balancing: CPU/GPU ratio and adaptive flags
  - Workers/pipelining: Hash/Signature/Tx workers tuned to GPU SM count and memory

6.3 Build/Link
- Makefile.cuda builds kernels and static artifacts
- geth linked with:
  - -L./common/gpu -lsplendor_cuda -lcudart -lcuda
  - CGO + PQ: liboqs + OpenSSL crypto libs

7) Configuration and Tuning (.env)
GPU-related defaults (created by setup):
- ENABLE_GPU=true
- PREFERRED_GPU_TYPE=CUDA
- GPU_MAX_BATCH_SIZE=160000
- GPU_MAX_MEMORY_USAGE=12884901888 (12 GiB)
- GPU_ENABLE_PIPELINING=true
- GPU_THRESHOLD=1000
- CPU_GPU_RATIO=0.85
- ADAPTIVE_LOAD_BALANCING=true
- MAX_CPU_UTILIZATION=0.85
- MAX_GPU_UTILIZATION=0.95
- THROUGHPUT_TARGET=3000000

Production tips (for 2.35M TPS envelope):
- Lower GPU_THRESHOLD (e.g., 500–800) to drive more traffic to GPU
- Increase cache sizes if RAM allows; disable debug/pprof in production
- Confirm CUDA MPS enabled on host when sharing GPU across services
- Utilize multiple RPC clients with persistent connections to saturate pipeline

8) Runbook
8.1 One-time setup
sudo bash Core-Blockchain/node-setup.sh --rpc --validator 0 --nopk

8.2 Start node
cd Core-Blockchain
./node-start.sh --rpc

8.3 Live verification
- x402:
  curl -s -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
  http://127.0.0.1:80
- GPU:
  curl -s -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"gpu_getGPUStats","params":[],"id":2}' \
  http://127.0.0.1:80

Expected GPU stats: {"gpu":{"type":"CUDA","deviceCount":1,"available":true,...},"miner":{"gpuEnabled":true,...}}

9) Evidence and Reproducibility
- Throughput snapshots:
  - images/2.35mTPS.jpeg (peak)
  - images/824kTPS.jpeg, 400kTPS.jpeg, 250kTPS.jpeg, 200kTPS.jpeg, 100kTPS.jpeg, tpsreport1.jpeg
- Block/headers artifacts in proofs/:
  - Headers.zip and headers for multiple heights (20980..21019)
  - Full blocks: Full Blocks/block-21018.json
  - dump-specific-blocks.sh for on-demand evidence extraction
- Keep proof artifacts under version control; run dump scripts on other nodes to validate structures and payload sizes.

10) Security Considerations
- PQ:
  - liboqs version pinned and validated; dynamic tests ensure parameter-size agreement
  - Precompile path enforces ML-DSA length checks and algorithm identifiers
- GPU:
  - Run with least-privileged accounts; audit LD_LIBRARY_PATH content
  - Avoid dev flags in production; disable extraneous RPC namespaces
- x402:
  - Rate limiting and nonce/timestamp checks recommended (see .env defaults)
  - Use HTTPS offload at the gateway for external traffic
  - Middleware should sanitize inputs and enforce per-endpoint pricing

11) Troubleshooting
- Missing CUDA:
  - Ensure nvidia-smi works; reboot if driver just installed
  - nvcc should be present; ensure /usr/local/cuda/bin in PATH
- liboqs link errors:
  - Re-run: make -f Makefile.pq liboqs
  - Verify /tmp/splendor_liboqs/include and lib paths exist
- GPU RPC missing:
  - Confirm node-start.sh sets --http.api includes gpu
- Low TPS:
  - Lower GPU_THRESHOLD; increase worker counts
  - Disable logging/pprof; scale up client connections

12) Change Log (Highlights)
- Added unified CUDA + PQ build in node-setup.sh
- Added GPU RPC namespace in node-start.sh and persistent CUDA env
- Added ML-DSA test portability with dynamic sizing
- Consolidated guidance from prior docs into this single specification

13) References
- PQ: https://github.com/open-quantum-safe/liboqs
- FIPS 204: ML-DSA specification
- NVIDIA CUDA: https://developer.nvidia.com/cuda-zone
- Splendor repo artifacts: images/ and proofs/

Appendix A: Commands Summary
Setup:
sudo bash Core-Blockchain/node-setup.sh --rpc --validator 0 --nopk

Start:
cd Core-Blockchain
./node-start.sh --rpc

x402 check:
curl -s -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' http://127.0.0.1:80

GPU check:
curl -s -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"gpu_getGPUStats","params":[],"id":2}' http://127.0.0.1:80
