#!/bin/bash

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
NC='\033[0m' # No Color
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
BASE_DIR="/root/splendor-blockchain-v4"
CORE_DIR="$BASE_DIR/Core-Blockchain"
TMP_DIR="/root/tmp"

#########################################################################
totalRpc=0
totalValidator=0
totalNodes=$(($totalRpc + $totalValidator))

isRPC=false
isValidator=false
node_type=""

#########################################################################

#+-----------------------------------------------------------------------------------------------+
#|                                                                                               |
#|                                           FUNCTIONS                                           |
#|                                                                                               |
#+-----------------------------------------------------------------------------------------------+

# Logger setup
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

log_warning() {
  echo -e "${YELLOW}⚠ $1${NC}"
}

progress_bar() {
  echo -en "${CYAN}["
  for i in {1..60}; do
    echo -en "#"
    sleep 0.01
  done
  echo -e "]${NC}"
}

# Check if we're in the correct directory and detect node type
check_environment_and_node_type() {
  log_step "Checking environment and detecting node type"
  
  if [ ! -d "$BASE_DIR" ]; then
    log_error "Splendor blockchain directory not found at $BASE_DIR"
    exit 1
  fi
  
  if [ ! -d "$CORE_DIR" ]; then
    log_error "Core blockchain directory not found at $CORE_DIR"
    exit 1
  fi
  
  # Check if node1 directory exists
  if [ ! -d "$CORE_DIR/chaindata/node1" ]; then
    log_error "No node1 directory found. Cannot determine node type."
    exit 1
  fi
  
  # Detect node type by checking for .validator or .rpc files
  if [ -f "$CORE_DIR/chaindata/node1/.validator" ]; then
    isValidator=true
    node_type="validator"
    log_success "Detected VALIDATOR node"
  elif [ -f "$CORE_DIR/chaindata/node1/.rpc" ]; then
    isRPC=true
    node_type="rpc"
    log_success "Detected RPC node"
  else
    log_error "Cannot determine node type. Neither .validator nor .rpc file found in node1 directory."
    exit 1
  fi
  
  log_success "Environment check passed"
}

# Create tmp directory and backup validator files
backup_validator_files() {
  if [ "$isValidator" = true ]; then
    log_step "Backing up validator files"
    
    # Create tmp directory if it doesn't exist
    if [ ! -d "$TMP_DIR" ]; then
      log_wait "Creating tmp directory at $TMP_DIR"
      mkdir -p "$TMP_DIR"
      log_success "Tmp directory created"
    fi
    
    # Copy keystore directory
    if [ -d "$CORE_DIR/chaindata/node1/keystore" ]; then
      log_wait "Backing up keystore directory"
      cp -r "$CORE_DIR/chaindata/node1/keystore" "$TMP_DIR/"
      log_success "Keystore directory backed up"
    else
      log_warning "Keystore directory not found"
    fi
    
    # Copy pass.txt file
    if [ -f "$CORE_DIR/chaindata/node1/pass.txt" ]; then
      log_wait "Backing up pass.txt file"
      cp "$CORE_DIR/chaindata/node1/pass.txt" "$TMP_DIR/"
      log_success "pass.txt file backed up"
    else
      log_warning "pass.txt file not found"
    fi
    
    # Copy .validator or .rpc file
    if [ -f "$CORE_DIR/chaindata/node1/.validator" ]; then
      log_wait "Backing up .validator file"
      cp "$CORE_DIR/chaindata/node1/.validator" "$TMP_DIR/"
      log_success ".validator file backed up"
    elif [ -f "$CORE_DIR/chaindata/node1/.rpc" ]; then
      log_wait "Backing up .rpc file"
      cp "$CORE_DIR/chaindata/node1/.rpc" "$TMP_DIR/"
      log_success ".rpc file backed up"
    fi
    
    log_success "Validator files backed up successfully"
  else
    log_step "Skipping backup (RPC node detected)"
  fi
}

# Stop running nodes
stop_nodes() {
  log_step "Stopping running nodes"
  
  cd "$CORE_DIR"
  
  if [ "$isValidator" = true ]; then
    log_wait "Stopping validator nodes"
    if [ -f "node-stop.sh" ]; then
      bash node-stop.sh --validator || log_warning "Failed to stop validator nodes gracefully"
    else
      log_error "node-stop.sh not found"
      exit 1
    fi
  elif [ "$isRPC" = true ]; then
    log_wait "Stopping RPC nodes"
    if [ -f "node-stop.sh" ]; then
      bash node-stop.sh --rpc || log_warning "Failed to stop RPC nodes gracefully"
    else
      log_error "node-stop.sh not found"
      exit 1
    fi
  fi
  
  log_success "Nodes stopped successfully"
}

# Remove old splendor-blockchain-v4 directory
remove_old_directory() {
  log_step "Removing old splendor-blockchain-v4 directory"
  
  cd /root/
  
  if [ -d "splendor-blockchain-v4" ]; then
    log_wait "Removing splendor-blockchain-v4 directory"
    rm -rf splendor-blockchain-v4
    log_success "Old directory removed"
  else
    log_warning "splendor-blockchain-v4 directory not found"
  fi
}

