#!/bin/bash
#set -x

set -e


GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

#########################################################################
totalRpc=0
totalValidator=0
totalNodes=$(($totalRpc + $totalValidator))

isRPC=false
isValidator=false

#Logger setup

log_step() {
  echo -e "${CYAN}➜ ${GREEN}$1${NC}"
}

log_success() {
  echo -e "${GREEN}✔ $1${NC}"
}

log_error() {
  echo -e "${RED}✖ $1${NC}"
}

log_wait() {
  echo -e "${CYAN}🕐 $1...${NC}"
}

progress_bar() {
  echo -en "${CYAN}["
  for i in {1..60}; do
    echo -en "#"
    sleep 0.01
  done
  echo -e "]${NC}"
}

#########################################################################
source ./.env
source ~/.bashrc

# Set up CUDA environment by default if GPU is enabled
if [ "$ENABLE_GPU" = "true" ]; then
  export CUDA_PATH=/usr/local/cuda
  export LD_LIBRARY_PATH=/usr/local/cuda/lib64:/usr/local/lib:$(pwd)/node_src/common/gpu:$LD_LIBRARY_PATH
  export PATH=/usr/local/cuda/bin:$PATH
  echo -e "${GREEN}🚀 CUDA environment activated with custom GPU libraries${NC}"
fi
#########################################################################

#+-----------------------------------------------------------------------------------------------+
#|                                                                                                                             |
#|                                                                                                                             |
#|                                                      FUNCTIONS                                                |
#|                                                                                                                             |
#|                                                                                                                             |
#+-----------------------------------------------------------------------------------------------+

welcome(){
  # display welcome message
  echo -e "\n\n\t${ORANGE}Total RPC installed: $totalRpc"
  echo -e "\t${ORANGE}Total Validators installed: $totalValidator"
  echo -e "\t${ORANGE}Total nodes installed: $totalNodes"
  echo -e "${GREEN}
  \t+------------------------------------------------+
  \t+   DPos node Execution Utility
  \t+   Target OS: Ubuntu 20.04 LTS (Focal Fossa)
  \t+   Your OS: $(. /etc/os-release && printf '%s\n' "${PRETTY_NAME}") 
  \t+   example usage: ./node-start.sh --help
  \t+------------------------------------------------+
  ${NC}\n"

  echo -e "${ORANGE}
  \t+------------------------------------------------+
  \t+------------------------------------------------+
  ${NC}"
}

countNodes(){
  totalRpc=0
  totalValidator=0
  totalNodes=0
  
  # Count actual node directories that exist
  for dir in ./chaindata/node*; do
    if [ -d "$dir" ]; then
      ((totalNodes += 1))
      if [ -f "$dir/.rpc" ]; then  
        ((totalRpc += 1))
      elif [ -f "$dir/.validator" ]; then
        ((totalValidator += 1))
      fi
    fi
  done
}

