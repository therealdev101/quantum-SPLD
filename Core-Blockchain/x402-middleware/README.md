# Splendor x402 Native Payments Middleware

The world's first **native x402 implementation** built directly into a blockchain. Ultra-fast micropayments with **millions of TPS** capability.

## 🚀 Features

- **Native Integration**: Built directly into Splendor blockchain core
- **Ultra-Fast**: Millions of TPS with instant settlement
- **No Gas Fees**: Users don't pay gas for micropayments
- **HTTP Native**: Standard x402 protocol over HTTP
- **Framework Support**: Express.js, Fastify, and more
- **$0.001 Minimum**: Smallest payments in crypto
- **💎 Validator Revenue**: 5% of all payments go to validators
- **🏦 Protocol Revenue**: 5% of all payments go to core blockchain

## 💰 Revenue Model

### **Automatic Revenue Split (Per Payment)**

```
User Payment: $0.001 SPLD
├── API Provider: $0.0009 SPLD (90%) ← YOU (THE DEVELOPER)
├── Validators: $0.00005 SPLD (5%) ← NETWORK VALIDATORS
└── Core Blockchain: $0.00005 SPLD (5%) ← PROTOCOL FEE
```

**You keep 90% of all payments to your API!**

## 📦 Installation

```bash
# Copy from Splendor blockchain
cp -r /path/to/Core-Blockchain/x402-middleware ./
cd x402-middleware
npm install
```

## 🔧 Quick Start

### Express.js

```javascript
const express = require('express');
const { splendorX402Express } = require('./x402-middleware');

const app = express();

// Add x402 payments to your API in 1 line
app.use('/api', splendorX402Express({
  payTo: '0xYourWalletAddress',        // You get 90% of payments
  rpcUrl: 'http://splendor-rpc:80',    // Splendor RPC endpoint
  pricing: {
    '/api/weather': '0.001',           // $0.001 per weather request
    '/api/premium': '0.01',            // $0.01 for premium data
    '/api/analytics': '0.05',          // $0.05 for analytics
    '/api/free': '0'                   // Free endpoint
  }
}));

// These endpoints now require payment
app.get('/api/weather', (req, res) => {
  res.json({ 
    weather: 'Sunny, 75°F',
    payment: req.x402,  // Payment details
    revenue: 'You earned $0.0009 from this request!'
  });
});

app.get('/api/premium', (req, res) => {
  res.json({ 
    data: 'Premium content here',
    payment: req.x402,
    revenue: 'You earned $0.009 from this request!'
  });
});

app.listen(3000);
```

### Fastify

```javascript
const fastify = require('fastify')();
const { splendorX402Fastify } = require('./x402-middleware');

// Register x402 plugin
fastify.register(splendorX402Fastify, {
  payTo: '0xYourWalletAddress',
  rpcUrl: 'http://splendor-rpc:80',
  pricing: {
    '/api/premium': '0.001'
  }
});

fastify.get('/api/premium', async (request, reply) => {
  return { 
    message: 'Premium content!',
    payment: request.x402,
    revenue: 'You earned 90% of this payment!'
  };
});

fastify.listen(3000);
```

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Client App    │    │  Your API Server │    │ Splendor Chain  │
│                 │    │                  │    │                 │
│ 1. Request API  │───▶│ 2. Check Payment │    │                 │
│ 2. Get 402      │◀───│ 3. Return 402    │    │                 │
│ 3. Sign Payment │    │                  │    │                 │
│ 4. Send Payment │───▶│ 5. Verify & Settle──▶│ 6. Instant TX   │
│ 5. Get Content  │◀───│ 6. Return Content│◀───│ 7. Revenue Split│
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                │ 90% → You       │
                                                │ 5% → Validators │
                                                │ 5% → Protocol   │
                                                └─────────────────┘
```

## 💰 Complete Payment Flow

### 1. Client Makes Request (No Payment)
```bash
curl http://localhost:3000/api/premium
```

**Response: 402 Payment Required**
```json
{
  "x402Version": 1,
  "accepts": [{
    "scheme": "exact",
    "network": "splendor",
    "maxAmountRequired": "0x8ac7230489e80000",
    "resource": "/api/premium",
    "payTo": "0xYourWalletAddress",
    "asset": "0x0000000000000000000000000000000000000000"
  }]
}
```

### 2. Client Creates Payment Signature (No EIP-3009!)
```javascript
// Simple message signing (much easier than EIP-3009)
const payment = {
  x402Version: 1,
  scheme: "exact",
  network: "splendor",
  payload: {
    from: "0xClientAddress",
    to: "0xYourWalletAddress", 
    value: "0x8ac7230489e80000", // 0.001 SPLD in wei
    validAfter: Math.floor(Date.now() / 1000),
    validBefore: Math.floor(Date.now() / 1000) + 3600,
    nonce: "0x" + crypto.randomBytes(32).toString('hex'),
    signature: "0x..." // Simple signature, not EIP-3009
  }
};
```

### 3. Client Sends Payment
```bash
curl -H "X-Payment: $(echo $PAYMENT | base64)" \
     http://localhost:3000/api/premium
