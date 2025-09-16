# Quantum Resistance Guide

Splendor Blockchain V4 implements post-quantum cryptography using ML-DSA (FIPS 204) signatures via liboqs.

## Overview

Quantum-resistant cryptography protects against future quantum computer attacks that could break traditional ECDSA signatures. Splendor uses ML-DSA (Module-Lattice-Based Digital Signature Algorithm), the standardized version of Dilithium.

## Implementation

### ML-DSA Variants

- **ML-DSA-44** (Dilithium2): Fastest, smallest signatures
- **ML-DSA-65** (Dilithium3): Balanced security/performance
- **ML-DSA-87** (Dilithium5): Maximum security

### Code Structure

```
Core-Blockchain/node_src/crypto/mldsa/
├── mldsa_cgo.go      # CGO bindings to liboqs
├── mldsa_common.go   # Dynamic parameter handling
├── mldsa.go          # Fallback implementation
└── mldsa_test.go     # Comprehensive tests
```

## Quick Setup

The automated setup script handles everything:

```bash
sudo bash Core-Blockchain/node-setup.sh --rpc --validator 0 --nopk
```

This automatically:
- Downloads and builds liboqs v0.8.0
- Compiles ML-DSA support into geth
- Configures environment variables
- Runs integration tests

## Manual Build

If you need to build manually:

```bash
cd Core-Blockchain/node_src

# Build liboqs
make -f Makefile.pq liboqs

# Build with PQ support
make -f Makefile.pq all

# Run tests
make -f Makefile.pq pq-test
```

## Usage

### Precompile Contract

ML-DSA verification is available as a precompile contract:

```solidity
// Verify ML-DSA signature
function verifyMLDSA(
    bytes memory message,
    bytes memory signature,
    bytes memory publicKey,
    uint8 algorithm  // 44, 65, or 87
) public view returns (bool) {
    // Call precompile at address 0x09
    (bool success, bytes memory result) = address(0x09).staticcall(
        abi.encode(message, signature, publicKey, algorithm)
    );
    return success && abi.decode(result, (bool));
}
```

### JSON-RPC API

```bash
# Get supported algorithms
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"pq_getSupportedAlgorithms","params":[],"id":1}' \
  http://localhost:8545

# Verify signature
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"pq_verifySignature","params":[{
    "algorithm": "ML-DSA-65",
    "message": "0x48656c6c6f20576f726c64",
    "signature": "0x...",
    "publicKey": "0x..."
  }],"id":1}' \
  http://localhost:8545
```

## Algorithm Selection

### ML-DSA-44 (Dilithium2)
- **Use Case**: High-frequency transactions, IoT devices
- **Signature Size**: ~2,420 bytes
- **Public Key**: ~1,312 bytes
- **Performance**: Fastest signing/verification

### ML-DSA-65 (Dilithium3)
- **Use Case**: General purpose, recommended default
- **Signature Size**: ~3,293 bytes
- **Public Key**: ~1,952 bytes
- **Performance**: Balanced

### ML-DSA-87 (Dilithium5)
- **Use Case**: High-security applications, long-term storage
- **Signature Size**: ~4,595 bytes
- **Public Key**: ~2,592 bytes
- **Performance**: Highest security

## Performance Characteristics

| Algorithm | Sign (ops/sec) | Verify (ops/sec) | Signature Size |
|-----------|----------------|------------------|----------------|
| ML-DSA-44 | ~15,000 | ~45,000 | 2,420 bytes |
| ML-DSA-65 | ~10,000 | ~30,000 | 3,293 bytes |
| ML-DSA-87 | ~7,000 | ~20,000 | 4,595 bytes |

*Benchmarks on Intel i9-13900K*

## Security Considerations

### Quantum Threat Timeline
- **Current**: Classical computers cannot break ML-DSA
- **Near-term (5-10 years)**: Small quantum computers emerge
- **Long-term (10-20 years)**: Large-scale quantum computers possible

### Migration Strategy
1. **Hybrid Period**: Support both ECDSA and ML-DSA
2. **Gradual Transition**: Encourage ML-DSA adoption
3. **Full Migration**: Eventually deprecate ECDSA

### Best Practices
- Use ML-DSA-65 for most applications
- Implement proper key management
- Regular security audits
- Monitor quantum computing developments

## Integration Examples

### Web3 Integration

```javascript
// Using ethers.js with ML-DSA
const provider = new ethers.providers.JsonRpcProvider('http://localhost:8545');

// Verify ML-DSA signature
async function verifyMLDSASignature(message, signature, publicKey) {
    const result = await provider.send('pq_verifySignature', [{
        algorithm: 'ML-DSA-65',
        message: ethers.utils.hexlify(ethers.utils.toUtf8Bytes(message)),
        signature: signature,
        publicKey: publicKey
    }]);
    return result;
}
```

### Smart Contract Integration

```solidity
pragma solidity ^0.8.0;

contract QuantumSafeContract {
    mapping(address => bytes) public mldsaPublicKeys;
    
    function registerMLDSAKey(bytes memory publicKey) external {
        mldsaPublicKeys[msg.sender] = publicKey;
    }
    
    function verifyQuantumSafeMessage(
        bytes memory message,
        bytes memory signature,
        address signer
    ) public view returns (bool) {
        bytes memory publicKey = mldsaPublicKeys[signer];
        require(publicKey.length > 0, "No ML-DSA key registered");
        
        return verifyMLDSA(message, signature, publicKey, 65);
    }
}
```

## Troubleshooting

### Build Issues

**liboqs not found:**
```bash
# Rebuild liboqs
cd Core-Blockchain/node_src
make -f Makefile.pq clean
make -f Makefile.pq liboqs
```

**CGO errors:**
```bash
# Check environment
echo $CGO_CFLAGS
echo $CGO_LDFLAGS

# Verify liboqs installation
ls -la /tmp/splendor_liboqs/
```

### Runtime Issues

**Algorithm not supported:**
- Ensure liboqs was built with ML-DSA support
- Check available algorithms: `pq_getSupportedAlgorithms`

**Signature verification fails:**
- Verify algorithm parameter matches key/signature
- Check message encoding (hex vs bytes)
- Ensure public key format is correct

## Future Enhancements

- **Hardware Acceleration**: GPU-accelerated ML-DSA operations
- **Hybrid Signatures**: ECDSA + ML-DSA for transition period
- **Key Rotation**: Automated quantum-safe key rotation
- **Standards Compliance**: Track NIST PQC standardization updates

For detailed technical implementation, see the [Unified Architecture Specification](SPLENDOR_UNIFIED_QUANTUM_X402_GPU_TPS_CONSENSUS.md).
