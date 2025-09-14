# Splendor Blockchain Quantum Resistance Deployment Guide

## Overview

This guide provides step-by-step instructions for deploying quantum-resistant cryptography on the Splendor blockchain. The implementation uses ML-DSA (Dilithium) signatures as specified in FIPS 204 to protect against quantum computer attacks.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Building with Quantum Resistance](#building-with-quantum-resistance)
3. [Configuration](#configuration)
4. [Deployment Strategy](#deployment-strategy)
5. [Testing and Validation](#testing-and-validation)
6. [Monitoring and Maintenance](#monitoring-and-maintenance)
7. [Troubleshooting](#troubleshooting)
8. [Security Considerations](#security-considerations)

## Prerequisites

### System Requirements

- **Operating System**: Linux (Ubuntu 20.04+, CentOS 8+) or macOS 10.15+
- **Memory**: Minimum 8GB RAM (16GB recommended)
- **Storage**: At least 100GB free space
- **CPU**: x86_64 architecture with AES-NI support

### Software Dependencies

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build libssl-dev git curl

# CentOS/RHEL
sudo yum install -y gcc gcc-c++ cmake ninja-build openssl-devel git curl

# macOS
brew install cmake ninja openssl git curl
```

### Go Environment

- Go 1.18 or later
- CGO enabled (required for liboqs integration)

```bash
export CGO_ENABLED=1
go version  # Verify Go installation
```

## Building with Quantum Resistance

### 1. Clone and Setup

```bash
git clone https://github.com/your-org/splendor-blockchain-v4.git
cd splendor-blockchain-v4/Core-Blockchain/node_src
```

### 2. Build liboqs and Splendor

```bash
# Install dependencies and build liboqs
make -f Makefile.pq install-deps
make -f Makefile.pq liboqs

# Validate liboqs installation
make -f Makefile.pq validate-liboqs

# Build Splendor with quantum resistance
make -f Makefile.pq geth
```

### 3. Verify Build

```bash
# Check that geth was built successfully
./build/bin/geth version

# Run post-quantum tests
make -f Makefile.pq pq-test
```

## Configuration

### 1. Genesis Configuration

Create or update your genesis.json to include post-quantum configuration:

```json
{
  "config": {
    "chainId": 6546,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "clique": {
      "period": 15,
      "epoch": 30000
    },
    "postQuantum": {
      "pqtBlock": 1000000,
      "transitionBlocks": 7200,
      "enableMLDSAConsensus": true,
      "enableMLDSAPrecompiles": true,
      "defaultMLDSAAlgorithm": 65
    }
  },
  "difficulty": "0x1",
  "gasLimit": "0x8000000",
  "alloc": {}
}
```

### 2. Node Configuration

Update your node configuration to support post-quantum features:

```bash
# Start node with PQ support
./build/bin/geth \
  --datadir ./data \
  --networkid 6546 \
  --http \
  --http.api eth,net,web3,personal,admin,clique \
  --http.corsdomain "*" \
  --allow-insecure-unlock \
  --unlock 0x... \
  --password password.txt \
  --mine \
  --miner.etherbase 0x... \
  --verbosity 4
```

### 3. Post-Quantum Parameters

Key configuration parameters:

- **pqtBlock**: Block number where PQ transition begins
- **transitionBlocks**: Duration of dual-signing period (default: 7200 blocks ≈ 24 hours)
- **defaultMLDSAAlgorithm**: ML-DSA variant (44, 65, or 87)

## Deployment Strategy

### Phase 1: Pre-Deployment Testing

1. **Testnet Deployment**
   ```bash
   # Deploy on testnet first
   ./build/bin/geth --testnet --datadir ./testdata init genesis-testnet.json
   ./build/bin/geth --testnet --datadir ./testdata --mine
   ```

2. **Validator Coordination**
   - Notify all validators about the upgrade
   - Ensure all validators have PQ-enabled nodes ready
   - Coordinate the fork block number

3. **Smart Contract Testing**
   ```solidity
   // Test ML-DSA precompile
   contract PQTest {
       function testMLDSAVerify(
           bytes memory message,
           bytes memory signature,
           bytes memory publicKey
       ) public view returns (bool) {
           bytes memory input = abi.encodePacked(
               uint8(0x65), // ML-DSA-65
               uint32(message.length),
               uint32(signature.length),
               uint32(publicKey.length),
               message,
               signature,
               publicKey
           );
           
           (bool success, bytes memory result) = address(0x0100).staticcall(input);
           return success && result.length == 32 && result[31] == 0x01;
       }
   }
   ```

### Phase 2: Mainnet Deployment

1. **Set Fork Block**
   ```bash
   # Calculate appropriate fork block (e.g., 2 weeks in future)
   CURRENT_BLOCK=$(./build/bin/geth attach --exec "eth.blockNumber")
   FORK_BLOCK=$((CURRENT_BLOCK + 100800))  # ~2 weeks at 12s blocks
   echo "Set pqtBlock to: $FORK_BLOCK"
   ```

2. **Validator Upgrade**
   - All validators must upgrade before the fork block
   - Verify PQ support: `geth version` should show PQ build info

3. **Monitor Transition**
   ```bash
   # Monitor dual-signing period
   ./build/bin/geth attach --exec "
     var block = eth.getBlock('latest');
     console.log('Block:', block.number);
     console.log('Extra data length:', block.extraData.length);
   "
   ```

### Phase 3: Post-Quantum Enforcement

After the transition period:
- Only ML-DSA signatures are accepted
- ECDSA signatures are rejected
- Monitor for any consensus issues

## Testing and Validation

### 1. Unit Tests

```bash
# Run all PQ-related tests
make -f Makefile.pq pq-test

# Run specific test suites
go test -v ./crypto/mldsa/...
go test -v ./core/vm/... -run TestMLDSA
go test -v ./consensus/clique/... -run TestPQ
```

### 2. Integration Tests

```bash
# Test precompile functionality
go test -v ./core/vm/... -run TestPrecompile

# Test consensus with PQ signatures
go test -v ./consensus/clique/... -run TestConsensus
```

### 3. Performance Benchmarks

```bash
# Run performance benchmarks
make -f Makefile.pq pq-bench

# Profile memory and CPU usage
make -f Makefile.pq profile-pq
```

### 4. NIST Validation

```bash
# Download and run NIST test vectors (when available)
make -f Makefile.pq generate-test-vectors
```

## Monitoring and Maintenance

### 1. Key Metrics to Monitor

- **Block Production**: Ensure blocks are being produced normally
- **Signature Verification**: Monitor ML-DSA verification success rates
- **Memory Usage**: PQ signatures are larger, monitor memory consumption
- **Network Sync**: Ensure nodes can sync with PQ blocks

### 2. Logging Configuration

```bash
# Enable detailed PQ logging
./build/bin/geth --verbosity 4 --vmodule "clique=5,mldsa=5"
```

### 3. Health Checks

```bash
# Check node health
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545

# Verify PQ precompile
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x0100","data":"0x..."},"latest"],"id":1}' \
  http://localhost:8545
```

## Troubleshooting

### Common Issues

1. **liboqs Build Failures**
   ```bash
   # Clean and rebuild
   make -f Makefile.pq liboqs-clean
   make -f Makefile.pq install-deps
   make -f Makefile.pq liboqs
   ```

2. **CGO Compilation Errors**
   ```bash
   # Ensure CGO is enabled
   export CGO_ENABLED=1
   export CGO_CFLAGS="-I$(pwd)/crypto/liboqs/install/include"
   export CGO_LDFLAGS="-L$(pwd)/crypto/liboqs/install/lib -loqs"
   ```

3. **Large Block Headers**
   - ML-DSA signatures are ~3.3KB each
   - Monitor extraData size in blocks
   - Adjust gas limits if needed

4. **Sync Issues**
   ```bash
   # Clear cache and resync
   ./build/bin/geth removedb
   ./build/bin/geth init genesis.json
   ```

### Debug Commands

```bash
# Check PQ signature in block
./build/bin/geth attach --exec "
  var block = eth.getBlock(1000000);
  console.log('PQ signature present:', block.extraData.length > 97);
"

# Verify precompile availability
./build/bin/geth attach --exec "
  eth.call({to: '0x0100', data: '0x'}, 'latest')
"
```

## Security Considerations

### 1. Key Management

- **Validator Keys**: Generate new ML-DSA key pairs for validators
- **Secure Storage**: Use hardware security modules (HSMs) when possible
- **Key Rotation**: Plan for periodic key rotation

### 2. Cryptographic Validation

- **NIST Compliance**: Ensure ML-DSA implementation follows FIPS 204
- **Side-Channel Protection**: Use constant-time implementations
- **Random Number Generation**: Use cryptographically secure RNG

### 3. Network Security

- **TLS Upgrade**: Consider ML-KEM for TLS connections
- **Peer Authentication**: Verify PQ-enabled peers
- **Attack Monitoring**: Monitor for quantum-specific attacks

### 4. Incident Response

- **Rollback Plan**: Prepare for potential rollback scenarios
- **Emergency Contacts**: Maintain list of validator contacts
- **Communication Plan**: Prepare public communication templates

## Performance Impact

### Expected Changes

- **Signature Size**: ML-DSA-65 signatures are ~3.3KB vs 65 bytes for ECDSA
- **Verification Time**: ~2-5ms per signature vs ~0.1ms for ECDSA
- **Memory Usage**: Increased due to larger signatures
- **Network Bandwidth**: Higher due to larger block headers

### Optimization Tips

1. **Use ML-DSA-44** for less critical applications (smaller signatures)
2. **Batch Verification** when possible
3. **Caching** of verification results
4. **Network Compression** for block propagation

## Conclusion

The quantum-resistant upgrade provides future-proof security for the Splendor blockchain. Follow this guide carefully, test thoroughly on testnet, and coordinate with all network participants for a smooth transition.

For additional support, consult the technical documentation or contact the development team.

## References

- [NIST FIPS 204: ML-DSA Standard](https://csrc.nist.gov/pubs/fips/204/final)
- [Open Quantum Safe Project](https://openquantumsafe.org/)
- [ERC-4337: Account Abstraction](https://eips.ethereum.org/EIPS/eip-4337)
- [Splendor Blockchain Documentation](./docs/)