```

**Response: 200 OK + Content + Revenue Split**
```json
{
  "message": "Premium content!",
  "payment": {
    "paid": true,
    "amount": "0.001",
    "txHash": "0x...",
    "payer": "0xClientAddress"
  }
}
```

**What happens behind the scenes:**
- ✅ **You get**: $0.0009 (90%)
- ✅ **Validator gets**: $0.00005 (5%)
- ✅ **Protocol gets**: $0.00005 (5%)

## ⚙️ Configuration Options

```javascript
const middleware = splendorX402Express({
  // Required
  payTo: '0xYourWalletAddress',        // Where YOUR 90% goes
  
  // Optional
  rpcUrl: 'http://localhost:80',       // Splendor RPC endpoint
  network: 'splendor',                 // Network name
  chainId: 6546,                       // Splendor chain ID
  defaultPrice: '0.001',               // Default price in USD
  
  // Pricing rules
  pricing: {
    '/api/free': '0',                  // Free endpoint
    '/api/premium': '0.001',           // $0.001 per request
    '/api/data/*': '0.01',             // $0.01 for wildcard paths
    '/api/analytics': '0.05'           // $0.05 for analytics
  }
});
```

## 🧪 Testing

### **1. Start Splendor Node**
```bash
cd Core-Blockchain
./node-start.sh --rpc
```

### **2. Install Dependencies**
```bash
cd x402-middleware
npm install
```

### **3. Run Test Server**
```bash
npm test
```

### **4. Test Endpoints**
```bash
# Free endpoint (no payment required)
curl http://localhost:3000/api/free

# Paid endpoint (returns 402 Payment Required)
curl http://localhost:3000/api/premium

# Health check
curl http://localhost:3000/health
```

## 🔗 Client Integration Examples

### JavaScript/Node.js Client
```javascript
const axios = require('axios');
const crypto = require('crypto');

// Create payment signature (simplified - no EIP-3009!)
function createPayment(from, to, amount) {
  return {
    x402Version: 1,
    scheme: "exact", 
    network: "splendor",
    payload: {
      from, to, 
      value: amount,
      validAfter: Math.floor(Date.now() / 1000),
      validBefore: Math.floor(Date.now() / 1000) + 3600,
      nonce: "0x" + crypto.randomBytes(32).toString('hex'),
      signature: "0x..." // Sign with wallet (simple message signing)
    }
  };
}

// Make paid request
async function paidRequest(url, payment) {
  const paymentHeader = Buffer.from(JSON.stringify(payment)).toString('base64');
  
  const response = await axios.get(url, {
    headers: { 'X-Payment': paymentHeader }
  });
  
  return response.data;
}

// Usage
const payment = createPayment(userAddress, apiProviderAddress, "0.001");
const result = await paidRequest('http://api.example.com/premium', payment);
```

### Python Client
```python
import requests
import json
import base64
import hashlib
import time

def create_payment(from_addr, to_addr, amount):
    return {
        "x402Version": 1,
        "scheme": "exact",
        "network": "splendor",
        "payload": {
            "from": from_addr,
            "to": to_addr,
            "value": amount,
            "validAfter": int(time.time()),
            "validBefore": int(time.time()) + 3600,
            "nonce": "0x" + hashlib.sha256(str(time.time()).encode()).hexdigest(),
            "signature": "0x..."  # Sign with wallet
        }
    }

def paid_request(url, payment):
    payment_header = base64.b64encode(
        json.dumps(payment).encode()
    ).decode()
    
    response = requests.get(url, headers={
        'X-Payment': payment_header
    })
    
    return response.json()

