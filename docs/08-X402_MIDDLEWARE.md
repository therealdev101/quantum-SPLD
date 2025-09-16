# x402 Middleware Integration Guide

Learn how to integrate Splendor's native x402 micropayments into your web applications.

## Overview

The x402 middleware provides seamless integration of HTTP-native micropayments into Express.js and Fastify applications. It automatically handles payment verification, settlement, and error responses.

## Quick Setup

### Installation

```bash
cd Core-Blockchain/x402-middleware
npm install
```

### Basic Express.js Integration

```javascript
const express = require('express');
const { x402Middleware } = require('./x402-middleware');

const app = express();

// Configure x402 middleware
app.use('/api/premium', x402Middleware({
    rpcUrl: 'http://localhost:80',
    pricePerRequest: '0.001', // SPLD tokens
    network: 'splendor-mainnet'
}));

// Protected endpoint
app.get('/api/premium/data', (req, res) => {
    res.json({ 
        message: 'This is premium content!',
        data: 'Exclusive information...'
    });
});

app.listen(3000);
```

### Basic Fastify Integration

```javascript
const fastify = require('fastify')({ logger: true });
const { x402FastifyPlugin } = require('./x402-middleware');

// Register x402 plugin
fastify.register(x402FastifyPlugin, {
    rpcUrl: 'http://localhost:80',
    pricePerRequest: '0.001',
    network: 'splendor-mainnet'
});

// Protected route
fastify.get('/api/premium/data', {
    preHandler: fastify.x402Required
}, async (request, reply) => {
    return { 
        message: 'This is premium content!',
        data: 'Exclusive information...'
    };
});

fastify.listen(3000);
```

## Configuration Options

### Middleware Configuration

```javascript
const x402Config = {
    // Required
    rpcUrl: 'http://localhost:80',           // Splendor RPC endpoint
    pricePerRequest: '0.001',                // Price in SPLD tokens
    
    // Optional
    network: 'splendor-mainnet',             // Network identifier
    timeout: 30000,                          // Payment timeout (ms)
    retryAttempts: 3,                        // Retry failed payments
    cacheTimeout: 300000,                    // Cache valid payments (ms)
    
    // Custom handlers
    onPaymentSuccess: (req, payment) => {    // Payment success callback
        console.log('Payment verified:', payment.txHash);
    },
    onPaymentFailure: (req, error) => {      // Payment failure callback
        console.log('Payment failed:', error.message);
    },
    
    // Rate limiting
    rateLimitWindow: 60000,                  // Rate limit window (ms)
    rateLimitMax: 100,                       // Max requests per window
    
    // Custom pricing
    dynamicPricing: (req) => {               // Dynamic pricing function
        if (req.path.includes('/premium')) return '0.002';
        return '0.001';
    }
};
```

### Environment Variables

```bash
# .env file
X402_RPC_URL=http://localhost:80
X402_DEFAULT_PRICE=0.001
X402_NETWORK=splendor-mainnet
X402_TIMEOUT=30000
X402_CACHE_TIMEOUT=300000
```

## Payment Flow

### 1. Client Request (No Payment)

```bash
curl -X GET http://localhost:3000/api/premium/data
```

**Response (402 Payment Required):**
```json
{
    "error": "Payment Required",
    "code": 402,
    "payment": {
        "amount": "0.001",
        "currency": "SPLD",
        "recipient": "0x742d35Cc6634C0532925a3b8D4C9db96590b5b8c",
        "network": "splendor-mainnet",
        "methods": ["x402-native", "metamask"]
    }
}
```

### 2. Client Payment

```javascript
// Using Web3/ethers.js
const payment = await wallet.sendTransaction({
    to: "0x742d35Cc6634C0532925a3b8D4C9db96590b5b8c",
    value: ethers.utils.parseEther("0.001"),
    data: "0x" // Optional payment data
});

const txHash = payment.hash;
```

### 3. Client Request (With Payment)

```bash
curl -X GET http://localhost:3000/api/premium/data \
  -H "X-Payment-Hash: 0xabc123..." \
  -H "X-Payment-Network: splendor-mainnet"
```

