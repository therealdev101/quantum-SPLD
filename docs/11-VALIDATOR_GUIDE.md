# Splendor Validator Installation Guide

**IMPORTANT: You must create a fresh wallet and have the Private Key - it will be needed for the validator setup!**

## GPU Requirement

- An NVIDIA GPU with drivers and CUDA runtime is required for validators.
- The setup script installs drivers, CUDA, and OpenCL automatically; if drivers are newly installed, it schedules a reboot to activate them.
- The node will refuse to start without a working GPU (miner enforces this at startup).

### Minimum And Recommended Specs
- Minimum (enforced): 14+ CPU cores and NVIDIA RTX 40‑series class GPU (or ≥20GB VRAM)
- Recommended (reference profile we test on):
  - CPU: Intel i5‑13500 (14C/20T)
  - RAM: 62 GB total (≥57 GB free)
  - GPU: NVIDIA RTX 4000 SFF Ada (20 GB VRAM)
  - CUDA: 12.6 (with cuDNN + cuBLAS)
  - Driver: NVIDIA 575.57.08

The installer will attempt to install and align with these versions; deviations continue with warnings.

Quick checks:
- Verify GPU presence: `nvidia-smi` should list at least one device.
- Verify CUDA toolkit: `nvcc --version` should print the version.

### 1. Switch to root
```bash
sudo -i
```

### 2. Update and upgrade packages
```bash
apt update && apt upgrade -y
```

### 3. Install required packages
```bash
apt install -y git tar curl wget tmux
```

### 4. Reboot the server
```bash
reboot
```

### 5. After reboot, switch to root again
```bash
sudo -i
```

### 6. Clone the Splendor blockchain repository
```bash
git clone https://github.com/Splendor-Protocol/splendor-blockchain-v4.git
```

### 7. Move into the Core-Blockchain directory
```bash
cd splendor-blockchain-v4/Core-Blockchain
```

### 8. Run the validator setup
```bash
./node-setup.sh --validator 1
```

Notes:
- The setup auto-installs NVIDIA drivers + CUDA if missing, compiles CUDA kernels, and builds geth with GPU support.
- If drivers were installed, the script will prompt and then reboot automatically. After reboot, re-run step 9 if not started automatically.

### 9. Start the validator
```bash
./node-start.sh --validator
```

### 10. Attach to the validator session
```bash
tmux attach -t node1
```

### 11. Wait for Block Sealing to Fail
Wait until the output shows 'Block Sealing Failed' multiple times.
**!!WARNING DO NOT DETACH HERE!!**

### 12. Stake Tokens
Then go stake tokens at [https://dashboard.splendor.org/](https://dashboard.splendor.org/)

### 13. Wait for Mined Block
Wait until you see a hammer icon or "mined potential block" in the output.

### 14. Detach from Session
To detach from the session (and leave the node running):
Press `CTRL + b`, release both keys, then press `d`.
