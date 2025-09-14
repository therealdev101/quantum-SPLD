# ✅ Splendor AI-Powered GPU Blockchain Deployment Checklist

## Overview
This checklist ensures all validator and RPC nodes run identically with AI-powered GPU acceleration, capable of handling 500B gas limits and 10M+ TPS.

---

## 1. **Binary & Code** ✅

### [ ] Rebuild geth from the same commit/tag across all servers
```bash
# On build server
cd Core-Blockchain/node_src
git log --oneline -1  # Record commit hash
make clean && make all
make -f Makefile.gpu clean && make -f Makefile.gpu all

# Generate checksums
sha256sum build/bin/geth > geth.sha256
sha256sum common/gpu/*.so >> gpu_libs.sha256
```

### [ ] Verify binaries have the same SHA256 checksum on every node
```bash
# Distribute and verify on each node
scp build/bin/geth user@node:/path/to/splendor/
scp geth.sha256 user@node:/path/to/splendor/
scp common/gpu/*.so user@node:/path/to/splendor/gpu/

# On each node
sha256sum -c geth.sha256
sha256sum -c gpu_libs.sha256
```

### [ ] Distribute via scp/ansible so all nodes run identical builds
```bash
# Ansible playbook for mass deployment
ansible-playbook -i inventory deploy-splendor.yml

# Or manual scp to each node
for node in validator1 validator2 rpc1 rpc2; do
    scp -r Core-Blockchain/ $node:/root/splendor-blockchain-v4/
done
```

---

## 2. **Hardware Requirements (minimum for every node)** ✅

### [ ] **CPU:** 16 physical cores (32 threads recommended)
```bash
# Verify on each node
nproc  # Should show 16+
lscpu | grep "CPU(s):"
cat /proc/cpuinfo | grep "processor" | wc -l
```

### [ ] **RAM:** 64 GB DDR4/DDR5 ECC
```bash
# Verify memory
free -h  # Should show 64GB+
dmidecode -t memory | grep "Size.*GB"
```

### [ ] **GPU:** NVIDIA A40 (48 GB VRAM) or better
```bash
# Verify GPU
nvidia-smi
nvidia-smi --query-gpu=name,memory.total --format=csv
# Should show: A40, 49140 MiB (48GB)
```

### [ ] **Disk:** 2–4 TB NVMe SSD (PCIe 4.0+, 7GB/s+)
```bash
# Verify storage
lsblk -d -o name,size,rota,type | grep nvme
# ROTA should be 0 (SSD), SIZE should be 2T+

# Test disk speed
fio --name=test --ioengine=libaio --rw=read --bs=1M --size=1G --numjobs=1 --time_based --runtime=10
# Should show 7GB/s+ read speed
```

### [ ] **Network:** 10–25 Gbps symmetric uplink
```bash
# Verify network interface
ethtool eth0 | grep Speed
# Should show: Speed: 10000Mb/s or higher

# Test bandwidth between nodes
iperf3 -s  # On one node
iperf3 -c <server_ip> -t 30  # On another node
```

---

## 3. **Node Flags (validators + RPCs identical)** ✅

### [ ] Update node-start.sh with optimized flags for all nodes
```bash
# Validator flags (500B gas + A40 optimized)
./node_src/build/bin/geth \
  --datadir ./chaindata/node$i \
  --networkid $CHAINID \
  --bootnodes $BOOTNODE \
  --mine \
  --port 30303 \
  --nat extip:$IP \
  --gpo.percentile 0 \
  --gpo.maxprice 100 \
  --gpo.ignoreprice 0 \
  --miner.gaslimit 500000000000 \
  --unlock 0 \
  --password ./chaindata/node$i/pass.txt \
  --syncmode=snap \
  --gcmode=full \
  --cache=16384 \
  --cache.database=8192 \
  --cache.trie=4096 \
  --cache.gc=4096 \
  --txpool.accountslots=10000 \
  --txpool.globalslots=2000000 \
  --txpool.accountqueue=10000 \
  --txpool.globalqueue=2000000 \
  --rpc.txfeecap=0 \
  --http --http.addr 0.0.0.0 --http.api eth,net,web3,txpool,miner,debug \
  --ws --ws.addr 0.0.0.0 \
  --nat any \
  --verbosity=3 \
  console
```

