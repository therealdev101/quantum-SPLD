/**
 * Test script for Splendor x402 Native Payments Middleware
 */

const express = require('express');
const { splendorX402Express } = require('./index.js');

// Create test server
const app = express();
app.use(express.json());

// Configure x402 middleware
const x402Middleware = splendorX402Express({
  payTo: '0x6BED5A6606fF44f7d986caA160F14771f7f14f69', // Test address from genesis
  rpcUrl: 'http://localhost:80',
  pricing: {
    '/api/premium': '0.001',    // $0.001 for premium endpoint
    '/api/data/*': '0.01',      // $0.01 for data endpoints
    '/api/free': '0'            // Free endpoint
  },
  defaultPrice: '0.005'         // Default $0.005 for other endpoints
});

// Apply middleware to all routes
app.use('/api', x402Middleware);

// Test endpoints
app.get('/api/free', (req, res) => {
  res.json({ 
    message: 'This is a free endpoint!',
    paid: req.x402?.paid || false
  });
});

app.get('/api/premium', (req, res) => {
  res.json({ 
    message: 'This is premium content!',
    payment: req.x402,
    data: {
      secret: 'Only paid users see this',
      timestamp: Date.now()
    }
  });
});

app.get('/api/data/analytics', (req, res) => {
  res.json({ 
    message: 'Analytics data',
    payment: req.x402,
    analytics: {
      users: 1000,
      revenue: '$50,000',
      growth: '25%'
    }
  });
});

app.get('/api/other', (req, res) => {
  res.json({ 
    message: 'Other endpoint with default pricing',
    payment: req.x402,
    info: 'This costs the default amount'
  });
});

// Health check endpoint (no payment required)
app.get('/health', (req, res) => {
  res.json({ status: 'OK', timestamp: Date.now() });
});

// Start server
const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`🚀 Splendor x402 Test Server running on port ${PORT}`);
  console.log('');
  console.log('Test endpoints:');
  console.log(`  GET http://localhost:${PORT}/health - Free health check`);
  console.log(`  GET http://localhost:${PORT}/api/free - Free API endpoint`);
  console.log(`  GET http://localhost:${PORT}/api/premium - $0.001 premium content`);
  console.log(`  GET http://localhost:${PORT}/api/data/analytics - $0.01 analytics data`);
  console.log(`  GET http://localhost:${PORT}/api/other - $0.005 default pricing`);
  console.log('');
  console.log('To test payments:');
  console.log('1. First call any paid endpoint without X-Payment header');
  console.log('2. You\'ll get a 402 response with payment requirements');
  console.log('3. Create payment signature and include in X-Payment header');
  console.log('4. Call endpoint again with payment header');
  console.log('');
  console.log('Example test:');
  console.log(`curl http://localhost:${PORT}/api/premium`);
});

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n👋 Shutting down Splendor x402 test server...');
  process.exit(0);
});
