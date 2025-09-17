# Quick Start Guide

Get up and running with Splendor Blockchain V4 in minutes.

## Prerequisites

- **Operating System**: Ubuntu 20.04+ LTS (recommended) or Windows Server 2019+
- **Hardware**: 16+ CPU cores, 64GB+ RAM, NVMe SSD, NVIDIA GPU (required)

### Minimum And Recommended Hardware

- Minimum (enforced):
  - CPU: 14+ physical cores
  - GPU: NVIDIA RTX 40‑series class or ≥20GB VRAM (GPU required)
- Recommended (reference profile we test on):
  - CPU: Intel i5‑13500 (14C/20T, up to 4.8 GHz)
  - RAM: 62 GB total (≥57 GB free)
  - GPU: NVIDIA RTX 4000 SFF Ada (20 GB VRAM)
  - CUDA: 12.6 (with cuDNN + cuBLAS)
  - Driver: NVIDIA 575.57.08

The installer targets the recommended versions automatically; if different versions are present, setup continues but may log warnings.
- **Software**: Node.js 16+, Go 1.22+, Git

## Quick Setup (Automated)

### 1. Clone Repository
```bash
git clone https://github.com/Splendor-Protocol/splendor-blockchain-v4.git
cd splendor-blockchain-v4
```

### 2. One-Time Setup (requires root)
```bash
sudo bash Core-Blockchain/node-setup.sh --rpc --validator 0 --nopk
```

This script automatically:
- Installs NVIDIA drivers, CUDA toolkit, and OpenCL
- Builds liboqs (quantum-resistant cryptography)
- Compiles GPU kernels and links with geth
- Configures environment variables
- Sets up AI optimization (MobileLLM-R1)

## Validator GPU Requirement

- GPU acceleration is mandatory. Nodes without a working NVIDIA GPU stack will exit during startup with a fatal error.
- Ensure `nvidia-smi` works and CUDA/OpenCL runtimes are installed before starting.
- The miner enforces GPU availability via the hybrid processor; if GPU init fails or is disabled, startup aborts.

### 3. Start Node
```bash
cd Core-Blockchain
./node-start.sh --rpc
```

### 4. Verify Installation

**Test x402 Payments:**
```bash
curl -s -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
  http://127.0.0.1:80
```

**Test GPU Acceleration:**
```bash
curl -s -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"gpu_getGPUStats","params":[],"id":2}' \
  http://127.0.0.1:80
```

Expected GPU response:
```json
{
  "gpu": {
    "type": "CUDA",
    "deviceCount": 1,
    "available": true
  },
  "miner": {
    "gpuEnabled": true
  }
}
```

## What You Get

- **Mainnet Connection**: Chain ID 6546, RPC at https://mainnet-rpc.splendor.org/
- **Quantum Resistance**: ML-DSA signatures ready
- **GPU Acceleration**: Up to 2.35M TPS capability
- **x402 Payments**: Native micropayments protocol
- **Full EVM**: Compatible with MetaMask and standard tools

## Effective Config (Quick Check)

Before starting heavy tests, confirm the performance env you want to use. Defaults in this repo target 3M TPS and large GPU batches.

```bash
# Core performance knobs (override in Core-Blockchain/.env if needed)
export THROUGHPUT_TARGET=3000000        # 2,000,000 if you want a strict 2M run
export GPU_MAX_BATCH_SIZE=200000        # 160000–200000 recommended
export GPU_THRESHOLD=1000               # Offload to GPU at this batch size
export GPU_HASH_WORKERS=32
export GPU_SIGNATURE_WORKERS=32
export GPU_TX_WORKERS=32
```

The start script automatically exports these into tmux sessions, and geth logs an “effective config” line (targetTPS, batch, workers, etc.) after GPU init.

## Next Steps

1. **[Configure Your Node](03-CONFIGURATION.md)** - Optimize performance settings
2. **[Set up MetaMask](20-METAMASK_SETUP.md)** - Connect your wallet
3. **[Deploy Smart Contracts](15-SMART_CONTRACTS.md)** - Start building
4. **[Run as Validator](11-VALIDATOR_GUIDE.md)** - Earn rewards

## Common Commands

```bash
# Check node status
tmux list-sessions

# View node logs
tmux attach -t node1

# Stop node
./node-stop.sh

# Restart with different config
./node-start.sh --validator
```

## Troubleshooting

**Node won't start?**
- Check system requirements
- Verify GPU availability: `nvidia-smi` must show at least one device
- Verify CUDA installation: `nvcc --version`
- Check ports: `netstat -tulpn | grep :80`

**GPU not detected (startup fails)?**
- Ensure NVIDIA drivers installed
- Reboot after driver installation
- Check CUDA path: `nvcc --version`

**Need help?** See [Troubleshooting Guide](17-TROUBLESHOOTING.md)

## AI Load Balancing (LLM) — Optional

node-setup can install vLLM + MobileLLM‑R1. To confirm it’s working:

```bash
systemctl status vllm-mobilellm
curl -s http://localhost:8000/v1/models
```

On geth startup you should see “AI-powered GPU load balancing activated”. This allows the node to adapt batch size and GPU/CPU split under fluctuating load.

## Performance Targets

- **Entry Level**: 100K+ TPS (entry GPU)
- **Mid Range**: 500K+ TPS (RTX 3060+)
- **High End**: 1M+ TPS (RTX 4080+)
- **Extreme**: 2.35M TPS (RTX 4090+)

Ready to dive deeper? Check out the [Unified Architecture Specification](SPLENDOR_UNIFIED_QUANTUM_X402_GPU_TPS_CONSENSUS.md) for complete technical details.
