# Splendor Blockchain Documentation

## Quick Start
- [Getting Started](GETTING_STARTED.md) - Installation and basic setup
- [Deployment Checklist](DEPLOYMENT_CHECKLIST.md) - Production deployment guide

## Core Features
- [AI-GPU Acceleration Guide](AI_GPU_ACCELERATION_GUIDE.md) - Complete GPU acceleration system
- [Validator Guide](VALIDATOR_GUIDE.md) - Running validator nodes
- [Smart Contracts](SMART_CONTRACTS.md) - Smart contract development

## Technical Reference
- [API Reference](API_REFERENCE.md) - RPC API documentation
- [RPC Setup Guide](RPC_SETUP_GUIDE.md) - RPC node configuration
- [Security](SECURITY.md) - Security considerations
- [Troubleshooting](TROUBLESHOOTING.md) - Common issues and solutions

## User Guides
- [MetaMask Setup](METAMASK_SETUP.md) - Wallet configuration

## Development
- [Contributing](CONTRIBUTING.md) - How to contribute
- [Code of Conduct](CODE_OF_CONDUCT.md) - Community guidelines

## Hardware Specifications

### Current System (RTX 4000 SFF Ada)
- **GPU**: NVIDIA RTX 4000 SFF Ada (20GB VRAM)
- **Target TPS**: 1.2M with AI optimization
- **Power Consumption**: 70W
- **Efficiency**: 17,143 TPS/Watt

### Production Scaling Path
- **A40 (48GB)**: 12.5M TPS - Minimum production
- **A100 80GB**: 47M TPS - Enterprise scale
- **H100 80GB**: 95M TPS - Hyperscale

## Architecture Overview

Splendor blockchain features:
- **AI-Powered Load Balancing**: vLLM + Phi-3 Mini (3.8B) for real-time optimization
- **GPU Acceleration**: CUDA/OpenCL support for massive parallel processing
- **Hybrid Processing**: Intelligent CPU/GPU workload distribution
- **Fixed Block Time**: 1-second blocks with 500B gas limits
- **Congress Consensus**: Enhanced Proof-of-Stake-Authority

## Performance Metrics

| Component | Current Performance | Target |
|-----------|-------------------|---------|
| **TPS** | 1.2M (RTX 4000 SFF) | 8M (configured) |
| **Block Time** | 1 second | Fixed |
| **Gas Limit** | 500B | Fixed |
| **Latency** | <50ms | <30ms target |
| **AI Decisions** | 2/second | 500ms intervals |

## Quick Configuration

```bash
# Current RTX 4000 SFF Ada Configuration
ENABLE_GPU=true
THROUGHPUT_TARGET=8000000
GPU_MAX_BATCH_SIZE=200000
GPU_MAX_MEMORY_USAGE=18GB
GPU_HASH_WORKERS=50
```

For detailed setup instructions, see [AI-GPU Acceleration Guide](AI_GPU_ACCELERATION_GUIDE.md).