### [ ] Verify flags are identical across all nodes
```bash
# Check node-start.sh on each server
grep "miner.gaslimit" node-start.sh  # Should show 500000000000
grep "txpool.globalslots" node-start.sh  # Should show 2000000
grep "cache=" node-start.sh  # Should show 16384
```

---

## 4. **Genesis File** ✅

### [ ] Set config.chainId = 2691 (or your chain ID)
```bash
# Verify genesis.json
grep "chainId" genesis.json  # Should show 2691
```

### [ ] Increase config.gasLimit to 500B in genesis
```bash
# Verify gas limit in genesis
grep "gasLimit" genesis.json  # Should show "0x746A528800" (500B)
```

### [ ] Ensure baseFee, difficulty, and consensus params are identical
```bash
# Verify critical genesis parameters
grep -E "(baseFeePerGas|difficulty|congress)" genesis.json
```

### [ ] Distribute the same genesis.json to all nodes
```bash
# Copy genesis to all nodes
for node in validator1 validator2 rpc1 rpc2; do
    scp genesis.json $node:/root/splendor-blockchain-v4/Core-Blockchain/
done

# Verify checksums match
sha256sum genesis.json  # Record this hash
# On each node: sha256sum genesis.json (should match)
```

---

## 5. **Validator Setup** ✅

### [ ] Each validator has unique nodekey
```bash
# Generate unique nodekey for each validator
for i in {1..N}; do
    ./node_src/build/bin/geth --datadir ./chaindata/node$i account new
    # Record addresses and ensure they're unique
done
```

### [ ] Unlock validator account with --unlock + --miner.etherbase
```bash
# Verify unlock configuration in node-start.sh
grep -E "(unlock|miner.etherbase)" node-start.sh
```

### [ ] Ensure password.txt exists in correct datadir
```bash
# Verify password files exist
for i in {1..N}; do
    test -f ./chaindata/node$i/pass.txt && echo "Node $i: OK" || echo "Node $i: MISSING"
done
```

### [ ] Verify enode:// addresses are shared with all peers
```bash
# Get enode from each validator
for i in {1..N}; do
    echo "Node $i enode:"
    # Extract from logs or admin.nodeInfo
done
```

---

## 6. **RPC Nodes** ✅

### [ ] Run with same flags as validators, but without --mine
```bash
# RPC node flags (no mining)
./node_src/build/bin/geth \
  --datadir ./chaindata/node$i \
  --networkid $CHAINID \
  --bootnodes $BOOTNODE \
  --port 30303 \
  --ws --ws.addr $IP --ws.origins '*' --ws.port 8545 \
  --http --http.port 80 --rpc.txfeecap 0 \
  --http.corsdomain '*' --nat 'any' \
  --http.api db,eth,net,web3,personal,txpool,miner,debug \
  --http.addr $IP \
  --vmdebug --pprof --pprof.port 6060 --pprof.addr $IP \
  --syncmode=full --gcmode=archive \
  --cache=16384 --cache.database=8192 --cache.trie=4096 --cache.gc=4096 \
  --txpool.accountslots=10000 --txpool.globalslots=2000000 \
  --txpool.accountqueue=10000 --txpool.globalqueue=2000000 \
  console
```

### [ ] Balance RPC load with Nginx/HAProxy in front (round-robin)
```bash
# Nginx configuration for RPC load balancing
upstream splendor_rpc {
    server rpc1.domain.com:8545;
    server rpc2.domain.com:8545;
    server rpc3.domain.com:8545;
}

server {
    listen 80;
    location / {
        proxy_pass http://splendor_rpc;
    }
}
```

### [ ] Enable --gcmode=archive for full history (if you need explorers)
```bash
# Verify archive mode for RPC nodes
grep "gcmode=archive" node-start.sh
```

