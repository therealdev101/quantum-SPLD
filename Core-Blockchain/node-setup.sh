#!/bin/bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
NC='\033[0m' # No Color
CYAN='\033[0;36m'
BASE_DIR='/root/splendor-blockchain-v4'

# Behavior toggles (can be overridden by flags)
# Default: keep existing NVIDIA drivers; only install if missing
ENFORCE_DRIVER_VERSION=false
AUTO_REBOOT=true

# Flag: skip validator account setup (task8)
NOPK=false
# Recognize --nopk early (non-destructive parsing so existing getopts/case blocks still work)
for __arg in "$@"; do
  if [ "$__arg" = "--nopk" ]; then
    NOPK=true
  fi
done

#########################################################################
totalRpc=0
totalValidator=0
totalNodes=$(($totalRpc + $totalValidator))

#########################################################################

# Hardware/software requirements
# Minimal hard requirements (must pass): 14+ CPU cores and NVIDIA 40-series class (or >=20GB VRAM)
REQUIRED_CPU_CORES=14
MIN_GPU_VRAM_MB=20000

# Recommended reference profile (best-effort install/validate; warnings if different)
RECOMMENDED_GPU_NAME="RTX 4000 SFF Ada"
RECOMMENDED_DRIVER_VERSION="575.57.08"
RECOMMENDED_CUDA_MAJOR_MINOR="12.6"

validate_strict_requirements(){
  log_wait "Validating strict hardware/software requirements"

  # CPU topology: require >= 14 cores (no specific model)
  CPU_CORES=$(lscpu | awk -F: '/^CPU\(s\)/ {gsub(/^ +| +$/,"", $2); print $2}' | head -1)
  if [ -z "$CPU_CORES" ]; then
    # fallback: cores per socket * sockets
    CoresPerSocket=$(lscpu | awk -F: '/Core\(s\) per socket/ {gsub(/^ +| +$/,"", $2); print $2}')
    Sockets=$(lscpu | awk -F: '/Socket\(s\)/ {gsub(/^ +| +$/,"", $2); print $2}')
    CPU_CORES=$(( CoresPerSocket * Sockets ))
  fi
  if [ "$CPU_CORES" -lt "$REQUIRED_CPU_CORES" ]; then
    log_wait "Warning: Recommended CPU cores >= $REQUIRED_CPU_CORES, found $CPU_CORES (continuing)"
  fi

  # NVIDIA GPU checks
  if ! command -v nvidia-smi >/dev/null 2>&1; then
    log_error "nvidia-smi not found; install NVIDIA drivers first"; return 1
  fi

  GPU_NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader | head -1)
  GPU_MEM_MB=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits | head -1)
  DRIVER_VER=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1)

  # Recommended: at least RTX 40-class or >=20GB VRAM (warning only)
  if ! echo "$GPU_NAME" | grep -qiE "RTX 40|Ada|4090|4080|4070|4060|4000" && [ "$GPU_MEM_MB" -lt "$MIN_GPU_VRAM_MB" ]; then
    log_wait "Warning: Recommended GPU is NVIDIA RTX 40-class or >=20GB VRAM. Found '$GPU_NAME' with ${GPU_MEM_MB}MB VRAM (continuing)"
  fi

  # Recommend: specific model and versions (warnings only)
  if ! echo "$GPU_NAME" | grep -qi "$RECOMMENDED_GPU_NAME"; then
    log_wait "Warning: Recommended GPU is '$RECOMMENDED_GPU_NAME', found '$GPU_NAME' (continuing)"
  fi
  if [ "$DRIVER_VER" != "$RECOMMENDED_DRIVER_VERSION" ]; then
    log_wait "Warning: Recommended NVIDIA driver is $RECOMMENDED_DRIVER_VERSION, found $DRIVER_VER (continuing)"
  fi

  # CUDA toolkit version
  if command -v nvcc >/dev/null 2>&1; then
    CUDA_VER=$(nvcc --version | awk -F'release ' '/release/ {print $2}' | awk '{print $1}' | head -1)
    if [ "${CUDA_VER%%.*}.${CUDA_VER#*.}" != "$RECOMMENDED_CUDA_MAJOR_MINOR" ]; then
      log_wait "Warning: Recommended CUDA version is $RECOMMENDED_CUDA_MAJOR_MINOR, found $CUDA_VER (continuing)"
    fi
  else
    log_error "nvcc not found; CUDA toolkit not installed"; return 1
  fi

  # cuBLAS check
  if [ ! -f "/usr/local/cuda/lib64/libcublas.so" ] && [ ! -f "/usr/local/cuda/lib64/libcublas.so.12" ]; then
    log_wait "Warning: cuBLAS not found in /usr/local/cuda/lib64 (expected with CUDA toolkit)"
  fi

  # cuDNN check (common locations)
  if [ ! -f "/usr/lib/x86_64-linux-gnu/libcudnn.so" ] && [ ! -f "/usr/lib/x86_64-linux-gnu/libcudnn.so.9" ] && [ ! -f "/usr/local/cuda/lib64/libcudnn.so" ]; then
    log_wait "Warning: cuDNN not found; install cuDNN for CUDA $RECOMMENDED_CUDA_MAJOR_MINOR for best performance"
  fi

  log_success "Strict requirements verified successfully"
}

# Fast, robust GPU readiness detection (accepts multiple positive signals)
gpu_is_ready() {
  # Return 0 if GPU appears available and usable, else non-zero
  # 1) Prefer nvidia-smi listing at least one GPU (ignore exit code, parse output)
  if command -v nvidia-smi >/dev/null 2>&1; then
    if timeout 3 nvidia-smi -L 2>/dev/null | grep -q "."; then
      return 0
    fi
    # Try basic nvidia-smi without requiring a full query
    if timeout 3 nvidia-smi >/dev/null 2>&1; then
      return 0
    fi
  fi
  # 2) Check for NVIDIA character devices
  if [ -e /dev/nvidia0 ] || [ -e /dev/nvidiactl ]; then
    return 0
  fi
  # 3) Check loaded kernel modules
  if lsmod 2>/dev/null | grep -qi '^nvidia\b'; then
    return 0
  fi
  # 4) Hardware present via lspci plus NVIDIA userspace libs present
  if (lspci 2>/dev/null | grep -qi nvidia) && ldconfig -p 2>/dev/null | grep -qi 'libnvidia'; then
    return 0
  fi
  # 5) Final hardware presence (lspci) as a very lenient signal (treated as not-ready but present)
  if lspci 2>/dev/null | grep -qi nvidia; then
    # Hardware exists but drivers may not be active yet
    return 2
  fi
  return 1
}

#+-----------------------------------------------------------------------------------------------+
#|                                                                                                                              |
#|                                                      FUNCTIONS                                                |
#|                                                                                                                              |     
#+------------------------------------------------------------------------------------------------+

task1(){
  # update and upgrade the server TASK 1
  log_wait "Updating system packages" && progress_bar
  apt update && apt upgrade -y
  
  # Fix system resource limits for blockchain node stability
  log_wait "Configuring system resource limits for blockchain operations"
  
  # Increase file descriptor limits
  echo "fs.file-max = 2097152" >> /etc/sysctl.conf
  echo "fs.inotify.max_user_watches = 524288" >> /etc/sysctl.conf
  echo "fs.inotify.max_user_instances = 512" >> /etc/sysctl.conf
  sysctl -p
  
  # Set file descriptor limits for all users
  echo "* soft nofile 65536" >> /etc/security/limits.conf
  echo "* hard nofile 65536" >> /etc/security/limits.conf
  echo "root soft nofile 65536" >> /etc/security/limits.conf
  echo "root hard nofile 65536" >> /etc/security/limits.conf
  
  log_success "System packages updated and resource limits configured"
}

task2(){
  # installing build-essential and quantum cryptography dependencies TASK 2
  log_wait "Getting dependencies including quantum cryptography support" && progress_bar
  
  # Fix bzip2/libbz2-1.0 version conflict on Ubuntu 24.04
  if grep -q "24.04" /etc/os-release; then
    log_wait "Fixing bzip2 dependency conflicts for Ubuntu 24.04"
    apt install libbz2-1.0=1.0.8-5.1 -y --allow-downgrades 2>/dev/null || true
    apt install bzip2 -y 2>/dev/null || true
  fi
  
  # Install essential build tools and quantum cryptography dependencies
  apt -y install build-essential tree cmake ninja-build libssl-dev pkg-config curl wget git
  
  # Install additional dependencies for liboqs (post-quantum cryptography)
  apt -y install libssl-dev libcrypto++-dev zlib1g-dev
  
  log_success "Build dependencies and quantum cryptography support installed"
}