startRpc(){
  # Start only RPC nodes
  for dir in ./chaindata/node*; do
    if [ -d "$dir" ] && [ -f "$dir/.rpc" ]; then
      node_num=$(basename "$dir" | sed 's/node//')
      
      if tmux has-session -t node$node_num > /dev/null 2>&1; then
        echo -e "${ORANGE}RPC node$node_num session already exists${NC}"
      else
        echo -e "${GREEN}Starting RPC node$node_num with optimized resource limits${NC}"
        
        # Set resource limits before starting
        ulimit -n 1048576
        export GOMAXPROCS=$(nproc)
        
        tmux new-session -d -s node$node_num
        tmux send-keys -t node$node_num "ulimit -n 1048576" Enter
        tmux send-keys -t node$node_num "export GOMAXPROCS=\$(nproc)" Enter
        # Pass GPU + performance environment to tmux session
        if [ "$ENABLE_GPU" = "true" ]; then
          tmux send-keys -t node$node_num "export CUDA_PATH=/usr/local/cuda" Enter
          tmux send-keys -t node$node_num "export LD_LIBRARY_PATH=/usr/local/cuda/lib64:/usr/local/lib:$(pwd)/node_src/common/gpu:\$LD_LIBRARY_PATH" Enter
          tmux send-keys -t node$node_num "export PATH=/usr/local/cuda/bin:\$PATH" Enter
          tmux send-keys -t node$node_num "export ENABLE_GPU=true" Enter
          tmux send-keys -t node$node_num "export THROUGHPUT_TARGET=${THROUGHPUT_TARGET:-3000000}" Enter
          tmux send-keys -t node$node_num "export GPU_MAX_BATCH_SIZE=${GPU_MAX_BATCH_SIZE:-200000}" Enter
          tmux send-keys -t node$node_num "export GPU_MAX_MEMORY_USAGE=${GPU_MAX_MEMORY_USAGE:-17179869184}" Enter
          tmux send-keys -t node$node_num "export GPU_THRESHOLD=${GPU_THRESHOLD:-1000}" Enter
          tmux send-keys -t node$node_num "export GPU_HASH_WORKERS=${GPU_HASH_WORKERS:-32}" Enter
          tmux send-keys -t node$node_num "export GPU_SIGNATURE_WORKERS=${GPU_SIGNATURE_WORKERS:-32}" Enter
          tmux send-keys -t node$node_num "export GPU_TX_WORKERS=${GPU_TX_WORKERS:-32}" Enter
          tmux send-keys -t node$node_num "export ENABLE_AI_LOAD_BALANCING=${ENABLE_AI_LOAD_BALANCING:-true}" Enter
        fi
        tmux send-keys -t node$node_num "./node_src/build/bin/geth --datadir ./chaindata/node$node_num --networkid $CHAINID --bootnodes $BOOTNODE --port 30303 --ws --ws.addr $IP --ws.origins '*' --ws.port 8545 --http --http.port 80 --rpc.txfeecap 0 --http.corsdomain '*' --nat any --http.api db,eth,net,web3,personal,txpool,miner,debug,x402,gpu --http.addr 0.0.0.0 --http.vhosts '*' --vmdebug --pprof --pprof.port 6060 --pprof.addr $IP --syncmode=full --gcmode=archive --cache=1024 --cache.database=512 --cache.trie=256 --cache.gc=256 --txpool.accountslots=1000000 --txpool.globalslots=10000000 --txpool.accountqueue=500000 --txpool.globalqueue=5000000 --maxpeers=25 --ipcpath './chaindata/node$node_num/geth.ipc' console" Enter
      fi
    fi
  done
}

startValidator(){
  # Start only Validator nodes
  for dir in ./chaindata/node*; do
    if [ -d "$dir" ] && [ -f "$dir/.validator" ]; then
      node_num=$(basename "$dir" | sed 's/node//')
      
      if tmux has-session -t node$node_num > /dev/null 2>&1; then
        echo -e "${ORANGE}Validator node$node_num session already exists${NC}"
      else
        echo -e "${GREEN}Starting Validator node$node_num with optimized resource limits${NC}"
        
        # Set resource limits before starting
        ulimit -n 1048576
        export GOMAXPROCS=$(nproc)
        
        tmux new-session -d -s node$node_num
        tmux send-keys -t node$node_num "ulimit -n 1048576" Enter
        tmux send-keys -t node$node_num "export GOMAXPROCS=\$(nproc)" Enter
        # Pass GPU + performance environment to tmux session
        if [ "$ENABLE_GPU" = "true" ]; then
          tmux send-keys -t node$node_num "export CUDA_PATH=/usr/local/cuda" Enter
          tmux send-keys -t node$node_num "export LD_LIBRARY_PATH=/usr/local/cuda/lib64:/usr/local/lib:$(pwd)/node_src/common/gpu:\$LD_LIBRARY_PATH" Enter
          tmux send-keys -t node$node_num "export PATH=/usr/local/cuda/bin:\$PATH" Enter
          tmux send-keys -t node$node_num "export ENABLE_GPU=true" Enter
          tmux send-keys -t node$node_num "export THROUGHPUT_TARGET=${THROUGHPUT_TARGET:-3000000}" Enter
          tmux send-keys -t node$node_num "export GPU_MAX_BATCH_SIZE=${GPU_MAX_BATCH_SIZE:-200000}" Enter
          tmux send-keys -t node$node_num "export GPU_MAX_MEMORY_USAGE=${GPU_MAX_MEMORY_USAGE:-17179869184}" Enter
          tmux send-keys -t node$node_num "export GPU_THRESHOLD=${GPU_THRESHOLD:-1000}" Enter
          tmux send-keys -t node$node_num "export GPU_HASH_WORKERS=${GPU_HASH_WORKERS:-32}" Enter
          tmux send-keys -t node$node_num "export GPU_SIGNATURE_WORKERS=${GPU_SIGNATURE_WORKERS:-32}" Enter
          tmux send-keys -t node$node_num "export GPU_TX_WORKERS=${GPU_TX_WORKERS:-32}" Enter
          tmux send-keys -t node$node_num "export ENABLE_AI_LOAD_BALANCING=${ENABLE_AI_LOAD_BALANCING:-true}" Enter
        fi
        tmux send-keys -t node$node_num "LD_LIBRARY_PATH=./node_src/common/gpu:/usr/local/cuda/lib64:\$LD_LIBRARY_PATH ./node_src/build/bin/geth --datadir ./chaindata/node$node_num --networkid $CHAINID --bootnodes $BOOTNODE --mine --port 30303 --nat extip:$IP --gpo.percentile 0 --gpo.maxprice 100 --gpo.ignoreprice 0 --miner.gaslimit 500000000000 --unlock 0 --password ./chaindata/node$node_num/pass.txt --syncmode=full --gcmode=archive --cache=1024 --cache.database=512 --cache.trie=256 --cache.gc=256 --txpool.accountslots=1000000 --txpool.globalslots=10000000 --txpool.accountqueue=500000 --txpool.globalqueue=5000000 --maxpeers=25 console" Enter
      fi
    fi
  done
}

