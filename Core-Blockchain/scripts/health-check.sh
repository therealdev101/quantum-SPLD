#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

echo -e "${PURPLE}🔍 Splendor AI-Powered GPU Blockchain Health Check${NC}"
echo -e "${CYAN}Verifying all deployment checklist items...${NC}\n"

# Initialize counters
PASSED=0
FAILED=0
WARNINGS=0

# Function to check and report status
check_item() {
    local description="$1"
    local command="$2"
    local expected="$3"
    
    echo -e "${CYAN}Checking: $description${NC}"
    
    if eval "$command" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ PASS: $description${NC}"
        ((PASSED++))
    else
        echo -e "${RED}❌ FAIL: $description${NC}"
        ((FAILED++))
    fi
}

check_warning() {
    local description="$1"
    local command="$2"
    
    echo -e "${CYAN}Checking: $description${NC}"
    
    if eval "$command" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ PASS: $description${NC}"
        ((PASSED++))
    else
        echo -e "${ORANGE}⚠️  WARN: $description${NC}"
        ((WARNINGS++))
    fi
}

echo -e "${PURPLE}=== 1. BINARY & CODE VERIFICATION ===${NC}"

check_item "Geth binary exists" "test -f ./node_src/build/bin/geth"
check_item "GPU libraries exist" "test -f ./node_src/common/gpu/gpu_processor.go"
check_item "CUDA kernels exist" "test -f ./node_src/common/gpu/native/cuda_kernels.cu"
check_item "OpenCL kernels exist" "test -f ./node_src/common/gpu/native/opencl_kernels.c"

echo -e "\n${PURPLE}=== 2. HARDWARE REQUIREMENTS ===${NC}"

# CPU Check (Intel i5-13500: 6P+8E cores = 14 cores, 20 threads)
CPU_CORES=$(nproc)
if [ "$CPU_CORES" -ge 14 ]; then
    echo -e "${GREEN}✅ PASS: CPU cores ($CPU_CORES >= 14)${NC}"
    ((PASSED++))
else
    echo -e "${RED}❌ FAIL: CPU cores ($CPU_CORES < 14)${NC}"
    ((FAILED++))
fi

# RAM Check (64GB DDR4)
RAM_GB=$(free -g | grep "Mem:" | awk '{print $2}')
if [ "$RAM_GB" -ge 60 ]; then
    echo -e "${GREEN}✅ PASS: RAM ($RAM_GB GB >= 60GB)${NC}"
    ((PASSED++))
else
    echo -e "${RED}❌ FAIL: RAM ($RAM_GB GB < 60GB)${NC}"
    ((FAILED++))
fi

# GPU Check (NVIDIA RTX 4000 SFF Ada 20GB)
if command -v nvidia-smi &> /dev/null; then
    GPU_NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader,nounits)
    GPU_MEMORY=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits)
    
    if [[ "$GPU_NAME" == *"RTX 4000"* ]] || [[ "$GPU_NAME" == *"A40"* ]] || [[ "$GPU_NAME" == *"A100"* ]] || [[ "$GPU_NAME" == *"4090"* ]]; then
        echo -e "${GREEN}✅ PASS: GPU ($GPU_NAME with ${GPU_MEMORY}MB)${NC}"
        ((PASSED++))
    else
        echo -e "${ORANGE}⚠️  WARN: GPU ($GPU_NAME) - RTX 4000/A40/A100/4090 recommended${NC}"
        ((WARNINGS++))
    fi
else
    echo -e "${RED}❌ FAIL: No NVIDIA GPU detected${NC}"
    ((FAILED++))
fi

# Disk Check
check_warning "NVMe SSD detected" "lsblk | grep nvme"

echo -e "\n${PURPLE}=== 3. CONFIGURATION VERIFICATION ===${NC}"

# Genesis file
check_item "Genesis file exists" "test -f ./genesis.json"
check_item "500B gas limit in genesis" "grep '0x746A528800' ./genesis.json"
check_item "Chain ID 6546" "grep '\"chainId\": 6546' ./genesis.json"

# Environment configuration
check_item ".env file exists" "test -f ./.env"
check_item "GPU enabled in config" "grep 'ENABLE_GPU=true' ./.env"
check_item "500B gas limit in env" "grep 'GAS_LIMIT=500000000000' ./.env"
check_item "GPU max batch size (>=150K)" "bash -c 'v=$(grep -E "^GPU_MAX_BATCH_SIZE=" ./.env | tail -n1 | cut -d= -f2); [ -n "$v" ] && [ "$v" -ge 150000 ]'"
check_warning "GPU max memory usage set (>=10GB)" "bash -c 'v=$(grep -E "^GPU_MAX_MEMORY_USAGE=" ./.env | tail -n1 | cut -d= -f2); [ -n "$v" ] && [ "$v" -ge 10737418240 ]'"
check_item "Throughput target (>=3M TPS)" "bash -c 'v=$(grep -E "^THROUGHPUT_TARGET=" ./.env | tail -n1 | cut -d= -f2); [ -n "$v" ] && [ "$v" -ge 3000000 ]'"

echo -e "\n${PURPLE}=== 4. GPU ACCELERATION ===${NC}"

check_item "CUDA toolkit installed" "command -v nvcc"
check_item "GPU Makefile exists" "test -f ./node_src/Makefile.gpu"
check_warning "GPU libraries built" "test -f ./node_src/common/gpu/libcuda_kernels.so || test -f ./node_src/common/gpu/libopencl_kernels.so"