# Usage
payment = create_payment(user_address, api_provider_address, "0.001")
result = paid_request('http://api.example.com/premium', payment)
```

## 🌟 Why Splendor x402 is Better

| Feature | **Splendor x402** | Standard x402 | Credit Cards |
|---------|------------------|---------------|--------------|
| **Settlement** | **Instant** | 2+ seconds | 2-3 days |
| **Minimum** | **$0.001** | $0.001 | $0.50+ |
| **Fees** | **None** | Gas fees | 2.9% + $0.30 |
| **TPS** | **Millions** | ~50,000 | ~65,000 |
| **Integration** | **1 line** | Multiple steps | Complex |
| **Revenue Share** | **90% to you** | Variable | ~97% to you |
| **EIP-3009** | **Not needed** | Required | N/A |

## 📚 API Reference

### Middleware Options

- `payTo` (string, required): Your wallet address (receives 90%)
- `rpcUrl` (string): Splendor RPC endpoint (default: 'http://localhost:80')
- `network` (string): Network name (default: 'splendor')
- `chainId` (number): Chain ID (default: 6546)
- `pricing` (object): Path-to-price mapping
- `defaultPrice` (string): Default price in USD (default: '0.001')

### Request Object Extensions

After successful payment, requests include:
```javascript
req.x402 = {
  paid: true,                    // Payment successful
  amount: "0.001",              // Amount paid (USD)
  txHash: "0x...",              // Transaction hash
  payer: "0x...",               // Payer address
  yourRevenue: "0.0009"         // Your 90% share
}
```

### Response Headers

Successful payments include:
```
X-Payment-Response: eyJzdWNjZXNzIjp0cnVlLCJ0eEhhc2giOiIweDEyMyIsIm5ldHdvcmtJZCI6InNwbGVuZG9yIn0=
```

## 🚀 Production Deployment

### 1. Configure Your Splendor Node
```bash
# Start with RPC enabled
./node-start.sh --rpc --http.addr 0.0.0.0 --http.port 80
```

### 2. Set Up Load Balancer
```nginx
upstream splendor_rpc {
    server rpc1.yourdomain.com:80;
    server rpc2.yourdomain.com:80;
    server rpc3.yourdomain.com:80;
}

server {
    listen 80;
    location / {
        proxy_pass http://splendor_rpc;
    }
}
```

### 3. Environment Variables
```bash
export SPLENDOR_RPC_URL=http://your-load-balancer:80
export PAYMENT_ADDRESS=0xYourDeveloperAddress
export NODE_ENV=production
```

## 💡 Real-World Examples

### **Weather API Service**
```javascript
app.use('/weather', splendorX402Express({
  payTo: '0xWeatherCompanyWallet',
  pricing: { '/weather/*': '0.001' }  // $0.001 per weather request
}));

// Revenue: 1000 requests/day = $0.90/day = $27/month
```

### **AI Image Generator**
```javascript
app.use('/generate', splendorX402Express({
  payTo: '0xAICompanyWallet',
  pricing: { '/generate/image': '0.05' }  // $0.05 per image
}));

// Revenue: 100 images/day = $4.50/day = $135/month
```

### **Data Analytics Platform**
```javascript
app.use('/analytics', splendorX402Express({
  payTo: '0xAnalyticsCompanyWallet',
  pricing: { '/analytics/report': '0.10' }  // $0.10 per report
}));

// Revenue: 50 reports/day = $4.50/day = $135/month
```

## 🤖 No EIP-3009 Complexity!

**Your users don't need to understand EIP-3009 or complex crypto:**

### **Standard x402 (Complex):**
```javascript
// Users need to understand EIP-3009, gas fees, etc.
const authorization = {
  from: userAddress,
  to: recipientAddress,
  value: amount,
  validAfter: timestamp,
  validBefore: timestamp + 3600,
  nonce: randomNonce
};
const signature = await wallet.signTypedData(EIP3009_DOMAIN, EIP3009_TYPES, authorization);
```

### **Splendor x402 (Simple):**
```javascript
// Users just sign a simple message
const message = `x402-payment:${from}:${to}:${amount}:${validAfter}:${validBefore}:${nonce}`;
const signature = await wallet.signMessage(message);
```

**Much easier for users and developers!**

## 📊 Revenue Tracking

### **Monitor Your Earnings**
```bash
# Check your wallet balance (your 90% share)
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0xYourWalletAddress","latest"],"id":1}' \
  http://splendor-rpc:80

# Get x402 payment statistics
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"x402_getRevenueStats","params":[],"id":1}' \
  http://splendor-rpc:80