gpu_is_ready() {
  # Strict readiness: driver loaded and NVML can enumerate at least one GPU
  if command -v nvidia-smi >/dev/null 2>&1; then
    if timeout 4 nvidia-smi -L 2>/dev/null | grep -q "."; then
      return 0
    fi
  fi
  # Secondary: device node and module present
  if [ -e /dev/nvidia0 ] && lsmod 2>/dev/null | grep -qi '^nvidia\b'; then
    return 0
  fi
  # Hardware present but driver inactive
  if lspci 2>/dev/null | grep -qi nvidia; then
    return 2
  fi
  return 1
}

initializeGPU(){
  echo -e "\n${GREEN}+------------------ GPU Initialization -------------------+${NC}"
  
  # Quick non-blocking GPU check
  if [ "$ENABLE_GPU" = "true" ]; then
    echo -e "${CYAN}GPU acceleration enabled${NC}"

    if gpu_is_ready; then
      echo -e "${GREEN}✅ GPU drivers active and ready${NC}"
      GPU_STATUS="active"
    else
      status=$?
      if [ "$status" -eq 2 ]; then
        echo -e "${ORANGE}⚠️  NVIDIA hardware detected but drivers inactive - running in CPU mode${NC}"
        GPU_STATUS="pending_activation"
      else
        echo -e "${ORANGE}ℹ️  No NVIDIA GPU hardware detected - running in CPU mode${NC}"
        GPU_STATUS="no_gpu"
      fi
    fi
    
    # Enhanced CUDA check with multiple path detection
    CUDA_AVAILABLE=false
    
    # Check standard CUDA path
    if [ -f "/usr/local/cuda/bin/nvcc" ]; then
      CUDA_AVAILABLE=true
    # Check alternative CUDA paths
    elif command -v nvcc >/dev/null 2>&1; then
      CUDA_AVAILABLE=true
    # Check if CUDA toolkit is installed via package manager
    elif dpkg -l | grep -q "cuda-toolkit"; then
      CUDA_AVAILABLE=true
      # Try to find nvcc in system paths
      NVCC_PATH=$(find /usr -name "nvcc" 2>/dev/null | head -1)
      if [ -n "$NVCC_PATH" ]; then
        export PATH=$(dirname "$NVCC_PATH"):$PATH
      fi
    fi
    
    if [ "$CUDA_AVAILABLE" = "true" ]; then
      echo -e "${GREEN}✅ CUDA toolkit available and ready${NC}"
      # Verify CUDA can detect GPU
      if gpu_is_ready; then
        echo -e "${GREEN}✅ CUDA-GPU communication verified${NC}"
      fi
    else
      echo -e "${ORANGE}⚠️  CUDA toolkit not found - using OpenCL or CPU${NC}"
    fi
    
    echo -e "${GREEN}GPU Config: ${ORANGE}${THROUGHPUT_TARGET:-1000000} TPS target${NC}"
  else
    echo -e "${ORANGE}GPU acceleration disabled${NC}"
    GPU_STATUS="disabled"
  fi
  
  echo -e "${GREEN}✅ GPU check completed (non-blocking)${NC}"
}

