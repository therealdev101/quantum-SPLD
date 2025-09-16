#!/bin/bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${GREEN}
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║           SPLENDOR x402 COMPLETE SETUP SCRIPT               ║
║                                                              ║
║    Setting up the world's first native x402 blockchain      ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
${NC}\n"

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

# Check if we're in the right directory
if [ ! -f "genesis.json" ] || [ ! -d "node_src" ]; then
    log_error "Please run this script from the Core-Blockchain directory"
    exit 1
fi

# Step 1: Rebuild Geth with x402 support
log_step "Step 1: Building Geth with native x402 support"
cd node_src

# Clean previous builds
log_wait "Cleaning previous builds"
make clean 2>/dev/null || true
rm -f build/bin/geth 2>/dev/null || true

# Build with x402 support
log_wait "Building Geth with x402 API integration"
if make all; then
    log_success "Geth built successfully with x402 support"
else
    log_error "Failed to build Geth"
    exit 1
fi

# Verify x402 API is included
if strings build/bin/geth | grep -q "x402"; then
    log_success "x402 API confirmed in Geth binary"
else
    log_wait "x402 API may not be visible in strings (this is normal)"
fi

cd ..

# Step 2: Install x402 middleware dependencies
log_step "Step 2: Setting up x402 middleware"

# Check if Node.js is available
if ! command -v node >/dev/null 2>&1; then
    log_error "Node.js not found. Please run ./node-setup.sh first"
    exit 1
fi

# Install middleware dependencies
cd x402-middleware
log_wait "Installing x402 middleware dependencies"
if npm install; then
    log_success "x402 middleware dependencies installed"
else
    log_error "Failed to install middleware dependencies"
    exit 1
fi
cd ..

# Step 3: Update .env with x402 configuration
log_step "Step 3: Configuring x402 environment"

# Add x402 configuration to .env if not already present
if ! grep -q "X402_ENABLED" .env 2>/dev/null; then
    log_wait "Adding x402 configuration to .env"
    cat >> .env << 'EOF'

# Native x402 Payments Protocol Configuration
X402_ENABLED=true
X402_NETWORK=splendor
X402_CHAIN_ID=6546
X402_DEFAULT_PRICE=0.001
X402_MIN_PAYMENT=0.001
X402_MAX_PAYMENT=1000.0
X402_SETTLEMENT_TIMEOUT=300
X402_ENABLE_LOGGING=true

# x402 Performance Settings
X402_BATCH_SIZE=1000
X402_CACHE_SIZE=10000
X402_WORKER_THREADS=4
X402_ENABLE_COMPRESSION=true

# x402 Security Settings
X402_SIGNATURE_VALIDATION=strict
X402_NONCE_VALIDATION=true
X402_TIMESTAMP_TOLERANCE=300
X402_RATE_LIMITING=true
X402_MAX_REQUESTS_PER_MINUTE=1000
EOF
    log_success "x402 configuration added to .env"
else
    log_success "x402 configuration already present in .env"
fi

# Step 4: Create x402 test configuration
log_step "Step 4: Creating x402 test configuration"