---

## 7. **Networking** ✅

### [ ] Open TCP/UDP 30303 for P2P
```bash
# Firewall rules for each node
sudo ufw allow 30303/tcp
sudo ufw allow 30303/udp
```

### [ ] Open 8545/8645/6060 for RPC/WS/pprof (secure behind firewall)
```bash
# RPC/WS ports (restrict to trusted IPs)
sudo ufw allow from <trusted_ip> to any port 8545
sudo ufw allow from <trusted_ip> to any port 8645
sudo ufw allow from <trusted_ip> to any port 6060
```

### [ ] Ensure each node's external IP is in --bootnodes
```bash
# Verify bootnode configuration
grep "BOOTNODE=" .env
# Should contain enode addresses of all validators
```

### [ ] Validate peer count via RPC
```bash
# Check peer connections on each node
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}'
# Should show connected peers
```

---

## 8. **GPU Acceleration Setup** ✅

### [ ] Install CUDA drivers and toolkit on all nodes
```bash
# On each node
sudo apt install nvidia-driver-470 cuda-toolkit
nvidia-smi  # Verify A40 detected
nvcc --version  # Verify CUDA toolkit
```

### [ ] Build GPU components on all nodes
```bash
# On each node
cd Core-Blockchain/node_src
make -f Makefile.gpu check-deps
make -f Makefile.gpu all
make -f Makefile.gpu test
```

### [ ] Verify GPU libraries are identical across nodes
```bash
# Generate checksums
sha256sum common/gpu/*.so > gpu_libs.sha256

# Verify on each node
sha256sum -c gpu_libs.sha256
```

---

## 9. **AI System Setup** ✅

### [ ] Install vLLM and Phi-3 Mini on all nodes
```bash
# On each node
./scripts/setup-ai-llm.sh

# Verify vLLM service
sudo systemctl status vllm-phi3
curl -s http://localhost:8000/v1/models
```

### [ ] Verify AI configuration is identical
```bash
# Check AI settings in .env
grep -A 10 "AI_LOAD_BALANCING" .env
# Should be identical on all nodes
```

### [ ] Test AI load balancer functionality
```bash
# Test AI API on each node
curl -s -X POST http://localhost:8000/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "microsoft/Phi-3-mini-4k-instruct",
    "prompt": "Test AI response",
    "max_tokens": 50
  }'
```

---

## 10. **Configuration Verification** ✅

### [ ] Verify .env settings are identical across all nodes
```bash
# Generate .env checksum
sha256sum .env > env.sha256

# Verify on each node
sha256sum -c env.sha256
```

### [ ] Check GPU configuration for A40 optimization
```bash
# Verify A40 settings
grep "GPU_MAX_BATCH_SIZE=100000" .env
grep "GPU_MAX_MEMORY_USAGE=42949672960" .env  # 40GB
grep "GPU_HASH_WORKERS=32" .env
grep "THROUGHPUT_TARGET=10000000" .env  # 10M TPS
```

### [ ] Verify 500B gas limit configuration
```bash
# Check gas limit settings
grep "GAS_LIMIT=500000000000" .env
grep "0x746A528800" genesis.json  # 500B in hex
```

---

## 11. **Monitoring Setup** ✅

### [ ] Install monitoring tools on all nodes
```bash
# Install monitoring dependencies
sudo apt install prometheus-node-exporter grafana jq htop

# Setup monitoring scripts
chmod +x scripts/ai-monitor.sh
chmod +x scripts/performance-dashboard.sh
```

### [ ] Configure alerts for critical metrics
```bash
# Alert thresholds
# - GPU temperature > 80°C
# - GPU utilization < 50% (underutilization)
# - Memory usage > 90%
# - TPS < 1M (performance degradation)
# - AI confidence < 50% (AI issues)
```

### [ ] Verify tmux session management works
```bash
# Test tmux functionality
tmux new-session -d -s test 'echo "test"'
tmux list-sessions
tmux kill-session -t test
```

---

## 12. **Network Synchronization** ✅