echo -e "\n${PURPLE}=== 5. AI SYSTEM (vLLM + MobileLLM-R1) ===${NC}"

check_item "vLLM setup script exists" "test -f ./scripts/setup-ai-llm.sh"
check_item "AI load balancer code exists" "test -f ./node_src/common/ai/ai_load_balancer.go"
check_warning "vLLM service running" "systemctl is-active --quiet vllm-mobilellm"
check_warning "vLLM API accessible" "curl -s http://localhost:8000/v1/models"
check_item "AI config in .env" "grep 'ENABLE_AI_LOAD_BALANCING=true' ./.env"

echo -e "\n${PURPLE}=== 6. MONITORING & MANAGEMENT ===${NC}"

check_item "AI monitor script exists" "test -f ./scripts/ai-monitor.sh"
check_item "Performance dashboard exists" "test -f ./scripts/performance-dashboard.sh"
check_item "AI startup script exists" "test -f ./scripts/start-ai-blockchain.sh"
check_item "Tmux available" "command -v tmux"

echo -e "\n${PURPLE}=== 7. NETWORK & SERVICES ===${NC}"

check_warning "Blockchain RPC responding" "curl -s -X POST http://localhost:8545 -H 'Content-Type: application/json' --data '{\"jsonrpc\":\"2.0\",\"method\":\"net_version\",\"params\":[],\"id\":1}'"
check_warning "Sync helper running" "pm2 list | grep -q sync-helper"

echo -e "\n${PURPLE}=== 8. SYSTEM OPTIMIZATION ===${NC}"

# Check system limits
NOFILE_LIMIT=$(ulimit -n)
if [ "$NOFILE_LIMIT" -ge 1048576 ]; then
    echo -e "${GREEN}✅ PASS: File descriptor limit ($NOFILE_LIMIT >= 1048576)${NC}"
    ((PASSED++))
else
    echo -e "${ORANGE}⚠️  WARN: File descriptor limit ($NOFILE_LIMIT < 1048576)${NC}"
    ((WARNINGS++))
fi

# Check kernel parameters
check_warning "Network buffer optimization" "sysctl net.core.rmem_max | grep -q 134217728"
check_warning "Memory map optimization" "sysctl vm.max_map_count | grep -q 262144"

echo -e "\n${PURPLE}=== SUMMARY ===${NC}"
echo -e "${GREEN}✅ Passed: $PASSED${NC}"
echo -e "${ORANGE}⚠️  Warnings: $WARNINGS${NC}"
echo -e "${RED}❌ Failed: $FAILED${NC}"

TOTAL=$((PASSED + WARNINGS + FAILED))
SUCCESS_RATE=$((PASSED * 100 / TOTAL))

echo -e "\n${CYAN}Success Rate: $SUCCESS_RATE%${NC}"

if [ "$FAILED" -eq 0 ] && [ "$SUCCESS_RATE" -ge 80 ]; then
    echo -e "\n${GREEN}🎉 SYSTEM READY FOR PRODUCTION DEPLOYMENT!${NC}"
    echo -e "${CYAN}Your AI-powered GPU blockchain is configured correctly.${NC}"
    echo -e "${CYAN}Expected performance: 3M+ TPS with NVIDIA RTX 4000 SFF Ada${NC}"
elif [ "$FAILED" -eq 0 ]; then
    echo -e "\n${ORANGE}⚠️  SYSTEM MOSTLY READY - Address warnings for optimal performance${NC}"
else
    echo -e "\n${RED}❌ SYSTEM NOT READY - Fix failed items before deployment${NC}"
fi

echo -e "\n${PURPLE}=== QUICK COMMANDS ===${NC}"
echo -e "${CYAN}Start AI blockchain: ${ORANGE}./scripts/start-ai-blockchain.sh --validator${NC}"
echo -e "${CYAN}Monitor performance: ${ORANGE}./scripts/performance-dashboard.sh${NC}"
echo -e "${CYAN}View AI decisions: ${ORANGE}./scripts/ai-monitor.sh${NC}"
echo -e "${CYAN}Check GPU status: ${ORANGE}nvidia-smi${NC}"
echo -e "${CYAN}Check vLLM status: ${ORANGE}sudo systemctl status vllm-mobilellm${NC}"

echo -e "\n${PURPLE}=== DETAILED SYSTEM INFO ===${NC}"
echo -e "${CYAN}CPU: $(nproc) cores${NC}"
echo -e "${CYAN}RAM: $(free -h | grep Mem | awk '{print $2}')${NC}"
if command -v nvidia-smi &> /dev/null; then
    echo -e "${CYAN}GPU: $(nvidia-smi --query-gpu=name,memory.total --format=csv,noheader)${NC}"
fi
echo -e "${CYAN}Disk: $(df -h / | tail -1 | awk '{print $4}') available${NC}"

if [ -f ./.env ]; then
    echo -e "${CYAN}Target TPS: $(grep THROUGHPUT_TARGET .env | cut -d'=' -f2)${NC}"
    echo -e "${CYAN}GPU Batch Size: $(grep GPU_MAX_BATCH_SIZE .env | cut -d'=' -f2)${NC}"
    echo -e "${CYAN}Gas Limit: $(grep GAS_LIMIT .env | cut -d'=' -f2)${NC}"
fi

exit $FAILED