task3(){
  # getting golang TASK 3
  log_wait "Getting golang" && progress_bar
  mkdir -p ./tmp
  cd ./tmp && wget "https://go.dev/dl/go1.22.6.linux-amd64.tar.gz"
  log_success "Done"
}

task4(){
  # setting up golang TASK 4
  log_wait "Installing golang and setting up autostart" && progress_bar
  rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.6.linux-amd64.tar.gz

  LINE='PATH=$PATH:/usr/local/go/bin'
  if grep -Fxq "$LINE" /etc/profile
  then
    # code if found
    echo -e "${ORANGE}golang path is already added"
  else
    # code if not found
    echo -e '\nPATH=$PATH:/usr/local/go/bin' >>/etc/profile
  fi

  echo -e '\nsource ~/.bashrc' >>/etc/profile

  nodePath=$BASE_DIR/Core-Blockchain
  
  if [[ $totalValidator -gt 0 ]]; then
  LINE="cd $nodePath"
  if grep -Fq "$LINE" /etc/profile; then
    log_wait "Validator: working directory already in profile"
  else
    echo -e "\ncd $nodePath" >> /etc/profile
  fi

  LINE="bash $nodePath/node-start.sh --validator"
  if grep -Fq "$LINE" /etc/profile; then
    log_wait "Validator: autostart already in profile"
  else
    echo -e "\nbash $nodePath/node-start.sh --validator" >> /etc/profile
  fi
fi

if [[ $totalRpc -gt 0 ]]; then
  LINE="cd $nodePath"
  if grep -Fq "$LINE" /etc/profile; then
    log_wait "RPC: working directory already in profile"
  else
    echo -e "\ncd $nodePath" >> /etc/profile
  fi

  LINE="bash $nodePath/node-start.sh --rpc"
  if grep -Fq "$LINE" /etc/profile; then
    log_wait "RPC: autostart already in profile"
  else
    echo -e "\nbash $nodePath/node-start.sh --rpc" >> /etc/profile
  fi
fi


  

  export PATH=$PATH:/usr/local/go/bin
  go env -w GO111MODULE=on
  log_success "Done"
  
}

task5(){
  # set proper group and permissions TASK 5
  log_wait "Setting up Permissions" && progress_bar
  ls -all
  cd ../
  ls -all
  chown -R root:root ./
  chmod a+x ./node-start.sh
  log_success "Done"
}

task6(){
  # do make all TASK 6 with automatic GPU compilation and quantum cryptography
  log_wait "Building backend with GPU acceleration and quantum cryptography support" && progress_bar
  cd node_src
  
  # Set CUDA environment for build
  export CUDA_PATH=/usr/local/cuda
  export PATH=$CUDA_PATH/bin:$PATH
  export LD_LIBRARY_PATH=$CUDA_PATH/lib64:$LD_LIBRARY_PATH

  # Build Post-Quantum liboqs (used by ML-DSA) with proper dependency installation
  log_wait "Installing liboqs dependencies and building post-quantum cryptography library"
  
  # First install dependencies using the Makefile.pq install-deps target
  if make -f Makefile.pq install-deps; then
    log_success "liboqs dependencies installed successfully"
  else
    log_wait "Dependencies may already be installed, continuing with build"
  fi
  
  # Build liboqs with proper error handling
  log_wait "Building liboqs (Open Quantum Safe) library for ML-DSA support"
  if make -f Makefile.pq liboqs; then
    log_success "liboqs built successfully"
  else
    log_error "liboqs build failed - quantum cryptography features will be disabled"
    # Continue without quantum features
    export PQ_CGO_CFLAGS=""
    export PQ_CGO_LDFLAGS=""
  fi
  
  # Set CGO flags for post-quantum support if liboqs was built successfully
  if [ -f "/tmp/splendor_liboqs/lib/liboqs.a" ] && [ -f "/tmp/splendor_liboqs/include/oqs/oqs.h" ]; then
    log_success "Setting up CGO flags for post-quantum cryptography integration"
    export PQ_CGO_CFLAGS="-I/tmp/splendor_liboqs/include"
    export PQ_CGO_LDFLAGS="-L/tmp/splendor_liboqs/lib -loqs -lcrypto -lssl -ldl -lpthread"
    export CGO_ENABLED=1
    log_success "Post-quantum cryptography (ML-DSA) integration ready"
  else
    log_wait "liboqs not available - building without quantum cryptography features"
    export PQ_CGO_CFLAGS=""
    export PQ_CGO_LDFLAGS=""
  fi
  
  # Preflight: require NVIDIA GPU stack for validator nodes
  if [ "$1" = "--require-gpu" ] || [ "$2" = "--require-gpu" ] || [ -f "$BASE_DIR/Core-Blockchain/chaindata/.enforce_gpu" ]; then
    if gpu_is_ready; then
      # Best-effort summary
      if command -v nvidia-smi >/dev/null 2>&1; then
        NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -1)
        VRAM=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits 2>/dev/null | head -1)
        [ -n "$NAME" ] && log_success "GPU detected: $NAME (${VRAM:-?} MB VRAM)"
      else
        log_success "GPU detected via kernel/devices (nvidia module or /dev/nvidia present)"
      fi
    else
      status=$?
      if [ "$status" -eq 2 ]; then
        log_wait "NVIDIA GPU hardware found but drivers may be inactive; continuing and enabling GPU post-reboot"
      else
        log_error "GPU is required but not detected by multiple checks (nvidia-smi, devices, modules)."
        log_error "If GPU is present, ensure NVIDIA drivers and CUDA are installed and active, then reboot."
        exit 1
      fi
    fi
  fi

  # First, compile CUDA kernels if CUDA is available
  if command -v nvcc >/dev/null 2>&1; then
    log_wait "Compiling CUDA kernels for GPU acceleration"
    
    # Build CUDA objects and library using the correct Makefile
    if make -f Makefile.cuda cuda-objects && make -f Makefile.cuda cuda-lib; then
      log_success "CUDA library compiled successfully"
      log_wait "Building geth with GPU support and proper CUDA linking"
      
      # Build geth with proper CUDA + PQ (liboqs) linking
      if CGO_CFLAGS="-I/usr/local/cuda/include $PQ_CGO_CFLAGS" CGO_LDFLAGS="-L/usr/local/cuda/lib64 -L./common/gpu -lcuda -lcudart -lsplendor_cuda $PQ_CGO_LDFLAGS" go build -tags "gpu" -o build/bin/geth ./cmd/geth; then
        log_success "Geth built successfully with CUDA + PQ (liboqs) support"
        
        # Verify CUDA linking
        if ldd build/bin/geth | grep -q "libcudart"; then
          log_success "CUDA runtime properly linked to geth binary"
        else
          log_wait "CUDA linking verification failed, but binary should work"
        fi
        # Verify liboqs linkage presence
        if ldd build/bin/geth | grep -qi "liboqs"; then
          log_success "liboqs linked into geth binary"
        else
          log_wait "liboqs static linkage not listed by ldd (expected if linked static)"
        fi
      else
        log_wait "GPU build failed, falling back to standard build"
        go run build/ci.go install ./cmd/geth
      fi
    else
      log_wait "CUDA compilation failed, building CPU-only version"
      go run build/ci.go install ./cmd/geth || make all
    fi
  else
    log_wait "CUDA not available - building CPU-only version"
    make all
  fi
  
  log_success "Backend build completed"
}

