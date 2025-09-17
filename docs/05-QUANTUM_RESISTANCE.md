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

### Quick Precompile Sanity Check (eth_call)

After starting the node via `Core-Blockchain/node-start.sh`, you can call the ML‑DSA precompile (0x0100) directly to confirm wiring:

```bash
bash Core-Blockchain/scripts/test-pq-precompile.sh
```

This sends a minimal header-only payload (alg=ML‑DSA‑65, zero lengths) and expects a 32‑byte zero result (false), proving the precompile is reachable. Use real message/signature/public key to expect a true result.

## Usage

### Precompile Contract

ML‑DSA verification is exposed as an EVM precompile at address `0x0100`.

Important:
- The input format is not a standard ABI tuple; it is a compact header + payload:
  - `[algorithm_id(1)] + [message_len(4)] + [signature_len(4)] + [pubkey_len(4)] + [message] + [signature] + [publicKey]`
- For a quick sanity check, prefer an `eth_call` using the provided script:

```bash
bash Core-Blockchain/scripts/test-pq-precompile.sh
```

This confirms the precompile is wired and callable. For Solidity integration, add a helper that assembles the exact byte layout and performs a `staticcall` to `address(0x0100)`.

### JSON-RPC

There are no `pq_*` JSON‑RPC methods exposed by the client. Use the precompile via `eth_call` (see the script in Core-Blockchain/scripts/test-pq-precompile.sh) or integrate at Solidity level with a helper that assembles the byte layout and `staticcall`s `0x0100`.

## Algorithm Selection

### ML-DSA-44 (Dilithium2)
- **Use Case**: High-frequency transactions, IoT devices
- **Signature Size**: ~2,420 bytes
- **Public Key**: ~1,312 bytes
- **Performance**: Fastest signing/verification

### ML-DSA-65 (Dilithium3)
- **Use Case**: General purpose, recommended default
- **Signature Size**: 3,309 bytes
- **Public Key**: 1,952 bytes
- **Performance**: Balanced

### ML-DSA-87 (Dilithium5)
- **Use Case**: High-security applications, long-term storage
- **Signature Size**: 4,627 bytes
- **Public Key**: 2,592 bytes
- **Performance**: Highest security

## Performance Characteristics

| Algorithm | Sign (ops/sec) | Verify (ops/sec) | Signature Size |
|-----------|----------------|------------------|----------------|
| ML-DSA-44 | ~15,000 | ~45,000 | 2,420 bytes |
| ML-DSA-65 | ~10,000 | ~30,000 | 3,309 bytes |
| ML-DSA-87 | ~7,000  | ~20,000 | 4,627 bytes |

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

### Web3 Integration (eth_call)

Using ethers.js to call the precompile (0x0100) with the compact header+payload format:

```javascript
import { ethers } from 'ethers'

const provider = new ethers.providers.JsonRpcProvider('http://localhost:8545')

function buildMLDSAInput(algId, messageBytes, sigBytes, pkBytes) {
  const header = ethers.utils.concat([
    ethers.utils.hexlify([algId]),                                       // 1 byte
    ethers.utils.hexZeroPad(ethers.utils.hexlify(messageBytes.length), 4),
    ethers.utils.hexZeroPad(ethers.utils.hexlify(sigBytes.length), 4),
    ethers.utils.hexZeroPad(ethers.utils.hexlify(pkBytes.length), 4),
  ])
  return ethers.utils.hexConcat([header, messageBytes, sigBytes, pkBytes])
}

async function verifyMLDSA(message, signature, publicKey, algId = 0x65) {
  const data = buildMLDSAInput(algId, message, signature, publicKey)
  const call = { to: '0x0000000000000000000000000000000000000100', data }
  const res = await provider.call(call, 'latest')
  // 32-byte boolean: 0x...01 means true
  return res.endsWith('01')
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
- For Go integrations, use `mldsa.IsMLDSASupported(algorithm)`; for precompile, a mismatch will simply return false.

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