### [ ] Ensure all nodes start with identical genesis
```bash
# Initialize all nodes with same genesis
for i in {1..N}; do
    ./node_src/build/bin/geth --datadir ./chaindata/node$i init ./genesis.json
done
```

### [ ] Verify bootnode connectivity
```bash
# Test bootnode reachability from each node
for bootnode in $BOOTNODES; do
    # Extract IP and test connectivity
    ping -c 3 <bootnode_ip>
    telnet <bootnode_ip> 30303
done
```

### [ ] Check peer discovery and connection
```bash
# On each node after startup
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}'
```

---

## 13. **Performance Optimization** ✅

### [ ] Verify system limits and kernel parameters
```bash
# Check and set system limits
echo "* soft nofile 1048576" >> /etc/security/limits.conf
echo "* hard nofile 1048576" >> /etc/security/limits.conf
echo "root soft nofile 1048576" >> /etc/security/limits.conf
echo "root hard nofile 1048576" >> /etc/security/limits.conf

# Kernel parameters for high performance
echo "net.core.rmem_max = 134217728" >> /etc/sysctl.conf
echo "net.core.wmem_max = 134217728" >> /etc/sysctl.conf
echo "vm.max_map_count = 262144" >> /etc/sysctl.conf
sysctl -p
```

### [ ] Verify GPU memory allocation
```bash
# Check GPU memory usage
nvidia-smi --query-gpu=memory.used,memory.total --format=csv
# Should show efficient utilization (40GB/48GB for blockchain)
```

### [ ] Test transaction pool capacity
```bash
# Verify txpool settings
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"txpool_status","params":[],"id":1}'
```

---

## 14. **Stress Testing** ✅

### [ ] Fund multiple wallets (not one) to spread nonces
```bash
# Create test wallets
for i in {1..100}; do
    ./node_src/build/bin/geth --datadir ./test account new --password <(echo "test")
done

# Fund wallets from genesis account
# Distribute funds to avoid nonce bottlenecks
```

### [ ] Use Python stress tester with workers tuned to 2–3× CPU cores
```bash
# Stress test configuration
WORKERS=$(($(nproc) * 3))  # 3x CPU cores
python3 stress_test.py --workers $WORKERS --target-tps 1000000
```

### [ ] Start with 20B gas limit (baseline)
```bash
# Initial testing with 20B
sed -i 's/GAS_LIMIT=500000000000/GAS_LIMIT=20000000000/' .env
./node-start.sh --validator
# Monitor performance and stability
```

### [ ] Increment to 100B, then 500B once stable
```bash
# Gradual increase
# 20B -> 100B -> 500B
# Monitor at each step:
# - Block processing time
# - Memory usage
# - GPU utilization
# - Network stability
```

### [ ] Confirm no forks/splits under load
```bash
# Monitor for forks during stress test
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Check on multiple nodes - block numbers should match
```

---

## 15. **Recovery / Safety Nets** ✅

### [ ] Snapshot each validator's datadir after clean sync
```bash
# Create snapshots
for i in {1..N}; do
    tar -czf chaindata_node${i}_backup.tar.gz chaindata/node$i/
done

# Store snapshots in safe location
```

### [ ] Automate auto-restart with systemd
```bash
# Create systemd service for each node
sudo tee /etc/systemd/system/splendor-validator.service > /dev/null <<EOF
[Unit]
Description=Splendor Blockchain Validator
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/splendor-blockchain-v4/Core-Blockchain
ExecStart=/root/splendor-blockchain-v4/Core-Blockchain/scripts/start-ai-blockchain.sh --validator
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable splendor-validator
```

### [ ] If mempool clogs: clear transactions.rlp + restart node
```bash
# Emergency mempool clear procedure
systemctl stop splendor-validator
rm -f chaindata/node*/geth/transactions.rlp
systemctl start splendor-validator
```

### [ ] Always keep 2 standby RPCs for failover
```bash
# Setup standby RPC nodes
# Configure load balancer to detect failures
# Automatic failover to standby nodes
```

---

## 16. **AI-Specific Verification** ✅