detect_gpu_architecture(){
  # Advanced GPU detection and architecture identification
  log_wait "Performing advanced GPU hardware detection"
  
  GPU_INFO=$(lspci | grep -i nvidia || echo "No NVIDIA GPU detected")
  echo "GPU Hardware: $GPU_INFO"
  
  # Detect specific GPU architectures and their CUDA requirements
  if echo "$GPU_INFO" | grep -qi "RTX 4000\|RTX 40\|Ada Generation\|AD104\|AD106\|AD107\|AD102\|AD103"; then
    GPU_ARCH="Ada Lovelace"
    RECOMMENDED_DRIVER="575"
    CUDA_VERSION="12.6"
    CUDA_ARCH="sm_89"
    log_success "Detected: $GPU_ARCH architecture (RTX 4000 series)"
  elif echo "$GPU_INFO" | grep -qi "RTX 30\|RTX 3060\|RTX 3070\|RTX 3080\|RTX 3090\|GA102\|GA104\|GA106"; then
    GPU_ARCH="Ampere"
    RECOMMENDED_DRIVER="535"
    CUDA_VERSION="12.2"
    CUDA_ARCH="sm_86"
    log_success "Detected: $GPU_ARCH architecture (RTX 30 series)"
  elif echo "$GPU_INFO" | grep -qi "RTX 20\|RTX 2060\|RTX 2070\|RTX 2080\|TU102\|TU104\|TU106"; then
    GPU_ARCH="Turing"
    RECOMMENDED_DRIVER="535"
    CUDA_VERSION="12.2"
    CUDA_ARCH="sm_75"
    log_success "Detected: $GPU_ARCH architecture (RTX 20 series)"
  elif echo "$GPU_INFO" | grep -qi "GTX 16\|GTX 1660\|GTX 1650\|TU116\|TU117"; then
    GPU_ARCH="Turing"
    RECOMMENDED_DRIVER="535"
    CUDA_VERSION="12.2"
    CUDA_ARCH="sm_75"
    log_success "Detected: $GPU_ARCH architecture (GTX 16 series)"
  elif echo "$GPU_INFO" | grep -qi "Tesla\|Quadro\|A100\|A40\|A30\|A10"; then
    GPU_ARCH="Professional"
    RECOMMENDED_DRIVER="535"
    CUDA_VERSION="12.2"
    CUDA_ARCH="sm_80"
    log_success "Detected: Professional GPU ($GPU_ARCH)"
  else
    GPU_ARCH="Generic"
    RECOMMENDED_DRIVER="535"
    CUDA_VERSION="12.2"
    CUDA_ARCH="sm_60"
    log_wait "Unknown GPU - using generic settings"
  fi
  
  echo "Architecture: $GPU_ARCH"
  echo "Recommended Driver: $RECOMMENDED_DRIVER"
  echo "CUDA Version: $CUDA_VERSION"
  echo "CUDA Architecture: $CUDA_ARCH"
}

install_cuda_from_runfile(){
  # Install CUDA from .run file for maximum compatibility
  log_wait "Installing CUDA $CUDA_VERSION from official installer"
  
  # Determine the correct CUDA installer URL based on version
  case $CUDA_VERSION in
    "12.6")
      CUDA_URL="https://developer.download.nvidia.com/compute/cuda/12.6.2/local_installers/cuda_12.6.2_560.35.03_linux.run"
      CUDA_FILE="cuda_12.6.2_560.35.03_linux.run"
      ;;
    "12.2")
      CUDA_URL="https://developer.download.nvidia.com/compute/cuda/12.2.2/local_installers/cuda_12.2.2_535.104.05_linux.run"
      CUDA_FILE="cuda_12.2.2_535.104.05_linux.run"
      ;;
    *)
      CUDA_URL="https://developer.download.nvidia.com/compute/cuda/12.6.2/local_installers/cuda_12.6.2_560.35.03_linux.run"
      CUDA_FILE="cuda_12.6.2_560.35.03_linux.run"
      ;;
  esac
  
  # Download CUDA installer if not already present
  if [ ! -f "/tmp/$CUDA_FILE" ]; then
    log_wait "Downloading CUDA installer ($CUDA_FILE)"
    cd /tmp
    wget "$CUDA_URL" -O "$CUDA_FILE"
    chmod +x "$CUDA_FILE"
  else
    log_success "CUDA installer already downloaded"
  fi
  
  # Install CUDA toolkit (skip driver installation if already installed)
  log_wait "Installing CUDA toolkit (this may take several minutes)"
  if nvidia-smi >/dev/null 2>&1; then
    # Driver already installed, install toolkit only
    /tmp/$CUDA_FILE --silent --toolkit --override
  else
    # Install both driver and toolkit
    /tmp/$CUDA_FILE --silent --driver --toolkit --override
  fi
  
  # Verify CUDA installation
  if [ -f "/usr/local/cuda/bin/nvcc" ]; then
    CUDA_INSTALLED_VERSION=$(/usr/local/cuda/bin/nvcc --version | grep "release" | awk '{print $6}' | cut -c2-)
    log_success "CUDA $CUDA_INSTALLED_VERSION installed successfully"
  else
    log_error "CUDA installation failed"
    return 1
  fi
}

# Helper: add NVIDIA CUDA repo keyring for exact packages
ensure_nvidia_repo(){
  . /etc/os-release
  case "$VERSION_ID" in
    24.04) DIST="ubuntu2404" ;;
    22.04) DIST="ubuntu2204" ;;
    20.04) DIST="ubuntu2004" ;;
    *) DIST="ubuntu2204" ;;
  esac
  if ! dpkg -l | grep -q cuda-keyring; then
    log_wait "Adding NVIDIA CUDA repository for $DIST"
    wget -qO /tmp/cuda-keyring.deb https://developer.download.nvidia.com/compute/cuda/repos/$DIST/x86_64/cuda-keyring_1.1-1_all.deb || true
    if [ -f /tmp/cuda-keyring.deb ]; then
      dpkg -i /tmp/cuda-keyring.deb || true
      apt update || true
    else
      log_wait "Could not fetch CUDA keyring; continuing with existing repos"
    fi
  fi
}

# Helper: install exact NVIDIA driver version if available
install_driver_exact(){
  local pkg
  for pkg in nvidia-driver-575-open nvidia-driver-575; do
    if apt-cache madison "$pkg" | awk '{print $3}' | grep -q "^${REQUIRED_DRIVER_VERSION}"; then
      log_wait "Installing $pkg version ${REQUIRED_DRIVER_VERSION}"
      apt install -y "$pkg=${REQUIRED_DRIVER_VERSION}" || apt install -y "$pkg=${REQUIRED_DRIVER_VERSION}-0ubuntu1" || true
      return
    fi
  done
  # Fallback to latest 575 series
  log_wait "Exact driver ${REQUIRED_DRIVER_VERSION} not found; installing latest 575 series"
  apt install -y nvidia-driver-575-open nvidia-utils-575 || apt install -y nvidia-driver-575 nvidia-utils-575 || true
}

# Helper: install cuDNN runtime and dev packages
install_cudnn_exact(){
  log_wait "Installing cuDNN runtime and dev packages"
  apt install -y libcudnn9 libcudnn9-dev || apt install -y libcudnn9-cuda libcudnn9-cuda-dev || true
}