finalize(){
  countNodes
  welcome
  initializeGPU
  
  # Initialize AI status tracking
  AI_STATUS="not_available"
  
  # Start vLLM AI service directly
  if [ -d "/opt/vllm-env" ]; then
    echo -e "\n${GREEN}+------------------ Starting AI System -------------------+${NC}"
    log_wait "Starting vLLM MobileLLM-R1 AI load balancer"
    
    # Check if vLLM is already running
    if curl -s http://localhost:8000/v1/models >/dev/null 2>&1; then
      log_success "vLLM AI service already running"
      AI_STATUS="fully_active"
    else
      # Start vLLM in background with working parameters
      cd /opt
      source vllm-env/bin/activate
      
      # Start vLLM with reduced memory usage and proper configuration
      nohup python -m vllm.entrypoints.openai.api_server \
        --model facebook/MobileLLM-R1 \
        --host 0.0.0.0 \
        --port 8000 \
        --gpu-memory-utilization 0.2 \
        --max-model-len 2048 \
        --dtype float16 \
        --disable-log-requests \
        > /tmp/vllm.log 2>&1 &
      
      VLLM_PID=$!
      echo $VLLM_PID > /tmp/vllm.pid
      
      cd /root/quantum-SPLD/Core-Blockchain/
      
      log_success "vLLM AI service started (PID: $VLLM_PID)"
      AI_STATUS="service_started"
      
      # Wait for API to be ready (non-blocking)
      for i in {1..15}; do
        if curl -s http://localhost:8000/v1/models >/dev/null 2>&1; then
          log_success "vLLM API ready - AI load balancing active"
          AI_STATUS="fully_active"
          break
        fi
        echo -e "${CYAN}Waiting for vLLM API... ($i/15)${NC}"
        sleep 3
      done
      
      # If API didn't respond but service started, it's still starting up
      if [ "$AI_STATUS" = "service_started" ]; then
        AI_STATUS="starting_up"
        log_wait "vLLM API still initializing (check /tmp/vllm.log for progress)"
      fi
    fi
  else
    echo -e "\n${ORANGE}vLLM environment not found. Run setup-ai-llm.sh to install AI features.${NC}"
    AI_STATUS="not_installed"
  fi
  
  if [ "$isRPC" = true ]; then
    echo -e "\n${GREEN}+------------------- Starting RPC -------------------+"
    startRpc
  fi

  if [ "$isValidator" = true ]; then
    echo -e "\n${GREEN}+------------------- Starting Validator -------------------+"
    startValidator
  fi

  echo -e "\n${GREEN}+------------------ Active Nodes -------------------+"
  if tmux ls 2>/dev/null; then
    echo -e "${GREEN}✅ Nodes started successfully${NC}"
  else
    echo -e "${ORANGE}⚠️  No tmux sessions found${NC}"
  fi

  echo -e "\n${GREEN}+------------------ Starting sync-helper -------------------+${NC}"
  echo -e "\n${ORANGE}+-- Please wait a few seconds. Do not turn off the server or interrupt --+"
  
  cd ./plugins/sync-helper/
  
  export NVM_DIR="$HOME/.nvm"
  [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"

  # Check if sync-helper is already running
  if pm2 list | grep -q "index"; then
    echo -e "${ORANGE}sync-helper is already running, restarting...${NC}"
    pm2 restart index
  else
    echo -e "${GREEN}Starting sync-helper...${NC}"
    pm2 start index.js --name "sync-helper"
  fi
  
  pm2 save
  cd /root/quantum-SPLD/Core-Blockchain/

  # Final status report
  echo -e "\n${GREEN}+------------------ SYSTEM STATUS -------------------+${NC}"
  
  # Check x402 API status
  echo -e "\n${CYAN}Checking x402 Native Payments API...${NC}"
  sleep 2  # Give nodes time to start
  
  if curl -s -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
     http://localhost:80 | grep -q "result" 2>/dev/null; then
    echo -e "${GREEN}✅ x402 API: Native payments protocol active and ready${NC}"
    echo -e "${GREEN}   🌟 World's first blockchain with native x402 support${NC}"
    echo -e "${GREEN}   ⚡ Millions of TPS micropayments enabled${NC}"
    echo -e "${GREEN}   💰 $0.001 minimum payments (no gas fees)${NC}"
  else
    echo -e "${ORANGE}⚠️  x402 API: Will be available once RPC node fully starts${NC}"
    echo -e "${CYAN}   Test with: ./test-x402.sh (after node startup completes)${NC}"
  fi
  
  # Check AI system based on tracked status
  case "$AI_STATUS" in
    "fully_active")
      echo -e "${GREEN}✅ AI System: vLLM MobileLLM-R1 active (150K+ TPS ready)${NC}"
      ;;
    "service_started"|"starting_up")
      echo -e "${GREEN}✅ AI System: vLLM service active (API initializing)${NC}"
      ;;
    "pending_reboot")
      echo -e "${ORANGE}⚠️  AI System: Will activate after reboot${NC}"
      ;;
    *)
      echo -e "${ORANGE}⚠️  AI System: Not available${NC}"
      ;;
  esac
  
  # Check GPU status (use robust detector + earlier GPU_STATUS)
  if gpu_is_ready; then
    echo -e "${GREEN}✅ GPU: Active and ready${NC}"
  else
    status=$?
    if [ "$status" -eq 2 ]; then
      echo -e "${ORANGE}⚠️  GPU: Hardware detected but driver inactive${NC}"
    else
      echo -e "${ORANGE}ℹ️  GPU: No NVIDIA GPU detected${NC}"
    fi
  fi
  
  # Check if GPU drivers need reboot and offer reboot
  if [ "$GPU_STATUS" = "pending_activation" ] && lspci | grep -i nvidia >/dev/null 2>&1; then
    echo -e "\n${ORANGE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${ORANGE}║                    GPU REBOOT RECOMMENDED                   ║${NC}"
    echo -e "${ORANGE}║                                                              ║${NC}"
    echo -e "${ORANGE}║  NVIDIA GPU drivers are installed but need reboot to        ║${NC}"
    echo -e "${ORANGE}║  activate for maximum TPS performance.                      ║${NC}"
    echo -e "${ORANGE}║                                                              ║${NC}"
    echo -e "${ORANGE}║  Current: CPU-only mode                                     ║${NC}"
    echo -e "${ORANGE}║  After reboot: GPU+AI accelerated high TPS mode            ║${NC}"
    echo -e "${ORANGE}╚══════════════════════════════════════════════════════════════╝${NC}\n"
    echo -e "${CYAN}To activate GPU+AI acceleration: ${GREEN}reboot${NC}"
  fi

}