```

## 🎯 Use Cases

### **API Monetization**
- **Weather APIs**: $0.001 per request
- **Stock Data**: $0.01 per quote
- **News Articles**: $0.005 per article
- **Maps/Directions**: $0.002 per route

### **AI Services**
- **Image Generation**: $0.05 per image
- **Text Generation**: $0.01 per request
- **Voice Synthesis**: $0.02 per audio file
- **Translation**: $0.001 per word

### **Data Services**
- **Analytics Reports**: $0.10 per report
- **Database Queries**: $0.001 per query
- **File Storage**: $0.001 per MB
- **CDN Access**: $0.0001 per file

### **Content & Media**
- **Premium Articles**: $0.01 per article
- **Video Streaming**: $0.05 per hour
- **Music Streaming**: $0.001 per song
- **E-books**: $0.50 per book

## ⚙️ Configuration Options

```javascript
const middleware = splendorX402Express({
  // Required
  payTo: '0xYourWalletAddress',        // Where YOUR 90% goes
  
  // Optional
  rpcUrl: 'http://localhost:80',       // Splendor RPC endpoint
  network: 'splendor',                 // Network name
  chainId: 6546,                       // Splendor chain ID
  defaultPrice: '0.001',               // Default price in USD
  
  // Pricing rules (flexible patterns)
  pricing: {
    '/api/free': '0',                  // Free endpoint
    '/api/premium': '0.001',           // Fixed price
    '/api/data/*': '0.01',             // Wildcard pattern
    '/api/analytics': '0.05',          // Higher value content
    '/api/bulk/*': '0.0001'            // Bulk pricing
  }
});
```

## 🧪 Testing Your Implementation

### **1. Start Splendor Node**
```bash
cd Core-Blockchain
./node-start.sh --rpc
```

### **2. Install Dependencies**
```bash
cd x402-middleware
npm install
```

### **3. Run Test Server**
```bash
npm test
```

### **4. Test Different Endpoints**
```bash
# Free endpoint (no payment required)
curl http://localhost:3000/api/free
# Returns: {"message":"This is a free endpoint!","paid":false}

# Paid endpoint (returns 402 Payment Required)
curl http://localhost:3000/api/premium
# Returns: 402 with payment requirements

# Health check
curl http://localhost:3000/health
# Returns: {"status":"OK","timestamp":...}
```

## 🌟 Why Choose Splendor x402?

### **For Developers:**
- **90% revenue share** (you keep most of the money)
- **1-line integration** (add payments instantly)
- **No crypto complexity** (HTTP-native)
- **Instant settlement** (no waiting for confirmations)
- **No gas fees** for users (better user experience)

### **For Users:**
- **Tiny payments** ($0.001 minimum)
- **No gas fees** (just pay for the service)
- **Instant access** (no waiting)
- **Simple signing** (no EIP-3009 complexity)
- **HTTP-native** (works with any app)

### **vs Competition:**

| Feature | **Splendor x402** | Standard x402 | Credit Cards |
|---------|------------------|---------------|--------------|
| **Your Revenue** | **90%** | Variable | ~97% |
| **Settlement** | **Instant** | 2+ seconds | 2-3 days |
| **Minimum** | **$0.001** | $0.001 | $0.50+ |
| **User Fees** | **None** | Gas fees | None |
| **Integration** | **1 line** | Multiple steps | Complex |
| **Crypto Knowledge** | **None needed** | EIP-3009 required | None |

## 📚 API Reference

### Middleware Options

- `payTo` (string, required): Your wallet address (receives 90%)
- `rpcUrl` (string): Splendor RPC endpoint
- `network` (string): Network name (default: 'splendor')
- `chainId` (number): Chain ID (default: 6546)
- `pricing` (object): Path-to-price mapping
- `defaultPrice` (string): Default price in USD

### Request Object Extensions

After successful payment:
```javascript
req.x402 = {
  paid: true,                    // Payment successful
  amount: "0.001",              // Amount paid (USD)
  txHash: "0x...",              // Transaction hash
  payer: "0x...",               // Payer address
  yourRevenue: "0.0009",        // Your 90% share
  validatorRevenue: "0.00005",  // Validator 5% share
  protocolRevenue: "0.00005"    // Protocol 5% share
}
```

## 🚀 Production Deployment

### 1. Configure Your Splendor Node
```bash
# Start with RPC enabled
./node-start.sh --rpc --http.addr 0.0.0.0 --http.port 80
```

### 2. Set Up Load Balancer
```nginx
upstream splendor_rpc {
    server rpc1.yourdomain.com:80;
    server rpc2.yourdomain.com:80;
    server rpc3.yourdomain.com:80;
}

server {
    listen 80;
    location / {
        proxy_pass http://splendor_rpc;
    }
}
```

### 3. Environment Variables
```bash
export SPLENDOR_RPC_URL=http://your-load-balancer:80
export PAYMENT_ADDRESS=0xYourDeveloperAddress
export NODE_ENV=production
```

## 🎊 Ready to Monetize Your API!

**With Splendor x402, you can:**
- ✅ **Add payments to any API** in 1 line of code
- ✅ **Keep 90% of all revenue** (best rate in crypto)
- ✅ **No gas fees** for your users (better experience)
- ✅ **Instant settlement** (millions of TPS)
- ✅ **No EIP-3009 complexity** (simple message signing)

**Start earning from your APIs today!**

---

**Built with ❤️ by the Splendor team**

*The first blockchain to make micropayments practical for developers.*