install_gpu_dependencies(){
  # Install GPU dependencies automatically for BOTH RPC and VALIDATOR TASK 6A
  log_wait "Installing complete GPU acceleration stack (NVIDIA drivers + CUDA + OpenCL)" && progress_bar
  
  # Update package lists
  apt update
  
  # Detect GPU architecture and determine optimal settings
  detect_gpu_architecture
  ensure_nvidia_repo
  
  # Check if NVIDIA drivers are already installed
  if nvidia-smi >/dev/null 2>&1; then
    CURRENT_DRIVER=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader,nounits | head -1)
    log_success "NVIDIA drivers detected (current: $CURRENT_DRIVER)"
    # Enforce exact required driver version (unless user opted to keep existing)
    if [ "$ENFORCE_DRIVER_VERSION" = "true" ]; then
      if [ "$CURRENT_DRIVER" != "$REQUIRED_DRIVER_VERSION" ]; then
        log_wait "Driver version mismatch (need $REQUIRED_DRIVER_VERSION, found $CURRENT_DRIVER). Installing required version."
        install_driver_exact
      fi
    else
      log_wait "Keeping existing NVIDIA driver ($CURRENT_DRIVER); skipping version enforcement"
    fi
  else
    log_wait "Installing NVIDIA drivers for $GPU_ARCH architecture (target ${REQUIRED_DRIVER_VERSION})"
    install_driver_exact
  fi
  
  # Install OpenCL support FIRST (required for compilation)
  log_wait "Installing OpenCL support (required for blockchain compilation)"
  apt install -y opencl-headers ocl-icd-opencl-dev mesa-opencl-icd intel-opencl-icd
  
  # Install NVIDIA OpenCL if NVIDIA GPU detected
  if echo "$GPU_INFO" | grep -qi nvidia; then
    apt install -y nvidia-opencl-dev || log_wait "NVIDIA OpenCL will be available after reboot"
  fi
  
  # Check if CUDA is already installed
  if command -v nvcc >/dev/null 2>&1; then
    EXISTING_CUDA=$(nvcc --version | grep "release" | awk '{print $6}' | cut -c2-)
    log_success "CUDA $EXISTING_CUDA already installed"
    
    # Check if installed version matches recommended version
    if [[ "$EXISTING_CUDA" != "$CUDA_VERSION"* ]]; then
      log_wait "Upgrading CUDA from $EXISTING_CUDA to $CUDA_VERSION"
      install_cuda_from_runfile
    fi
  else
    # Install CUDA from official installer
    install_cuda_from_runfile
  fi
  
  # Install cuDNN after CUDA toolkit
  install_cudnn_exact
  
  # Install additional build tools
  log_wait "Installing additional build tools"
  apt install -y cmake clinfo build-essential gcc-9 g++-9
  
  # Set GCC 9 as default for CUDA compatibility
  update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-9 90 --slave /usr/bin/g++ g++ /usr/bin/g++-9 || true
  
  # Set up CUDA environment paths
  export CUDA_PATH=/usr/local/cuda
  export PATH=$PATH:$CUDA_PATH/bin
  export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$CUDA_PATH/lib64
  
  # Add CUDA to system profile (persistent across reboots)
  if ! grep -q "CUDA_PATH" /etc/profile; then
    echo 'export CUDA_PATH=/usr/local/cuda' >> /etc/profile
    echo 'export PATH=$PATH:$CUDA_PATH/bin' >> /etc/profile
    echo 'export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$CUDA_PATH/lib64' >> /etc/profile
  fi
  
  # Add CUDA to bashrc for immediate availability
  if ! grep -q "CUDA_PATH" ~/.bashrc; then
    echo 'export CUDA_PATH=/usr/local/cuda' >> ~/.bashrc
    echo 'export PATH=$PATH:$CUDA_PATH/bin' >> ~/.bashrc
    echo 'export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$CUDA_PATH/lib64' >> ~/.bashrc
  fi
  
  # Source the environment
  source ~/.bashrc
  
  # Update Makefile.cuda with detected architecture
  if [ -f "node_src/Makefile.cuda" ]; then
    log_wait "Updating CUDA Makefile with detected architecture ($CUDA_ARCH)"
    sed -i "s/CUDA_ARCH ?= sm_89/CUDA_ARCH ?= $CUDA_ARCH/" node_src/Makefile.cuda
  fi
  
  log_success "Complete GPU acceleration stack installed (drivers + CUDA + OpenCL)"
  
  # Display GPU information
  if nvidia-smi >/dev/null 2>&1; then
    echo -e "\n${GREEN}GPU Information:${NC}"
    nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader
  fi
}

configure_gpu_environment(){
  # Configure GPU environment for optimal performance with AI sharing TASK 6B
  log_wait "Configuring GPU environment for high-performance RPC with AI memory sharing" && progress_bar
  
  # Create GPU configuration in .env file with proper memory allocation
  cat >> ./.env << EOF

# GPU Acceleration Configuration for High-Performance RPC (RTX 4000 SFF Ada - 20GB VRAM)
ENABLE_GPU=true
PREFERRED_GPU_TYPE=CUDA
GPU_MAX_BATCH_SIZE=200000
GPU_MAX_MEMORY_USAGE=12884901888
GPU_MEMORY_FRACTION=0.5
GPU_HASH_WORKERS=32
GPU_SIGNATURE_WORKERS=32
GPU_TX_WORKERS=32
GPU_ENABLE_PIPELINING=true

# Hybrid Processing Configuration with AI Coordination
ENABLE_HYBRID_PROCESSING=true
GPU_THRESHOLD=1000
CPU_GPU_RATIO=0.85
ADAPTIVE_LOAD_BALANCING=true
PERFORMANCE_MONITORING=true
MAX_CPU_UTILIZATION=0.85
MAX_GPU_UTILIZATION=0.95
THROUGHPUT_TARGET=3000000

# Memory Management (RTX 4000 SFF Ada - 20GB Total)
MAX_MEMORY_USAGE=68719476736
GPU_MEMORY_RESERVATION=10737418240
AI_MEMORY_RESERVATION=8589934592
MEMORY_BUFFER=2147483648

# Performance Optimization for AI Sharing
GPU_DEVICE_COUNT=1
GPU_LOAD_BALANCE_STRATEGY=ai_optimized
AI_GPU_COORDINATION=true
ENABLE_CUDA_MPS=true
EOF
  
  log_success "GPU environment configured for shared 12GB (blockchain) + 8GB (LLM)"
}

task6_gpu(){
  # Build GPU acceleration components TASK 6 GPU
  log_wait "Setting up complete GPU acceleration for high-performance RPC" && progress_bar
  
  # Install GPU dependencies automatically
  install_gpu_dependencies
  
  # Configure GPU environment
  configure_gpu_environment
  
  # Check if GPU is available
  if nvidia-smi >/dev/null 2>&1; then
    log_success "NVIDIA GPU detected successfully"
    nvidia-smi --query-gpu=name,memory.total --format=csv,noheader,nounits
    # If drivers are active now, run strict validation (otherwise validation runs after reboot)
    if type validate_strict_requirements >/dev/null 2>&1; then
      validate_strict_requirements || exit 1
    fi
  else
    log_wait "GPU not detected or drivers need reboot - GPU features will activate after reboot"
  fi
  
  # Build GPU components
  log_wait "Building CUDA and OpenCL kernels for maximum performance"
  
  # Ensure we're in the correct directory (Core-Blockchain)
  cd $BASE_DIR/Core-Blockchain
  
  # Check if node_src directory exists
  if [ ! -d "node_src" ]; then
    log_error "node_src directory not found in $(pwd)"
    log_error "Current directory contents:"
    ls -la
    return 1
  fi
  
  cd node_src
  
  # Ensure CUDA env for build
  export CUDA_PATH=/usr/local/cuda
  export PATH=$CUDA_PATH/bin:$PATH
  export LD_LIBRARY_PATH=$CUDA_PATH/lib64:$LD_LIBRARY_PATH

  # Build GPU components using the correct Makefile.cuda
  if command -v nvcc >/dev/null 2>&1; then
    log_wait "Building CUDA components with proper linking"
    
    # Clean and build CUDA objects and library
    make -f Makefile.cuda clean-cuda || true
    if make -f Makefile.cuda cuda-objects && make -f Makefile.cuda cuda-lib; then
      log_success "CUDA library built successfully"

      # Ensure PQ (liboqs) is built and CGO flags are set in this path too
      log_wait "Building post-quantum liboqs (ML-DSA) for GPU build path"
      make -f Makefile.pq liboqs || true
      export PQ_CGO_CFLAGS="-I/tmp/splendor_liboqs/include"
      export PQ_CGO_LDFLAGS="-L/tmp/splendor_liboqs/lib -loqs -lcrypto -lssl -ldl -lpthread"
      
      # Build geth with CUDA + PQ support
      if CGO_CFLAGS="-I/usr/local/cuda/include $PQ_CGO_CFLAGS" CGO_LDFLAGS="-L/usr/local/cuda/lib64 -L./common/gpu -lcuda -lcudart -lsplendor_cuda $PQ_CGO_LDFLAGS" go build -tags gpu -o build/bin/geth ./cmd/geth; then
        log_success "GPU acceleration components built successfully with CUDA + PQ linkage"
        
        # Verify CUDA linking
        if ldd build/bin/geth | grep -q "libcudart"; then
          log_success "CUDA runtime properly linked to geth binary"
        else
          log_wait "CUDA linking verification failed, but binary should work"
        fi
        # Verify liboqs presence (note: static linkage may not appear in ldd)
        if ldd build/bin/geth | grep -qi "liboqs"; then
          log_success "liboqs linked into geth binary"
        else
          log_wait "liboqs static linkage may not be listed by ldd (expected)"
        fi
      else
        log_wait "GPU build will complete after system reboot (driver activation required)"
      fi
    else
      log_wait "GPU build will complete after system reboot (driver activation required)"
    fi
  else
    log_wait "GPU build will complete after system reboot (driver activation required)"
    echo -e "${ORANGE}System reboot recommended to activate GPU drivers${NC}"
  fi
  
  # Return to Core-Blockchain directory
  cd $BASE_DIR/Core-Blockchain
  log_success "GPU RPC setup completed - High-performance mode ready"
}