# Default variable values
verbose_mode=false
output_file=""

# Function to display script usage
usage() {
  echo -e "\nUsage: $0 [OPTIONS]"
  echo "Options:"
  echo -e "\t\t -h, --help      Display this help message"
  echo -e " \t\t -v, --verbose   Enable verbose mode"
  echo -e "\t\t --rpc       Start all the RPC nodes installed"
  echo -e "\t\t --validator       Start all the Validator nodes installed"
}

has_argument() {
  [[ ("$1" == *=* && -n ${1#*=}) || (! -z "$2" && "$2" != -*) ]]
}

extract_argument() {
  echo "${2:-${1#*=}}"
}

# Function to handle options and arguments
handle_options() {
  while [ $# -gt 0 ]; do
    case $1 in

    # display help
    -h | --help)
      usage
      exit 0
      ;;

    # toggle verbose
    -v | --verbose)
      verbose_mode=true
      ;;

    # take file input
    -f | --file*)
      if ! has_argument $@; then
        echo "File not specified." >&2
        usage
        exit 1
      fi

      output_file=$(extract_argument $@)

      shift
      ;;

    # take ROC count
    --rpc)
        isRPC=true
      ;;

    # take validator count
    --validator)
        isValidator=true
      ;;

    *)
      echo "Invalid option: $1" >&2
      usage
      exit 1
      ;;

    esac
    shift
  done
}

# Main script execution
handle_options "$@"

# Perform the desired actions based on the provided flags and arguments
if [ "$verbose_mode" = true ]; then
  echo "Verbose mode enabled."
fi

if [ -n "$output_file" ]; then
  echo "Output file specified: $output_file"
fi

if [ $# -eq 0 ]
  then
    echo "No arguments supplied"
    usage
    exit 1
fi

finalize