**Response (Success):**
```json
{
    "message": "This is premium content!",
    "data": "Exclusive information..."
}
```

## Advanced Usage

### Custom Payment Verification

```javascript
const x402Config = {
    rpcUrl: 'http://localhost:80',
    pricePerRequest: '0.001',
    
    // Custom verification logic
    customVerification: async (req, paymentHash) => {
        // Get transaction details
        const tx = await web3.eth.getTransaction(paymentHash);
        
        // Custom validation logic
        if (tx.value < web3.utils.toWei('0.001', 'ether')) {
            throw new Error('Insufficient payment amount');
        }
        
        // Additional checks
        if (tx.to !== expectedRecipient) {
            throw new Error('Invalid payment recipient');
        }
        
        return {
            valid: true,
            amount: tx.value,
            sender: tx.from,
            timestamp: Date.now()
        };
    }
};
```

### Tiered Pricing

```javascript
const x402Config = {
    rpcUrl: 'http://localhost:80',
    
    // Dynamic pricing based on endpoint
    dynamicPricing: (req) => {
        const pricingTiers = {
            '/api/basic': '0.0001',
            '/api/premium': '0.001',
            '/api/enterprise': '0.01'
        };
        
        for (const [path, price] of Object.entries(pricingTiers)) {
            if (req.path.startsWith(path)) {
                return price;
            }
        }
        
        return '0.001'; // Default price
    }
};
```

### Subscription Model

```javascript
const subscriptions = new Map();

const x402Config = {
    rpcUrl: 'http://localhost:80',
    
    // Check for active subscription
    customVerification: async (req, paymentHash) => {
        const userAddress = req.headers['x-user-address'];
        
        // Check for active subscription
        const subscription = subscriptions.get(userAddress);
        if (subscription && subscription.expiresAt > Date.now()) {
            return {
                valid: true,
                type: 'subscription',
                expiresAt: subscription.expiresAt
            };
        }
        
        // Verify payment for new subscription
        const tx = await web3.eth.getTransaction(paymentHash);
        const subscriptionPrice = web3.utils.toWei('0.1', 'ether'); // Monthly
        
        if (tx.value >= subscriptionPrice) {
            // Create 30-day subscription
            const expiresAt = Date.now() + (30 * 24 * 60 * 60 * 1000);
            subscriptions.set(tx.from, { expiresAt });
            
            return {
                valid: true,
                type: 'new_subscription',
                expiresAt
            };
        }
        
        throw new Error('Invalid subscription payment');
    }
};
```

## Client-Side Integration

### JavaScript/Web3 Client

```javascript
class X402Client {
    constructor(rpcUrl, walletProvider) {
        this.rpcUrl = rpcUrl;
        this.web3 = new Web3(walletProvider);
    }
    
    async makePayment(recipient, amount) {
        const accounts = await this.web3.eth.getAccounts();
        const tx = await this.web3.eth.sendTransaction({
            from: accounts[0],
            to: recipient,
            value: this.web3.utils.toWei(amount, 'ether')
        });
        return tx.transactionHash;
    }
    
    async requestWithPayment(url, options = {}) {
        try {
            // Try request without payment first
            const response = await fetch(url, options);
            
            if (response.status === 402) {
                // Payment required
                const paymentInfo = await response.json();
                
                // Make payment
                const txHash = await this.makePayment(
                    paymentInfo.payment.recipient,
                    paymentInfo.payment.amount
                );
                
                // Retry request with payment proof
                const paidResponse = await fetch(url, {
                    ...options,
                    headers: {
                        ...options.headers,
                        'X-Payment-Hash': txHash,
                        'X-Payment-Network': paymentInfo.payment.network
                    }
                });
                
                return paidResponse;
            }
            
            return response;
        } catch (error) {
            console.error('Payment request failed:', error);
            throw error;
        }
    }
}

// Usage
const client = new X402Client('http://localhost:80', window.ethereum);

client.requestWithPayment('/api/premium/data')
    .then(response => response.json())
    .then(data => console.log(data));
```