# Clone fresh repository
clone_repository() {
  log_step "Cloning fresh repository"
  
  cd /root/
  
  log_wait "Cloning https://github.com/Splendor-Protocol/splendor-blockchain-v4.git"
  if git clone https://github.com/Splendor-Protocol/splendor-blockchain-v4.git; then
    log_success "Repository cloned successfully"
  else
    log_error "Failed to clone repository"
    exit 1
  fi
}

# Setup node based on type
setup_node() {
  log_step "Setting up node based on detected type"
  
  cd "$CORE_DIR"
  
  if [ "$isValidator" = true ]; then
    log_wait "Setting up validator node with --nopk flag"
    if [ -f "node-setup.sh" ]; then
      bash node-setup.sh --validator 1 --nopk || {
        log_error "Failed to setup validator node"
        exit 1
      }
    else
      log_error "node-setup.sh not found"
      exit 1
    fi
  elif [ "$isRPC" = true ]; then
    log_wait "Setting up RPC node"
    if [ -f "node-setup.sh" ]; then
      bash node-setup.sh --rpc || {
        log_error "Failed to setup RPC node"
        exit 1
      }
    else
      log_error "node-setup.sh not found"
      exit 1
    fi
  fi
  
  log_success "Node setup completed"
}

# Restore validator files
restore_validator_files() {
  if [ "$isValidator" = true ]; then
    log_step "Restoring validator files"
    
    # Restore keystore directory
    if [ -d "$TMP_DIR/keystore" ]; then
      log_wait "Restoring keystore directory"
      cp -r "$TMP_DIR/keystore" "$CORE_DIR/chaindata/node1/"
      log_success "Keystore directory restored"
    else
      log_warning "Keystore backup not found"
    fi
    
    # Restore pass.txt file
    if [ -f "$TMP_DIR/pass.txt" ]; then
      log_wait "Restoring pass.txt file"
      cp "$TMP_DIR/pass.txt" "$CORE_DIR/chaindata/node1/"
      log_success "pass.txt file restored"
    else
      log_warning "pass.txt backup not found"
    fi
    
    # Restore .validator or .rpc file
    if [ -f "$TMP_DIR/.validator" ]; then
      log_wait "Restoring .validator file"
      cp "$TMP_DIR/.validator" "$CORE_DIR/chaindata/node1/"
      log_success ".validator file restored"
    elif [ -f "$TMP_DIR/.rpc" ]; then
      log_wait "Restoring .rpc file"
      cp "$TMP_DIR/.rpc" "$CORE_DIR/chaindata/node1/"
      log_success ".rpc file restored"
    fi
    
    log_success "Validator files restored successfully"
  else
    log_step "Skipping restore (RPC node detected)"
  fi
}

# Cleanup and remove update files
cleanup_and_rename() {
  log_step "Cleaning up and removing files"
  
  cd /root/
  
  # Removing tmp directory
  if [ -d "$TMP_DIR" ]; then
    log_wait "Removing tmp directory"
    rm -rf "$TMP_DIR"
    log_success "tmp directory removed"
  fi
  
  # Remove update.sh
  if [ -f "/root/update.sh" ]; then
    log_wait "Removing update.sh"
    rm "/root/update.sh"
    log_success "update.sh removed"
  fi
  
  log_success "Cleanup completed"
}

# Display usage information
usage() {
  echo -e "\n${GREEN}Splendor Blockchain Update Script v2.0${NC}"
  echo -e "\nUsage: $0 [OPTIONS]"
  echo -e "\nOptions:"
  echo -e "\t-h, --help          Display this help message"
  echo -e "\nDescription:"
  echo -e "\tThis script automatically detects if the node is a validator or RPC,"
  echo -e "\tbacks up necessary files, updates the blockchain, and restores the files."
  echo -e "\nExample:"
  echo -e "\t$0                  # Run the update process"
}

# Parse command line arguments
handle_options() {
  while [ $# -gt 0 ]; do
    case $1 in
      -h | --help)
        usage
        exit 0
        ;;
      *)
        echo -e "${RED}Invalid option: $1${NC}" >&2
        usage
        exit 1
        ;;
    esac
    shift
  done
}

# Main execution function
main() {
  echo -e "${GREEN}"
  echo "╔══════════════════════════════════════════════════════════════╗"
  echo "║                 Splendor Blockchain Updater v2.0             ║"
  echo "║                    Auto Node Type Detection                  ║"
  echo "╚══════════════════════════════════════════════════════════════╝"
  echo -e "${NC}\n"
  
  check_environment_and_node_type
  backup_validator_files
  stop_nodes
  remove_old_directory
  clone_repository
  setup_node
  restore_validator_files
  cleanup_and_rename
  
  echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════╗"
  echo "║                    Update Complete!                          ║"
  echo "║              Node Type: $(printf "%-8s" "$node_type")                        ║"
  echo "╚══════════════════════════════════════════════════════════════╝${NC}"
  
  log_success "Splendor blockchain updated successfully"
  log_step "Now starting your $node_type node"
  cd $CORE_DIR
  ./node-start.sh --$node_type 
  cd $CORE_DIR
}

# Script execution
handle_options "$@"
main