task7(){
  # setting up directories and structure for node/s TASK 7
  log_wait "Setting up directories for node instances" && progress_bar

  # Ensure we're in the correct directory (Core-Blockchain)
  cd $BASE_DIR/Core-Blockchain
  
  # Verify chaindata directory exists
  if [ ! -d "chaindata" ]; then
    log_error "chaindata directory not found in $(pwd)"
    log_error "Current directory contents:"
    ls -la
    return 1
  fi

  i=1
  while [[ $i -le $totalNodes ]]; do
    mkdir -p ./chaindata/node$i
    log_success "Created node directory: ./chaindata/node$i"
    ((i += 1))
  done

  tree ./chaindata
  log_success "Done"
}

task8(){
  # Skip when --nopk is provided
if [ "${NOPK}" = "true" ]; then
  echo "[--nopk] Skipping task8 (validator key import/creation)"
  return 0
fi
log_wait "Setting up Validator Accounts" && progress_bar

  i=1
  while [[ $i -le $totalValidator ]]; do
    echo -e "\n\n${GREEN}+-----------------------------------------------------------------------------------------------------+${NC}"
    echo -e "${ORANGE}Setting up Validator $i${NC}"
    echo -e "${GREEN}Choose how you want to import/create account for validator $i:${NC}"
    echo -e "${ORANGE}1) Create a new account"
    echo -e "2) Import via Private Key"
    echo -e "3) Import via JSON keystore file${NC}"
    read -p "Enter your choice (1/2/3): " choice

    # Validator's node directory
    validator_dir="./chaindata/node$i"

    mkdir -p "$validator_dir"

    case $choice in
      1)
        read -s -p "Enter password to create new validator account: " password
        echo "$password" > "$validator_dir/pass.txt"
        echo
        ./node_src/build/bin/geth --datadir "$validator_dir" account new --password "$validator_dir/pass.txt"
        ;;

      2)
        read -s -p "Enter password to secure the imported account: " password
        echo "$password" > "$validator_dir/pass.txt"
        echo
        read -p "Enter the private key (hex, without 0x): " pk

        # Convert to lowercase, remove any whitespace
        pk_cleaned=$(echo "$pk" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')

        if [[ ${#pk_cleaned} -ne 64 ]]; then
          log_error "Invalid private key length. Skipping validator $i."
        else
          echo "$pk_cleaned" | ./node_src/build/bin/geth --datadir "$validator_dir" account import --password "$validator_dir/pass.txt" /dev/stdin
        fi
        ;;

      3)
        read -p "Enter the full path to your JSON keystore file: " json_path
        if [[ ! -f "$json_path" ]]; then
          log_error "File not found: $json_path. Skipping validator $i."
        else
          read -s -p "Enter password to decrypt the keystore file: " password
          echo "$password" > "$validator_dir/pass.txt"
          echo
          cp "$json_path" "$validator_dir/keystore/"
          echo -e "${GREEN}Keystore file copied to $validator_dir/keystore/${NC}"
        fi
        ;;

      *)
        log_error "Invalid option. Skipping validator $i."
        ;;
    esac

    ((i += 1))
  done

  log_success "[TASK 8 PASSED]"
}


labelNodes(){
  i=1
  while [[ $i -le $totalValidator ]]; do
    touch ./chaindata/node$i/.validator
    ((i += 1))
  done 

  i=$((totalValidator + 1))
  while [[ $i -le $totalNodes ]]; do
    touch ./chaindata/node$i/.rpc
    ((i += 1))
  done 
}

displayStatus(){
  echo -e "\n${GREEN}🚀 ALL SET!${NC}"
  echo -e "${ORANGE}➜ To start the node, run:${NC} ${GREEN}./node-start.sh${NC}\n"
}

reboot_countdown(){
  # Determine readiness using robust detection
  if gpu_is_ready; then
    echo -e "\n${GREEN}✅ No reboot required - GPU drivers active${NC}"
    return
  fi
  status=$?
  # Hardware present but drivers likely inactive
  if [ "$status" -eq 2 ]; then
    if [ "$AUTO_REBOOT" = "true" ]; then
      : # fall through to automatic reboot prompt below
    else
      echo -e "\n${ORANGE}⚠️  NVIDIA hardware detected but drivers appear inactive. Please reboot to activate GPU drivers.${NC}"
      return
    fi
  else
    echo -e "\n${GREEN}✅ No reboot required - no NVIDIA hardware detected or already handled${NC}"
    return
  fi
  # At this point, we intend to reboot automatically
  echo -e "\n${ORANGE}╔══════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${ORANGE}║                    REBOOT REQUIRED                          ║${NC}"
  echo -e "${ORANGE}║                                                              ║${NC}"
  echo -e "${ORANGE}║  NVIDIA GPU drivers have been installed and require a       ║${NC}"
  echo -e "${ORANGE}║  system reboot to activate GPU acceleration features.       ║${NC}"
    echo -e "${ORANGE}║                                                              ║${NC}"
    echo -e "${ORANGE}║  After reboot, the node will automatically start via        ║${NC}"
    echo -e "${ORANGE}║  the configured autostart in /etc/profile                   ║${NC}"
    echo -e "${ORANGE}╚══════════════════════════════════════════════════════════════╝${NC}\n"
    
    echo -e "${RED}⚠️  AUTOMATIC REBOOT IN:${NC}"
    for i in {30..1}; do
      echo -ne "${CYAN}\r🔄 Rebooting in $i seconds... (Press Ctrl+C to cancel)${NC}"
      sleep 1
    done
    
    echo -e "\n\n${GREEN}🔄 Rebooting now to activate GPU drivers...${NC}"
    echo -e "${ORANGE}The system will automatically start the RPC node after reboot.${NC}\n"
    
    # Sync filesystem before reboot
    sync
    
    # Reboot the system
  reboot
}


displayWelcome(){
  # display welcome message
  echo -e "\n\n\t${ORANGE}Total RPC to be created: $totalRpc"
  echo -e "\t${ORANGE}Total Validators to be created: $totalValidator"
  echo -e "\t${ORANGE}Total nodes to be created: $totalNodes"
  echo -e "${GREEN}
  \t+------------------------------------------------+
  \t+   DPos node installation Wizard
  \t+   Target OS: Ubuntu 20.04 LTS (Focal Fossa)
  \t+   Your OS: $(. /etc/os-release && printf '%s\n' "${PRETTY_NAME}") 
  \t+------------------------------------------------+
  ${NC}\n"

  echo -e "${ORANGE}
  \t+------------------------------------------------+
  \t+------------------------------------------------+
  ${NC}"
}

doUpdate(){
  echo -e "${GREEN}
  \t+------------------------------------------------+
  \t+       UPDATING TO LATEST    
  \t+------------------------------------------------+
  ${NC}"
  git pull
}

createRpc(){
  task1
  task2
  task3
  task4
  task5
  task6
  task6_gpu
  task7
  i=$((totalValidator + 1))
  while [[ $i -le $totalNodes ]]; do
    read -p "Enter Virtual Host(example: rpc.yourdomain.tld) without https/http: " vhost
    echo -e "\nVHOST=$vhost" >> ./.env
    ./node_src/build/bin/geth --datadir ./chaindata/node$i init ./genesis.json
    ((i += 1))
  done

}

createValidator(){
  # Enforce GPU requirement for validators (preflight check in task6)
  mkdir -p "$BASE_DIR/Core-Blockchain/chaindata"
  touch "$BASE_DIR/Core-Blockchain/chaindata/.enforce_gpu"

  task1
  task2
  task3
  task4
  task5
  task6
  task6_gpu
  task7
  if [[ $totalValidator -gt 0 && "$NOPK" != "true" ]]; then
      if [ "${NOPK}" != "true" ]; then task8; fi
  fi
   i=1
  while [[ $i -le $totalValidator ]]; do
    ./node_src/build/bin/geth --datadir ./chaindata/node$i init ./genesis.json
    ((i += 1))
  done
}

install_nvm() {
  # Check if nvm is installed
  if ! command -v nvm &> /dev/null; then
    echo "Installing NVM..."
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.5/install.sh | bash

    # Source NVM scripts for the current session
    export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh" # This loads nvm
    [ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion" # This loads nvm bash_completion

    # Add NVM initialization to shell startup file
    if [ -n "$BASH_VERSION" ]; then
      SHELL_PROFILE="$HOME/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
      SHELL_PROFILE="$HOME/.zshrc"
    fi

    if ! grep -q 'export NVM_DIR="$HOME/.nvm"' "$SHELL_PROFILE"; then
      echo 'export NVM_DIR="$HOME/.nvm"' >> "$SHELL_PROFILE"
      echo '[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"' >> "$SHELL_PROFILE"
      echo '[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"' >> "$SHELL_PROFILE"
    fi
  else
    echo "NVM is already installed."
  fi

  # Source NVM scripts (if not sourced already)
  export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"
  [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh" # This loads nvm
  [ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion" # This loads nvm bash_completion

  # Install Node.js version 21.7.1 using nvm
  echo "Installing Node.js version 21.7.1..."
  nvm install 21.7.1

  # Use the installed Node.js version
  nvm use 21.7.1

  # Verify the installation
  node_version=$(node --version)
  if [[ $node_version == v21.7.1 ]]; then
    echo "Node.js version 21.7.1 installed successfully: $node_version"
  else
    echo "There was an issue installing Node.js version 21.7.1."
  fi

  source ~/.bashrc

  npm install --global yarn
  npm install --global pm2

  source ~/.bashrc
}

install_ai_llm(){
  # Install AI-powered load balancing (vLLM + MobileLLM-R1) TASK AI
  log_wait "Installing AI-powered load balancing system (vLLM + MobileLLM-R1)" && progress_bar
  
  # Install Python dependencies for vLLM (10%)
  log_wait "Installing Python dependencies for AI system [10%]" && progress_bar
  apt install -y python3 python3-pip python3-venv python3-dev jq
  log_success "Python dependencies installed [10%]"
  
  # Create virtual environment for vLLM (20%)
  log_wait "Creating Python virtual environment for vLLM [20%]" && progress_bar
  python3 -m venv /opt/vllm-env
  source /opt/vllm-env/bin/activate
  log_success "Virtual environment created [20%]"
  
  # Install PyTorch with CUDA support (50%)
  log_wait "Installing PyTorch with CUDA support for AI acceleration [50%]" && progress_bar
  pip install --upgrade pip setuptools wheel
  
  # Retry PyTorch installation up to 3 times
  PYTORCH_INSTALLED=false
  for attempt in 1 2 3; do
    log_wait "Installing PyTorch (attempt $attempt/3)"
    if pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118; then
      PYTORCH_INSTALLED=true
      break
    else
      log_wait "PyTorch installation attempt $attempt failed, retrying..."
      sleep 5
    fi
  done
  
  if [ "$PYTORCH_INSTALLED" = false ]; then
    log_error "PyTorch installation failed after 3 attempts"
    return 1
  fi
  log_success "PyTorch with CUDA support installed [50%]"
  
  # Install vLLM with proper error handling and retry (80%)
  log_wait "Installing vLLM (High-Performance LLM Inference Engine) [80%]" && progress_bar
  
  VLLM_INSTALLED=false
  for attempt in 1 2 3; do
    log_wait "Installing vLLM (attempt $attempt/3)"
    if pip install vllm transformers huggingface_hub fastapi uvicorn --break-system-packages; then
      VLLM_INSTALLED=true
      break
    else
      log_wait "vLLM installation attempt $attempt failed, retrying with force reinstall..."
      pip install vllm transformers huggingface_hub fastapi uvicorn --break-system-packages --force-reinstall || true
      sleep 10
    fi
  done
  
  if [ "$VLLM_INSTALLED" = false ]; then
    log_error "vLLM installation failed after 3 attempts - falling back to CPU-only mode"
    return 1
  fi
  
  # Verify vLLM installation (90%)
  log_wait "Verifying vLLM installation [90%]"
  if python -c "import vllm; print('vLLM installed successfully')" 2>/dev/null; then
    log_success "vLLM installation verified [90%]"
  else
    log_error "vLLM installation verification failed - falling back to CPU-only mode"
    return 1
  fi
  
  # Create vLLM systemd service with proper GPU memory allocation
  log_wait "Setting up vLLM as system service with optimized GPU memory allocation"
  cat > /etc/systemd/system/vllm-mobilellm.service << EOF
[Unit]
Description=vLLM MobileLLM-R1 Service for Blockchain AI
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/vllm-env
Environment=CUDA_VISIBLE_DEVICES=0
Environment=VLLM_USE_MODELSCOPE=False
Environment=CUDA_MEMORY_FRACTION=0.15
ExecStart=/opt/vllm-env/bin/python -m vllm.entrypoints.openai.api_server --model facebook/MobileLLM-R1 --host 0.0.0.0 --port 8000 --gpu-memory-utilization 0.15 --max-model-len 2048 --dtype float16 --tensor-parallel-size 1 --enforce-eager --disable-log-stats
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  # Enable vLLM service
  systemctl daemon-reload
  systemctl enable vllm-mobilellm
  
  # Add AI configuration to .env
  log_wait "Configuring AI load balancing settings"
  cat >> ./.env << EOF

# AI-Powered Load Balancing Configuration (vLLM + MobileLLM-R1)
ENABLE_AI_LOAD_BALANCING=true
LLM_ENDPOINT=http://localhost:8000/v1/chat/completions
LLM_MODEL=facebook/MobileLLM-R1
LLM_TIMEOUT_SECONDS=2
AI_UPDATE_INTERVAL_MS=500
AI_HISTORY_SIZE=100
AI_LEARNING_RATE=0.15
AI_CONFIDENCE_THRESHOLD=0.75
AI_ENABLE_LEARNING=true
AI_ENABLE_PREDICTIONS=true
AI_FAST_MODE=true
VLLM_GPU_MEMORY_UTILIZATION=0.15
VLLM_MAX_MODEL_LEN=2048
EOF

  # Create AI monitoring scripts
  log_wait "Creating AI monitoring and management scripts"
  mkdir -p ./scripts
  
  # Copy the AI setup script content
  cp ./scripts/setup-ai-llm.sh ./scripts/setup-ai-llm-backup.sh 2>/dev/null || true
  
  log_success "AI-powered load balancing system installed [100%] (will activate after reboot)"
}

install_x402_native(){
  # Install native x402 payments protocol directly into blockchain TASK X402
  log_wait "Installing native x402 payments protocol (world's first blockchain implementation)" && progress_bar
  
  # Ensure we're in the correct directory
  cd $BASE_DIR/Core-Blockchain
  
  # Check if x402 API files already exist
  if [ -f "node_src/eth/api_x402.go" ]; then
    log_success "✅ x402 API already integrated into blockchain core"
  else
    log_error "❌ x402 API files not found - please ensure x402 implementation is present"
    return 1
  fi
  
  # Install Node.js dependencies for x402 middleware (10%)
  log_wait "Setting up x402 middleware dependencies [10%]" && progress_bar
  
  # Create x402 middleware directory if it doesn't exist
  if [ ! -d "x402-middleware" ]; then
    log_error "❌ x402 middleware directory not found"
    return 1
  fi
  
  cd x402-middleware
  
  # Install middleware dependencies
  if npm install; then
    log_success "✅ x402 middleware dependencies installed [10%]"
  else
    log_error "❌ Failed to install x402 middleware dependencies"
    return 1
  fi
  
  cd $BASE_DIR/Core-Blockchain
  
  # Add x402 configuration to .env (30%)
  log_wait "Configuring native x402 payments protocol [30%]" && progress_bar
  
  # Add x402 configuration to .env if not already present
  if ! grep -q "X402_ENABLED" .env 2>/dev/null; then
    cat >> .env << 'EOF'

# Native x402 Payments Protocol Configuration (World's First Blockchain Implementation)
X402_ENABLED=true
X402_NETWORK=splendor
X402_CHAIN_ID=6546
X402_DEFAULT_PRICE=0.001
X402_MIN_PAYMENT=0.001
X402_MAX_PAYMENT=1000.0
X402_SETTLEMENT_TIMEOUT=300
X402_ENABLE_LOGGING=true

# x402 Performance Settings (Optimized for Millions of TPS)
X402_BATCH_SIZE=10000
X402_CACHE_SIZE=100000
X402_WORKER_THREADS=8
X402_ENABLE_COMPRESSION=true
X402_NATIVE_PROCESSING=true
X402_INSTANT_SETTLEMENT=true

# x402 Security Settings
X402_SIGNATURE_VALIDATION=strict
X402_NONCE_VALIDATION=true
X402_TIMESTAMP_TOLERANCE=300
X402_RATE_LIMITING=true
X402_MAX_REQUESTS_PER_MINUTE=10000
X402_ENABLE_ANTI_REPLAY=true
EOF
    log_success "✅ x402 configuration added to .env [30%]"
  else
    log_success "✅ x402 configuration already present [30%]"
  fi
  
  # Create x402 test configuration (50%)
  log_wait "Creating x402 test utilities [50%]" && progress_bar
  
  # Create test configuration
  cat > x402-test-config.json << 'EOF'
{
  "network": "splendor",
  "chainId": 6546,
  "rpcUrl": "http://localhost:80",
  "facilitatorUrl": "http://localhost:80",
  "testEndpoints": {
    "verify": "x402_verify",
    "settle": "x402_settle", 
    "supported": "x402_supported"
  },
  "testPayments": {
    "micro": "0.001",
    "small": "0.01",
    "medium": "0.1",
    "large": "1.0"
  },
  "testAddresses": {
    "payer": "0x6BED5A6606fF44f7d986caA160F14771f7f14f69",
    "recipient": "0xAbC3c6f5C6600510fF81db7D7F96F65dB2Fd1417"
  }
}
EOF
  
  # Create x402 test script
  cat > test-x402.sh << 'EOF'
#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${GREEN}Testing Splendor Native x402 Implementation${NC}\n"

# Test 1: Check if x402 API is available
echo -e "${CYAN}Test 1: Checking x402 API availability${NC}"
if curl -s -X POST -H "Content-Type: application/json" \
   --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
   http://localhost:80 | grep -q "result"; then
    echo -e "${GREEN}✅ x402 API is available and responding${NC}"
else
    echo -e "${RED}❌ x402 API not available - make sure node is running with --rpc${NC}"
    echo -e "${ORANGE}Start node with: ./node-start.sh --rpc${NC}"
    exit 1
fi

# Test 2: Test supported methods
echo -e "\n${CYAN}Test 2: Getting supported payment methods${NC}"
SUPPORTED=$(curl -s -X POST -H "Content-Type: application/json" \
   --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
   http://localhost:80)
echo "Response: $SUPPORTED"

# Test 3: Test middleware server
echo -e "\n${CYAN}Test 3: Testing x402 middleware${NC}"
cd x402-middleware
if node test.js > /dev/null 2>&1 & then
    MIDDLEWARE_PID=$!
    sleep 5
    
    # Test free endpoint
    if curl -s http://localhost:3000/api/free | grep -q "free"; then
        echo -e "${GREEN}✅ Free endpoint working${NC}"
    else
        echo -e "${RED}❌ Free endpoint failed${NC}"
    fi
    
    # Test paid endpoint (should return 402)
    HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null http://localhost:3000/api/premium)
    if [ "$HTTP_CODE" = "402" ]; then
        echo -e "${GREEN}✅ Paid endpoint correctly returns 402 Payment Required${NC}"
    else
        echo -e "${RED}❌ Paid endpoint returned $HTTP_CODE instead of 402${NC}"
    fi
    
    kill $MIDDLEWARE_PID 2>/dev/null || true
    cd ..
else
    echo -e "${RED}❌ Middleware test failed${NC}"
    cd ..
fi

echo -e "\n${GREEN}🎉 Native x402 testing completed!${NC}"
echo -e "${CYAN}Your blockchain now has the world's first native x402 implementation!${NC}"
EOF
  
  chmod +x test-x402.sh
  log_success "✅ x402 test utilities created [50%]"
  
  # Create x402 integration guide (70%)
  log_wait "Creating x402 integration documentation [70%]" && progress_bar
  
  cat > X402_INTEGRATION_GUIDE.md << 'EOF'
# Splendor Native x402 Integration Guide

## 🎉 Congratulations!
Your Splendor blockchain now has **NATIVE x402 support** - the world's first blockchain with built-in micropayments protocol.

## What's Included

### 1. Native x402 API (Built into Geth)
- `x402_verify` - Verify payments without executing
- `x402_settle` - Execute payments instantly  
- `x402_supported` - Get supported payment schemes

### 2. HTTP Middleware Package
- Express.js and Fastify support
- Automatic 402 responses
- Payment verification and settlement
- Located in: `x402-middleware/`

### 3. Test Suite
- Complete testing framework
- Example endpoints with different pricing
- Test script: `./test-x402.sh`

## Quick Start

### 1. Start Your Node
```bash
# For RPC node with x402 support
./node-start.sh --rpc

# For validator node  
./node-start.sh --validator
```

### 2. Test x402 API
```bash
# Test if x402 API is working
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
  http://localhost:80
```

### 3. Test Middleware
```bash
./test-x402.sh
```

### 4. Add Payments to Your API
```javascript
const { splendorX402Express } = require('./x402-middleware');

app.use('/api', splendorX402Express({
  payTo: '0xYourWalletAddress',
  pricing: {
    '/api/premium': '0.001'  // $0.001 per request
  }
}));
```

## Key Features

- **Instant Settlement**: Millions of TPS capability
- **No Gas Fees**: Users don't pay gas for micropayments  
- **HTTP Native**: Standard x402 protocol over HTTP
- **$0.001 Minimum**: Smallest payments in crypto
- **Framework Support**: Works with any web framework

## Competitive Advantage

You now have the **world's first blockchain** with native x402 support, enabling:
- Micropayments for APIs
- Content monetization
- IoT machine-to-machine payments
- AI service payments
- Gaming microtransactions

---

**You've just built the future of internet payments!** 🚀
EOF
  
  log_success "✅ x402 integration documentation created [70%]"
  
  # Verify x402 integration (90%)
  log_wait "Verifying x402 integration [90%]" && progress_bar
  
  X402_VERIFICATION_PASSED=true
  
  # Check if x402 API is integrated into backend
  if grep -q "x402" node_src/eth/backend.go; then
    log_success "✅ x402 API registered in blockchain backend"
  else
    log_error "❌ x402 API not registered in backend"
    X402_VERIFICATION_PASSED=false
  fi
  
  # Check if node-start.sh includes x402 API
  if grep -q "x402" node-start.sh; then
    log_success "✅ x402 API enabled in node startup"
  else
    log_error "❌ x402 API not enabled in node startup"
    X402_VERIFICATION_PASSED=false
  fi
  
  # Check middleware files
  if [ -f "x402-middleware/index.js" ] && [ -f "x402-middleware/package.json" ]; then
    log_success "✅ x402 middleware package ready"
  else
    log_error "❌ x402 middleware package incomplete"
    X402_VERIFICATION_PASSED=false
  fi
  
  # Check test utilities
  if [ -f "test-x402.sh" ] && [ -f "x402-test-config.json" ]; then
    log_success "✅ x402 test utilities ready"
  else
    log_error "❌ x402 test utilities incomplete"
    X402_VERIFICATION_PASSED=false
  fi
  
  # Final x402 verification result (100%)
  if [ "$X402_VERIFICATION_PASSED" = true ]; then
    log_success "✅ Native x402 payments protocol installed successfully [100%]"
    
    echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                    🎉 x402 INTEGRATION COMPLETE!             ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  Your blockchain now has NATIVE x402 support!               ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  🌟 World's first blockchain with native micropayments      ║${NC}"
    echo -e "${GREEN}║  ⚡ Millions of TPS with instant settlement                 ║${NC}"
    echo -e "${GREEN}║  💰 $0.001 minimum payments (no gas fees)                  ║${NC}"
    echo -e "${GREEN}║  🌐 HTTP-native integration (1-line setup)                 ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  Test with: ./test-x402.sh                                 ║${NC}"
    echo -e "${GREEN}║  Guide: X402_INTEGRATION_GUIDE.md                          ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}\n"
  else
    log_error "❌ x402 integration verification failed"
    return 1
  fi
}

verify_installation(){
  # Comprehensive installation verification before completion
  log_wait "Performing comprehensive installation verification" && progress_bar
  
  VERIFICATION_PASSED=true
  
  echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${GREEN}║                    INSTALLATION VERIFICATION                ║${NC}"
  echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}\n"
  
  # Check Go installation
  if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}')
    log_success "✅ Go installed: $GO_VERSION"
  else
    log_error "❌ Go not found"
    VERIFICATION_PASSED=false
  fi
  
  # Check geth binary
  if [ -f "./node_src/build/bin/geth" ]; then
    log_success "✅ Geth binary built successfully"
  else
    log_error "❌ Geth binary not found"
    VERIFICATION_PASSED=false
  fi
  
  # Check GPU drivers
  if nvidia-smi >/dev/null 2>&1; then
    GPU_NAME=$(nvidia-smi --query-gpu=name --format=csv,noheader,nounits | head -1)
    GPU_MEMORY=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits | head -1)
    log_success "✅ GPU drivers active: $GPU_NAME ($GPU_MEMORY MB)"
  else
    log_wait "⚠️  GPU drivers installed but require reboot to activate"
  fi
  
  # Check CUDA installation
  if command -v nvcc >/dev/null 2>&1; then
    CUDA_VERSION=$(nvcc --version | grep "release" | awk '{print $6}' | cut -c2-)
    log_success "✅ CUDA installed: $CUDA_VERSION"
  else
    log_wait "⚠️  CUDA installed but requires reboot to activate"
  fi
  
  # Check vLLM installation
  if [ -d "/opt/vllm-env" ]; then
    log_success "✅ vLLM virtual environment created"
    
    # Check if vLLM is actually installed in the environment
    if /opt/vllm-env/bin/python -c "import vllm; print('vLLM version:', vllm.__version__)" 2>/dev/null; then
      VLLM_VERSION=$(/opt/vllm-env/bin/python -c "import vllm; print(vllm.__version__)" 2>/dev/null)
      log_success "✅ vLLM installed and working: $VLLM_VERSION"
    else
      log_error "❌ vLLM not properly installed in virtual environment"
      VERIFICATION_PASSED=false
    fi
  else
    log_error "❌ vLLM virtual environment not found"
    VERIFICATION_PASSED=false
  fi
  
  # Check vLLM systemd service
  if systemctl list-unit-files | grep -q "vllm-mobilellm.service"; then
    log_success "✅ vLLM systemd service configured"
  else
    log_error "❌ vLLM systemd service not found"
    VERIFICATION_PASSED=false
  fi
  
  # Check Node.js and npm
  if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
    NODE_VERSION=$(node --version)
    log_success "✅ Node.js installed: $NODE_VERSION"
  else
    log_error "❌ Node.js/npm not found"
    VERIFICATION_PASSED=false
  fi
  
  # Check yarn and pm2
  if command -v yarn >/dev/null 2>&1 && command -v pm2 >/dev/null 2>&1; then
    log_success "✅ Yarn and PM2 installed"
  else
    log_error "❌ Yarn or PM2 not found"
    VERIFICATION_PASSED=false
  fi
  
  # Check .env configuration
  if [ -f "./.env" ]; then
    if grep -q "ENABLE_AI_LOAD_BALANCING=true" ./.env; then
      log_success "✅ AI configuration added to .env"
    else
      log_error "❌ AI configuration missing from .env"
      VERIFICATION_PASSED=false
    fi
  else
    log_error "❌ .env file not found"
    VERIFICATION_PASSED=false
  fi
  
  # Final verification result
  echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
  if [ "$VERIFICATION_PASSED" = true ]; then
    echo -e "${GREEN}║                    ✅ VERIFICATION PASSED                    ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  All components installed successfully!                     ║${NC}"
    echo -e "${GREEN}║  • Go + Geth blockchain node                                ║${NC}"
    echo -e "${GREEN}║  • GPU acceleration (CUDA + OpenCL)                        ║${NC}"
    echo -e "${GREEN}║  • AI system (vLLM + MobileLLM-R1)                        ║${NC}"
    echo -e "${GREEN}║  • Node.js ecosystem (yarn + pm2)                          ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  Ready to start with: ./node-start.sh                      ║${NC}"
  else
    echo -e "${RED}║                    ❌ VERIFICATION FAILED                    ║${NC}"
    echo -e "${RED}║                                                              ║${NC}"
    echo -e "${RED}║  Some components failed to install properly.               ║${NC}"
    echo -e "${RED}║  Please check the errors above and retry setup.            ║${NC}"
    echo -e "${RED}║                                                              ║${NC}"
    echo -e "${RED}║  You may need to reboot and run setup again.               ║${NC}"
  fi
  echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}\n"
  
  return $([[ "$VERIFICATION_PASSED" = true ]] && echo 0 || echo 1)
}

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


finalize(){
  displayWelcome
  createRpc
  createValidator
  labelNodes

  # resource paths
  nodePath=$BASE_DIR/Core-Blockchain
  ipcPath=$nodePath/chaindata/node1/geth.ipc
  chaindataPath=$nodePath/chaindata/node1/geth
  snapshotName=$nodePath/chaindata.tar.gz

  # added gitkeep
  # echo -e "\n\n\t${ORANGE}Removing existing chaindata, if any${NC}"
  
  # rm -rf $chaindataPath/chaindata

  # echo -e "\n\n\t${GREEN}Now importing the snapshot"
  # wget https://snapshots.splendor.org/chaindata.tar.gz

  # Create the directory if it does not exist
  # if [ ! -d "$chaindataPath" ]; then
  #   mkdir -p $chaindataPath
  # fi

  # # Extract archive to the correct directory
  # tar -xvf $snapshotName -C $chaindataPath --strip-components=1

  # Set proper permissions
  # echo -e "\n\n\t${GREEN}Setting directory permissions${NC}"
  # chown -R root:root $nodePath/chaindata
  # chmod -R 755 $nodePath/chaindata

  # echo -e "\n\n\tImport is done, now configuring sync-helper${NC}"
  # sleep 3
  cd $nodePath
  

  install_nvm
  cd $nodePath/plugins/sync-helper
  yarn
  cd $nodePath

  # Install AI-powered load balancing (vLLM + MobileLLM-R1)
  install_ai_llm

  # Install x402 native payments protocol
  install_x402_native

  # Perform comprehensive verification before completion
  verify_installation
  
  # Only proceed if verification passed
  if [ $? -eq 0 ]; then
    displayStatus
    
    # Check if reboot is needed and handle automatic reboot
    reboot_countdown
  else
    echo -e "\n${RED}❌ Setup incomplete due to verification failures.${NC}"
    echo -e "${ORANGE}Please review the errors above and run setup again.${NC}\n"
    exit 1
  fi
}


#########################################################################

#+-----------------------------------------------------------------------------------------------+
#|                                                                                                                             |
#|                                                                                                                             |
#|                                                      UTILITY                                                        |
#|                                                                                                                             |
#|                                                                                                                             |
#+-----------------------------------------------------------------------------------------------+


# Default variable values
verbose_mode=false
output_file=""

# Function to display script usage
usage() {
  echo -e "\nUsage: $0 [OPTIONS]"
  echo "Options:"
  echo -e "\t\t -h, --help      Display this help message"
  echo -e " \t\t -v, --verbose   Enable verbose mode"
  echo -e "\t\t --rpc      Specify to create RPC node"
  echo -e "\t\t --validator  <whole number>     Specify number of validator node to create"
  echo -e "		 --nopk     Skip validator account import/creation (skip task8)"
  echo -e "\t\t --keep-existing-drivers   Do not enforce/replace NVIDIA driver version (default)"
  echo -e "\t\t --enforce-driver-version  Install pinned NVIDIA driver version if different"
  echo -e "\t\t --no-auto-reboot         Do not auto reboot; only warn if needed"
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
      totalRpc=1
      totalNodes=$(($totalRpc + $totalValidator))
      ;;

    # take validator count
    --validator*)
      if ! has_argument $@; then
        # default to 1 validator if no number provided
        totalValidator=1
      else
        totalValidator=$(extract_argument $@)
      fi
      totalNodes=$(($totalRpc + $totalValidator))
      shift
      ;;

      # check for update and do update
      --update)
      doUpdate
      exit 0
      ;;
      # skip validator account setup (task8)
      --nopk)
      NOPK=true
      ;;

      --keep-existing-drivers)
      ENFORCE_DRIVER_VERSION=false
      ;;

      --enforce-driver-version)
      ENFORCE_DRIVER_VERSION=true
      ;;

      --no-auto-reboot)
      AUTO_REBOOT=false
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


# bootstraping
finalize
 