### React Hook

```javascript
import { useState, useCallback } from 'react';
import { useWeb3React } from '@web3-react/core';

export function useX402Payment() {
    const { library, account } = useWeb3React();
    const [isPaymentPending, setIsPaymentPending] = useState(false);
    
    const makePaymentRequest = useCallback(async (url, options = {}) => {
        setIsPaymentPending(true);
        
        try {
            // Try request without payment
            let response = await fetch(url, options);
            
            if (response.status === 402) {
                const paymentInfo = await response.json();
                
                // Make payment using Web3
                const tx = await library.getSigner().sendTransaction({
                    to: paymentInfo.payment.recipient,
                    value: ethers.utils.parseEther(paymentInfo.payment.amount)
                });
                
                await tx.wait(); // Wait for confirmation
                
                // Retry with payment proof
                response = await fetch(url, {
                    ...options,
                    headers: {
                        ...options.headers,
                        'X-Payment-Hash': tx.hash,
                        'X-Payment-Network': paymentInfo.payment.network
                    }
                });
            }
            
            return response;
        } finally {
            setIsPaymentPending(false);
        }
    }, [library]);
    
    return { makePaymentRequest, isPaymentPending };
}
```

## Testing

### Running Tests

```bash
cd Core-Blockchain/x402-middleware
npm test
```

### Test Configuration

```javascript
// test.js
const { x402Middleware } = require('./index');
const request = require('supertest');
const express = require('express');

describe('x402 Middleware', () => {
    let app;
    
    beforeEach(() => {
        app = express();
        app.use('/paid', x402Middleware({
            rpcUrl: 'http://localhost:80',
            pricePerRequest: '0.001'
        }));
        app.get('/paid/content', (req, res) => {
            res.json({ message: 'Premium content' });
        });
    });
    
    it('should return 402 for unpaid requests', async () => {
        const response = await request(app)
            .get('/paid/content')
            .expect(402);
            
        expect(response.body.error).toBe('Payment Required');
        expect(response.body.payment.amount).toBe('0.001');
    });
    
    it('should allow access with valid payment', async () => {
        const response = await request(app)
            .get('/paid/content')
            .set('X-Payment-Hash', 'valid_tx_hash')
            .set('X-Payment-Network', 'splendor-mainnet')
            .expect(200);
            
        expect(response.body.message).toBe('Premium content');
    });
});
```

## Production Deployment

### Security Considerations

```javascript
const x402Config = {
    rpcUrl: process.env.X402_RPC_URL,
    pricePerRequest: process.env.X402_DEFAULT_PRICE,
    
    // Security settings
    requireHttps: true,                      // Require HTTPS in production
    validateOrigin: true,                    // Validate request origin
    allowedOrigins: ['https://myapp.com'],   // Allowed origins
    
    // Rate limiting
    rateLimitWindow: 60000,
    rateLimitMax: 100,
    
    // Logging
    logPayments: true,
    logFailures: true,
    
    // Monitoring
    onPaymentSuccess: (req, payment) => {
        // Log to monitoring service
        analytics.track('payment_success', {
            amount: payment.amount,
            txHash: payment.txHash,
            endpoint: req.path
        });
    }
};
```

### Load Balancing

```javascript
// Multiple RPC endpoints for redundancy
const x402Config = {
    rpcUrls: [
        'https://rpc1.splendor.org',
        'https://rpc2.splendor.org',
        'https://rpc3.splendor.org'
    ],
    rpcTimeout: 5000,
    rpcRetries: 2
};
```

## Troubleshooting

### Common Issues

**Payment verification fails:**
- Check RPC endpoint connectivity
- Verify transaction hash format
- Ensure sufficient confirmations

**High latency:**
- Use multiple RPC endpoints
- Implement payment caching
- Optimize verification logic

**Rate limiting issues:**
- Adjust rate limit settings
- Implement user-based limits
- Use Redis for distributed rate limiting

For more examples and advanced usage, see the middleware source code and tests in `Core-Blockchain/x402-middleware/`.
