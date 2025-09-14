// Copyright 2024 The Splendor Authors
// This file contains post-quantum cryptography configuration parameters

package params

import "math/big"

// Post-quantum fork configuration
var (
	// PQTForkBlock defines the block number where post-quantum transition begins
	// During this fork, dual-signing (ECDSA + ML-DSA) is required
	PQTForkBlock *big.Int

	// PQTTransitionBlocks defines the number of blocks for the transition period
	// After this period, only ML-DSA signatures are accepted
	PQTTransitionBlocks = uint64(7200) // ~24 hours at 12s block time

	// PQTEnforceBlock defines when ML-DSA-only mode begins
	// Calculated as PQTForkBlock + PQTTransitionBlocks
	PQTEnforceBlock *big.Int
)

// Post-quantum precompile gas costs
const (
	// ML-DSA signature verification base gas cost
	MLDSAVerifyBaseGas uint64 = 15000

	// ML-DSA signature verification per-byte gas cost
	MLDSAVerifyPerByteGas uint64 = 3

	// ML-KEM key encapsulation base gas cost (for future use)
	MLKEMEncapsBaseGas uint64 = 8000

	// ML-KEM key encapsulation per-byte gas cost (for future use)
	MLKEMEncapsPerByteGas uint64 = 2

	// SLH-DSA signature verification base gas cost (for future use)
	SLHDSAVerifyBaseGas uint64 = 25000

	// SLH-DSA signature verification per-byte gas cost (for future use)
	SLHDSAVerifyPerByteGas uint64 = 5
)

// Post-quantum algorithm identifiers for precompiles
const (
	// ML-DSA algorithm IDs
	MLDSA44_ID byte = 0x44
	MLDSA65_ID byte = 0x65
	MLDSA87_ID byte = 0x87

	// ML-KEM algorithm IDs (for future use)
	MLKEM512_ID byte = 0x12
	MLKEM768_ID byte = 0x18
	MLKEM1024_ID byte = 0x24

	// SLH-DSA algorithm IDs (for future use)
	SLHDSA128S_ID byte = 0x81
	SLHDSA128F_ID byte = 0x82
	SLHDSA192S_ID byte = 0x91
	SLHDSA192F_ID byte = 0x92
	SLHDSA256S_ID byte = 0xA1
	SLHDSA256F_ID byte = 0xA2
)

// PostQuantumConfig represents post-quantum cryptography configuration
type PostQuantumConfig struct {
	// Fork block where PQ transition begins
	PQTBlock *big.Int `json:"pqtBlock,omitempty"`

	// Transition period in blocks
	TransitionBlocks uint64 `json:"transitionBlocks,omitempty"`

	// Enable ML-DSA for consensus
	EnableMLDSAConsensus bool `json:"enableMLDSAConsensus,omitempty"`

	// Enable ML-DSA precompiles
	EnableMLDSAPrecompiles bool `json:"enableMLDSAPrecompiles,omitempty"`

	// Enable ML-KEM for networking (future use)
	EnableMLKEMNetworking bool `json:"enableMLKEMNetworking,omitempty"`

	// Default ML-DSA algorithm for consensus (44, 65, or 87)
	DefaultMLDSAAlgorithm int `json:"defaultMLDSAAlgorithm,omitempty"`
}

// String returns the string representation of PostQuantumConfig
func (pq *PostQuantumConfig) String() string {
	if pq == nil {
		return "post-quantum disabled"
	}
	return "post-quantum enabled"
}

// Helper functions for post-quantum fork detection

// IsPQTFork returns whether num represents a block number after the PQT fork
func (c *ChainConfig) IsPQTFork(num *big.Int) bool {
	if c.PostQuantum == nil || c.PostQuantum.PQTBlock == nil {
		return false
	}
	return isForked(c.PostQuantum.PQTBlock, num)
}

// IsPQTTransition returns whether num is in the dual-signing transition period
func (c *ChainConfig) IsPQTTransition(num *big.Int) bool {
	if !c.IsPQTFork(num) {
		return false
	}

	transitionBlocks := c.PostQuantum.TransitionBlocks
	if transitionBlocks == 0 {
		transitionBlocks = PQTTransitionBlocks
	}

	enforceBlock := new(big.Int).Add(c.PostQuantum.PQTBlock, big.NewInt(int64(transitionBlocks)))
	return num.Cmp(enforceBlock) < 0
}

// IsPQTEnforced returns whether num is after the transition period (ML-DSA only)
func (c *ChainConfig) IsPQTEnforced(num *big.Int) bool {
	if !c.IsPQTFork(num) {
		return false
	}

	transitionBlocks := c.PostQuantum.TransitionBlocks
	if transitionBlocks == 0 {
		transitionBlocks = PQTTransitionBlocks
	}

	enforceBlock := new(big.Int).Add(c.PostQuantum.PQTBlock, big.NewInt(int64(transitionBlocks)))
	return num.Cmp(enforceBlock) >= 0
}

// GetDefaultMLDSAAlgorithm returns the default ML-DSA algorithm for the chain
func (c *ChainConfig) GetDefaultMLDSAAlgorithm() string {
	if c.PostQuantum == nil {
		return "ML-DSA-65" // Default to ML-DSA-65
	}

	switch c.PostQuantum.DefaultMLDSAAlgorithm {
	case 44:
		return "ML-DSA-44"
	case 65:
		return "ML-DSA-65"
	case 87:
		return "ML-DSA-87"
	default:
		return "ML-DSA-65" // Default fallback
	}
}

// Signature size constants for different ML-DSA variants
const (
	MLDSA44SignatureSize = 2420
	MLDSA44PublicKeySize = 1312

	MLDSA65SignatureSize = 3309
	MLDSA65PublicKeySize = 1952

	MLDSA87SignatureSize = 4627
	MLDSA87PublicKeySize = 2592
)

// GetMLDSASizes returns signature and public key sizes for an ML-DSA variant
func GetMLDSASizes(algorithm string) (sigSize, pkSize int) {
	switch algorithm {
	case "ML-DSA-44":
		return MLDSA44SignatureSize, MLDSA44PublicKeySize
	case "ML-DSA-65":
		return MLDSA65SignatureSize, MLDSA65PublicKeySize
	case "ML-DSA-87":
		return MLDSA87SignatureSize, MLDSA87PublicKeySize
	default:
		return MLDSA65SignatureSize, MLDSA65PublicKeySize // Default to ML-DSA-65
	}
}

// Maximum extra data size for post-quantum signatures in block headers
// This accounts for multiple validator signatures in consensus
const (
	// Maximum number of validators for size calculations
	MaxValidators = 50

	// Maximum extra data size with PQ signatures
	// 32 (vanity) + 50 * 20 (addresses) + 50 * 3309 (ML-DSA-65 signatures) + buffer
	MaxExtraDataSizePQ = 32 + MaxValidators*20 + MaxValidators*MLDSA65SignatureSize + 1024
)