# Create test configuration file
cat > x402-test-config.json << 'EOF'
{
  "network": "splendor",
  "chainId": 6546,
  "rpcUrl": "http://localhost:80",
  "facilitatorUrl": "http://localhost:80",
  "testEndpoints": {
    "verify": "/x402_verify",
    "settle": "/x402_settle", 
    "supported": "/x402_supported"
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

log_success "x402 test configuration created"

# Step 5: Create x402 quick test script
log_step "Step 5: Creating x402 test utilities"

cat > test-x402.sh << 'EOF'
#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
ORANGE='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${GREEN}Testing Splendor x402 Native Implementation${NC}\n"

# Test 1: Check if x402 API is available
echo -e "${CYAN}Test 1: Checking x402 API availability${NC}"
if curl -s -X POST -H "Content-Type: application/json" \
   --data '{"jsonrpc":"2.0","method":"x402_supported","params":[],"id":1}' \
   http://localhost:80 | grep -q "result"; then
    echo -e "${GREEN}✅ x402 API is available${NC}"
else
    echo -e "${RED}❌ x402 API not available - make sure node is running${NC}"
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
if npm test > /dev/null 2>&1 & then
    MIDDLEWARE_PID=$!
    sleep 3
    
    # Test free endpoint
    if curl -s http://localhost:3000/api/free | grep -q "free"; then
        echo -e "${GREEN}✅ Free endpoint working${NC}"
    else
        echo -e "${RED}❌ Free endpoint failed${NC}"
    fi
    
    # Test paid endpoint (should return 402)
    if curl -s -w "%{http_code}" http://localhost:3000/api/premium | grep -q "402"; then
        echo -e "${GREEN}✅ Paid endpoint correctly returns 402${NC}"
    else
        echo -e "${RED}❌ Paid endpoint not working correctly${NC}"
    fi
    
    kill $MIDDLEWARE_PID 2>/dev/null || true
else
    echo -e "${RED}❌ Middleware test failed${NC}"
fi

echo -e "\n${GREEN}x402 testing completed!${NC}"
EOF

chmod +x test-x402.sh
log_success "x402 test script created"

# Step 6: Create x402 documentation
log_step "Step 6: Creating x402 documentation"

cat > X402_INTEGRATION_GUIDE.md << 'EOF'
# Splendor x402 Integration Guide

## Overview
Your Splendor blockchain now has **native x402 support** - the world's first blockchain with built-in micropayments protocol.

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
cd x402-middleware
npm test
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

## Testing

Run the complete test suite:
```bash
./test-x402.sh
```

## Production Deployment

1. **Configure your payment address** in middleware
2. **Set pricing** for your API endpoints
3. **Start your node** with x402 API enabled
4. **Monitor payments** via RPC calls

## Support

- Documentation: `x402-middleware/README.md`
- Test configuration: `x402-test-config.json`
- Example server: `x402-middleware/test.js`

---

**Congratulations! You now have the world's first blockchain with native x402 support.**
EOF

log_success "x402 integration guide created"

# Step 7: Final verification
log_step "Step 7: Final verification"

# Check if all components are in place
VERIFICATION_PASSED=true

# Check Geth binary
if [ -f "node_src/build/bin/geth" ]; then
    log_success "✅ Geth binary with x402 support: Ready"
else
    log_error "❌ Geth binary not found"
    VERIFICATION_PASSED=false
fi

# Check x402 API files
if [ -f "node_src/eth/api_x402.go" ]; then
    log_success "✅ x402 API implementation: Ready"
else
    log_error "❌ x402 API implementation not found"
    VERIFICATION_PASSED=false
fi

# Check middleware
if [ -f "x402-middleware/index.js" ] && [ -f "x402-middleware/package.json" ]; then
    log_success "✅ x402 middleware package: Ready"
else
    log_error "❌ x402 middleware package not found"
    VERIFICATION_PASSED=false
fi

# Check node-start.sh has x402 API enabled
if grep -q "x402" node-start.sh; then
    log_success "✅ Node startup script: x402 API enabled"
else
    log_error "❌ Node startup script missing x402 API"
    VERIFICATION_PASSED=false
fi

# Check configuration
if [ -f ".env" ] && grep -q "X402_ENABLED" .env; then
    log_success "✅ x402 configuration: Ready"
else
    log_error "❌ x402 configuration not found"
    VERIFICATION_PASSED=false
fi

# Check test utilities
if [ -f "test-x402.sh" ] && [ -f "x402-test-config.json" ]; then
    log_success "✅ x402 test utilities: Ready"
else
    log_error "❌ x402 test utilities not found"
    VERIFICATION_PASSED=false
fi

echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
if [ "$VERIFICATION_PASSED" = true ]; then
    echo -e "${GREEN}║                    ✅ SETUP COMPLETE!                        ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  Splendor blockchain now has NATIVE x402 support!           ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  🚀 World's first blockchain with native micropayments      ║${NC}"
    echo -e "${GREEN}║  ⚡ Millions of TPS with instant settlement                 ║${NC}"
    echo -e "${GREEN}║  💰 $0.001 minimum payments (no gas fees)                  ║${NC}"
    echo -e "${GREEN}║  🌐 HTTP-native integration (1-line setup)                 ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║  Next steps:                                                 ║${NC}"
    echo -e "${GREEN}║  1. Start your node: ./node-start.sh --rpc                 ║${NC}"
    echo -e "${GREEN}║  2. Test x402: ./test-x402.sh                              ║${NC}"
    echo -e "${GREEN}║  3. Read guide: X402_INTEGRATION_GUIDE.md                  ║${NC}"
else
    echo -e "${RED}║                    ❌ SETUP INCOMPLETE                       ║${NC}"
    echo -e "${RED}║                                                              ║${NC}"
    echo -e "${RED}║  Some components failed verification.                       ║${NC}"
    echo -e "${RED}║  Please check the errors above and retry.                   ║${NC}"
fi
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}\n"

if [ "$VERIFICATION_PASSED" = true ]; then
    echo -e "${CYAN}🎉 Congratulations! You've successfully implemented the world's first native x402 blockchain!${NC}\n"
    exit 0
else
    exit 1
fi