### [ ] Verify vLLM service is running on all nodes
```bash
# Check vLLM status
sudo systemctl status vllm-phi3
curl -s http://localhost:8000/v1/models | jq .
```

### [ ] Test AI decision making
```bash
# Test AI load balancer
./scripts/ai-monitor.sh
# Should show active AI decisions every 500ms
```

### [ ] Verify AI configuration consistency
```bash
# Check AI settings
grep -A 15 "AI_LOAD_BALANCING" .env
# Should be identical on all nodes
```

---

## 17. **Final Validation** ✅

### [ ] Start all nodes and verify synchronization
```bash
# Start all nodes
for node in validator1 validator2 rpc1 rpc2; do
    ssh $node "cd /root/splendor-blockchain-v4/Core-Blockchain && ./scripts/start-ai-blockchain.sh --validator"
done

# Wait 5 minutes, then check block numbers match
```

### [ ] Run performance benchmark
```bash
# Benchmark test
python3 benchmark.py --duration 300 --target-tps 5000000
# Should achieve 5M+ TPS sustained
```

### [ ] Verify AI load balancing is active
```bash
# Check AI decisions on all nodes
for node in validator1 validator2 rpc1 rpc2; do
    ssh $node "curl -s http://localhost:8000/v1/models"
done
```

### [ ] Monitor system stability for 24 hours
```bash
# 24-hour stability test
# Monitor:
# - Block production consistency
# - Memory usage stability
# - GPU temperature and utilization
# - AI decision accuracy
# - Network connectivity
```

---

## 18. **Production Deployment Checklist** ✅

### [ ] Security hardening
```bash
# Disable unnecessary services
sudo systemctl disable apache2 nginx (if not needed)

# Configure firewall
sudo ufw enable
sudo ufw default deny incoming
sudo ufw allow 22/tcp  # SSH
sudo ufw allow 30303  # P2P
sudo ufw allow from <trusted_ips> to any port 8545  # RPC
```

### [ ] Backup and disaster recovery
```bash
# Automated backup script
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
tar -czf /backup/chaindata_$DATE.tar.gz chaindata/
# Keep last 7 days of backups
```

### [ ] Monitoring and alerting
```bash
# Setup Prometheus monitoring
# Configure Grafana dashboards
# Setup PagerDuty/Slack alerts
```

---

## Verification Commands

### Quick Health Check (run on each node):
```bash
#!/bin/bash
echo "=== Splendor Node Health Check ==="

# Hardware
echo "CPU Cores: $(nproc)"
echo "RAM: $(free -h | grep Mem | awk '{print $2}')"
echo "GPU: $(nvidia-smi --query-gpu=name,memory.total --format=csv,noheader)"

# Services
echo "Geth: $(pgrep geth > /dev/null && echo 'Running' || echo 'Stopped')"
echo "vLLM: $(systemctl is-active vllm-phi3)"

# Performance
echo "Block Number: $(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result)"
echo "Peer Count: $(curl -s -X POST http://localhost:8545 -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}' | jq -r .result)"

# AI Status
echo "AI API: $(curl -s http://localhost:8000/v1/models > /dev/null && echo 'Online' || echo 'Offline')"
```

## Success Criteria

### ✅ All items checked means:
- **Identical binaries** across all nodes
- **NVIDIA A40 (48GB VRAM)** on every node
- **500B gas limit** configured everywhere
- **vLLM + Phi-3 Mini** AI system active
- **10M+ TPS capability** with AI optimization
- **Tmux-compatible** management
- **Production-ready** monitoring and recovery

### Expected Results:
- **Sustained TPS**: 10M+ transactions per second
- **Latency**: <30ms average processing time
- **GPU Utilization**: 98% on A40 hardware
- **AI Decisions**: Every 500ms for optimal performance
- **Uptime**: 99.9%+ with automatic recovery
- **Scalability**: Linear scaling with additional A40 nodes

This checklist ensures your Splendor blockchain network is the world's first truly AI-powered blockchain with enterprise-grade performance and reliability.
