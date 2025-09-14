# Splendor Blockchain Quantum Resistance Implementation Plan

## Overview
This document outlines the implementation plan to make the Splendor blockchain quantum-resistant using ML-DSA (Dilithium) signatures as specified in FIPS 204. The implementation follows the developer-ready plan provided and focuses on consensus, precompiles, and account abstraction.

## Current System Analysis
- **Base**: Geth (go-ethereum) fork
- **Consensus**: Clique PoA with custom Congress consensus
- **Chain ID**: 6546 (mainnet), 256 (testnet)
- **Current Crypto**: secp256k1/ECDSA signatures
- **Precompiles**: Standard Ethereum precompiles (ecrecover, sha256, etc.)

## Implementation Strategy

### Phase 1: Core Cryptographic Infrastructure
1. **Integrate liboqs library** for ML-DSA support
2. **Add ML-DSA verify precompile** at address 0x0100
3. **Create quantum-resistant crypto utilities**

### Phase 2: Consensus Layer Modifications
1. **Modify Clique consensus** for dual-signing transition
2. **Update block headers** to support PQ signatures
3. **Implement transition mechanism** (ECDSA + ML-DSA → ML-DSA only)

### Phase 3: Account Layer (Account Abstraction)
1. **Deploy ERC-4337 infrastructure**
2. **Create PQ wallet smart contracts**
3. **Implement ML-DSA signature verification**

### Phase 4: Network and Testing
1. **Optional TLS ML-KEM integration**
2. **Comprehensive testing suite**
3. **NIST ACVP/KAT validation**

## Technical Specifications

### Cryptographic Parameters
- **Consensus signatures**: ML-DSA-65 (validator keys)
- **User signatures**: ML-DSA-44 (size-optimized)
- **Fallback**: SLH-DSA (SPHINCS+) for long-lived keys
- **Network encryption**: ML-KEM-768 (TLS)
- **Hash functions**: Keep Keccak/SHA3-256

### Fork Configuration
- **PQT Fork Block**: TBD (Post-Quantum Transition)
- **Transition Period**: Dual-signing window (ECDSA + ML-DSA)
- **Post-transition**: ML-DSA only

### Precompile Specification
- **Address**: 0x0100
- **ABI**: `mldsaverify(bytes message, bytes signature, bytes pubkey) returns (bool)`
- **Gas Model**: Base 15,000 + 3 * (|sig| + |pk| + |msg|)

## Implementation Files Structure

```
Core-Blockchain/node_src/
├── crypto/
│   ├── mldsa/           # ML-DSA implementation
│   ├── pq/              # Post-quantum utilities
│   └── liboqs/          # liboqs integration
├── core/vm/
│   └── contracts_pq.go  # PQ precompiles
├── consensus/clique/
│   └── clique_pq.go     # PQ consensus modifications
├── params/
│   └── pq_config.go     # PQ fork configuration
└── accounts/abi/bind/
    └── pq_wallet.go     # PQ wallet utilities
```

## Next Steps
1. Set up liboqs integration and build system
2. Implement ML-DSA verify precompile
3. Modify Clique consensus for dual-signing
4. Create comprehensive test suite
5. Deploy on testnet for validation

## Security Considerations
- Dual-signing transition prevents replay attacks
- ML-DSA parameter validation against NIST specs
- Secure key generation and storage
- Backward compatibility during transition

## Timeline Estimate
- **Phase 1**: 2-3 weeks (crypto infrastructure)
- **Phase 2**: 3-4 weeks (consensus modifications)
- **Phase 3**: 2-3 weeks (account abstraction)
- **Phase 4**: 2-3 weeks (testing and validation)
- **Total**: 9-13 weeks for complete implementation
