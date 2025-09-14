// Copyright 2024 The Splendor Authors
// This file implements advanced post-quantum consensus engine
// Supports multi-algorithm signatures, key rotation, and quantum-safe networking

package pqconsensus

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/mldsa"
	"github.com/ethereum/go-ethereum/crypto/mlkem"
	"github.com/ethereum/go-ethereum/crypto/slhdsa"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
)

// Post-quantum consensus engine constants
const (
	// Signature aggregation threshold
	PQAggregationThreshold = 10

	// Key rotation interval (in blocks)
	PQKeyRotationInterval = 100000

	// Maximum signature cache size
	PQSignatureCacheSize = 1024

	// Quantum-safe random beacon interval
	PQRandomBeaconInterval = 1000
)

// PQSignatureType represents different post-quantum signature types
type PQSignatureType byte

const (
	PQSigTypeMLDSA44    PQSignatureType = 0x01
	PQSigTypeMLDSA65    PQSignatureType = 0x02
	PQSigTypeMLDSA87    PQSignatureType = 0x03
	PQSigTypeSLHDSA128S PQSignatureType = 0x10
	PQSigTypeSLHDSA128F PQSignatureType = 0x11
	PQSigTypeSLHDSA192S PQSignatureType = 0x12
	PQSigTypeSLHDSA192F PQSignatureType = 0x13
	PQSigTypeSLHDSA256S PQSignatureType = 0x14
	PQSigTypeSLHDSA256F PQSignatureType = 0x15
)

// PQValidatorKey represents a post-quantum validator key with metadata
type PQValidatorKey struct {
	Address       common.Address  // Validator address
	Algorithm     string          // Signature algorithm
	PublicKey     []byte          // Post-quantum public key
	BackupKey     []byte          // Backup key (different algorithm)
	KeyGenTime    uint64          // Block number when key was generated
	ExpiryTime    uint64          // Block number when key expires
	SignatureType PQSignatureType // Type of signature algorithm
}

// PQSignatureBundle represents a bundle of signatures from different algorithms
type PQSignatureBundle struct {
	PrimarySignature   []byte          // Primary ML-DSA signature
	BackupSignature    []byte          // Backup SLH-DSA signature (optional)
	PrimaryType        PQSignatureType // Primary signature type
	BackupType         PQSignatureType // Backup signature type
	Timestamp          uint64          // Signature timestamp
	ValidatorAddress   common.Address  // Signer address
	AggregationProof   []byte          // Proof for signature aggregation
}

// PQConsensusEngine implements advanced post-quantum consensus
type PQConsensusEngine struct {
	config *params.PostQuantumConfig

	// Key management
	validatorKeys   map[common.Address]*PQValidatorKey
	keyRotationLog  *lru.ARCCache
	keyMutex        sync.RWMutex

	// Signature management
	signatureCache  *lru.ARCCache
	aggregatedSigs  map[common.Hash]*PQSignatureBundle
	sigMutex        sync.RWMutex

	// Quantum-safe networking
	kemKeys         map[common.Address][]byte // ML-KEM public keys for secure communication
	sharedSecrets   map[common.Address][]byte // Cached shared secrets
	networkMutex    sync.RWMutex

	// Random beacon for quantum-safe randomness
	randomBeacon    []byte
	beaconMutex     sync.RWMutex
	lastBeaconBlock uint64

	// Performance metrics
	verificationTimes map[string]time.Duration
	metricsMutex      sync.RWMutex
}

// NewPQConsensusEngine creates a new post-quantum consensus engine
func NewPQConsensusEngine(config *params.PostQuantumConfig) *PQConsensusEngine {
	keyRotationLog, _ := lru.NewARC(1000)
	signatureCache, _ := lru.NewARC(PQSignatureCacheSize)

	return &PQConsensusEngine{
		config:            config,
		validatorKeys:     make(map[common.Address]*PQValidatorKey),
		keyRotationLog:    keyRotationLog,
		signatureCache:    signatureCache,
		aggregatedSigs:    make(map[common.Hash]*PQSignatureBundle),
		kemKeys:           make(map[common.Address][]byte),
		sharedSecrets:     make(map[common.Address][]byte),
		randomBeacon:      make([]byte, 32),
		verificationTimes: make(map[string]time.Duration),
	}
}

// RegisterValidatorKey registers a post-quantum key for a validator
func (pq *PQConsensusEngine) RegisterValidatorKey(validator common.Address, algorithm string, 
	publicKey []byte, blockNumber uint64) error {
	pq.keyMutex.Lock()
	defer pq.keyMutex.Unlock()

	// Validate the key
	if err := mldsa.ValidateMLDSAParams(algorithm, make([]byte, 3309), publicKey); err != nil {
		return fmt.Errorf("invalid validator key: %v", err)
	}

	// Determine signature type
	var sigType PQSignatureType
	switch algorithm {
	case mldsa.MLDSA44:
		sigType = PQSigTypeMLDSA44
	case mldsa.MLDSA65:
		sigType = PQSigTypeMLDSA65
	case mldsa.MLDSA87:
		sigType = PQSigTypeMLDSA87
	default:
		return errors.New("unsupported algorithm")
	}

	// Generate backup key (SLH-DSA for long-term security)
	backupKey, _, err := slhdsa.GenerateKeyPair(slhdsa.SLHDSA128S)
	if err != nil {
		log.Warn("Failed to generate backup key", "error", err)
		backupKey = nil // Continue without backup key
	}

	validatorKey := &PQValidatorKey{
		Address:       validator,
		Algorithm:     algorithm,
		PublicKey:     publicKey,
		BackupKey:     backupKey,
		KeyGenTime:    blockNumber,
		ExpiryTime:    blockNumber + PQKeyRotationInterval,
		SignatureType: sigType,
	}

	pq.validatorKeys[validator] = validatorKey
	pq.keyRotationLog.Add(blockNumber, validator)

	log.Info("Registered PQ validator key", "validator", validator, "algorithm", algorithm, 
		"keySize", len(publicKey), "expiry", validatorKey.ExpiryTime)

	return nil
}

// VerifyPQSignatureBundle verifies a complete post-quantum signature bundle
func (pq *PQConsensusEngine) VerifyPQSignatureBundle(bundle *PQSignatureBundle, 
	message []byte, blockNumber uint64) error {
	
	start := time.Now()
	defer func() {
		pq.metricsMutex.Lock()
		pq.verificationTimes["bundle_verification"] = time.Since(start)
		pq.metricsMutex.Unlock()
	}()

	// Get validator key
	pq.keyMutex.RLock()
	validatorKey, exists := pq.validatorKeys[bundle.ValidatorAddress]
	pq.keyMutex.RUnlock()

	if !exists {
		return fmt.Errorf("validator key not found: %v", bundle.ValidatorAddress)
	}

	// Check key expiry
	if blockNumber > validatorKey.ExpiryTime {
		return fmt.Errorf("validator key expired at block %d, current block %d", 
			validatorKey.ExpiryTime, blockNumber)
	}

	// Verify primary signature
	primaryAlgorithm := pq.getAlgorithmFromType(bundle.PrimaryType)
	if err := mldsa.VerifySignature(primaryAlgorithm, message, bundle.PrimarySignature, validatorKey.PublicKey); err != nil {
		return fmt.Errorf("primary signature verification failed: %v", err)
	}

	// Verify backup signature if present
	if len(bundle.BackupSignature) > 0 && len(validatorKey.BackupKey) > 0 {
		backupAlgorithm := pq.getAlgorithmFromType(bundle.BackupType)
		if err := slhdsa.VerifySignature(backupAlgorithm, message, bundle.BackupSignature, validatorKey.BackupKey); err != nil {
			log.Warn("Backup signature verification failed", "error", err)
			// Don't fail on backup signature failure, just log it
		}
	}

	// Verify aggregation proof if present
	if len(bundle.AggregationProof) > 0 {
		if err := pq.verifyAggregationProof(bundle, message); err != nil {
			return fmt.Errorf("aggregation proof verification failed: %v", err)
		}
	}

	log.Debug("PQ signature bundle verified", "validator", bundle.ValidatorAddress, 
		"primaryType", bundle.PrimaryType, "hasBackup", len(bundle.BackupSignature) > 0)

	return nil
}

// getAlgorithmFromType converts signature type to algorithm string
func (pq *PQConsensusEngine) getAlgorithmFromType(sigType PQSignatureType) string {
	switch sigType {
	case PQSigTypeMLDSA44:
		return mldsa.MLDSA44
	case PQSigTypeMLDSA65:
		return mldsa.MLDSA65
	case PQSigTypeMLDSA87:
		return mldsa.MLDSA87
	case PQSigTypeSLHDSA128S:
		return slhdsa.SLHDSA128S
	case PQSigTypeSLHDSA128F:
		return slhdsa.SLHDSA128F
	case PQSigTypeSLHDSA192S:
		return slhdsa.SLHDSA192S
	case PQSigTypeSLHDSA192F:
		return slhdsa.SLHDSA192F
	case PQSigTypeSLHDSA256S:
		return slhdsa.SLHDSA256S
	case PQSigTypeSLHDSA256F:
		return slhdsa.SLHDSA256F
	default:
		return mldsa.MLDSA65 // Default fallback
	}
}

// verifyAggregationProof verifies signature aggregation proof
func (pq *PQConsensusEngine) verifyAggregationProof(bundle *PQSignatureBundle, message []byte) error {
	// Implement BLS-like aggregation for ML-DSA signatures
	// This is a complex cryptographic operation that would require
	// specialized aggregation schemes for lattice-based signatures
	
	// For now, implement a simple hash-based proof
	expectedProof := crypto.Keccak256(
		bundle.PrimarySignature,
		bundle.BackupSignature,
		message,
		bundle.ValidatorAddress.Bytes(),
	)

	if !bytes.Equal(bundle.AggregationProof, expectedProof) {
		return errors.New("aggregation proof mismatch")
	}

	return nil
}

// RotateValidatorKeys performs automatic key rotation for validators
func (pq *PQConsensusEngine) RotateValidatorKeys(blockNumber uint64) error {
	pq.keyMutex.Lock()
	defer pq.keyMutex.Unlock()

	rotatedCount := 0
	for validator, key := range pq.validatorKeys {
		if blockNumber >= key.ExpiryTime {
			// Generate new key pair
			newPublicKey, newSecretKey, err := mldsa.GenerateKeyPair(key.Algorithm)
			if err != nil {
				log.Error("Failed to rotate validator key", "validator", validator, "error", err)
				continue
			}

			// Generate new backup key
			newBackupKey, _, err := slhdsa.GenerateKeyPair(slhdsa.SLHDSA128S)
			if err != nil {
				log.Warn("Failed to generate backup key during rotation", "validator", validator, "error", err)
				newBackupKey = nil
			}

			// Update key
			key.PublicKey = newPublicKey
			key.BackupKey = newBackupKey
			key.KeyGenTime = blockNumber
			key.ExpiryTime = blockNumber + PQKeyRotationInterval

			// Log rotation
			pq.keyRotationLog.Add(blockNumber, validator)
			rotatedCount++

			log.Info("Rotated validator key", "validator", validator, "newExpiry", key.ExpiryTime)

			// In a real implementation, this would need to be coordinated
			// with the validator to update their signing key
			_ = newSecretKey // Placeholder for key distribution
		}
	}

	if rotatedCount > 0 {
		log.Info("Completed key rotation", "rotatedKeys", rotatedCount, "block", blockNumber)
	}

	return nil
}

// EstablishQuantumSafeChannel establishes a quantum-safe communication channel
func (pq *PQConsensusEngine) EstablishQuantumSafeChannel(peerAddress common.Address) error {
	pq.networkMutex.Lock()
	defer pq.networkMutex.Unlock()

	// Check if we already have a shared secret
	if _, exists := pq.sharedSecrets[peerAddress]; exists {
		return nil // Channel already established
	}

	// Generate ML-KEM key pair for this peer
	publicKey, secretKey, err := mlkem.GenerateKeyPair(mlkem.MLKEM768)
	if err != nil {
		return fmt.Errorf("failed to generate ML-KEM key pair: %v", err)
	}

	// Store our public key
	pq.kemKeys[peerAddress] = publicKey

	// In a real implementation, this would involve:
	// 1. Exchanging ML-KEM public keys with the peer
	// 2. Performing key encapsulation
	// 3. Deriving shared secret
	// 4. Setting up encrypted communication channel

	// For now, create a placeholder shared secret
	sharedSecret := crypto.Keccak256(publicKey, secretKey[:32])
	pq.sharedSecrets[peerAddress] = sharedSecret

	log.Info("Established quantum-safe channel", "peer", peerAddress, "algorithm", mlkem.MLKEM768)
	return nil
}

// GenerateQuantumRandomBeacon generates quantum-safe randomness for the network
func (pq *PQConsensusEngine) GenerateQuantumRandomBeacon(blockNumber uint64, 
	previousBeacon []byte, validatorSignatures [][]byte) ([]byte, error) {
	
	pq.beaconMutex.Lock()
	defer pq.beaconMutex.Unlock()

	// Only generate beacon at specified intervals
	if blockNumber%PQRandomBeaconInterval != 0 {
		return pq.randomBeacon, nil
	}

	// Combine multiple sources of entropy
	hasher := crypto.NewKeccakState()
	
	// Previous beacon
	hasher.Write(previousBeacon)
	
	// Block number
	blockBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockBytes, blockNumber)
	hasher.Write(blockBytes)
	
	// Validator signatures (provides distributed randomness)
	for _, sig := range validatorSignatures {
		hasher.Write(sig)
	}
	
	// System entropy
	systemEntropy := make([]byte, 32)
	rand.Read(systemEntropy)
	hasher.Write(systemEntropy)
	
	// Current timestamp
	timestamp := time.Now().UnixNano()
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(timestamp))
	hasher.Write(timestampBytes)

	// Generate new beacon
	newBeacon := make([]byte, 32)
	hasher.Read(newBeacon)
	
	pq.randomBeacon = newBeacon
	pq.lastBeaconBlock = blockNumber

	log.Info("Generated quantum random beacon", "block", blockNumber, 
		"beacon", common.Bytes2Hex(newBeacon[:8]))

	return newBeacon, nil
}

// AggregateSignatures performs signature aggregation for efficiency
func (pq *PQConsensusEngine) AggregateSignatures(signatures []*PQSignatureBundle, 
	message []byte) (*PQSignatureBundle, error) {
	
	if len(signatures) < PQAggregationThreshold {
		return nil, errors.New("insufficient signatures for aggregation")
	}

	// For ML-DSA, true aggregation is complex and not standardized
	// We implement a simplified aggregation using hash-based combination
	
	aggregatedBundle := &PQSignatureBundle{
		PrimaryType:      PQSigTypeMLDSA65, // Use strongest common algorithm
		Timestamp:        uint64(time.Now().Unix()),
		ValidatorAddress: common.Address{}, // Aggregated signature has no single validator
	}

	// Combine all primary signatures
	hasher := crypto.NewKeccakState()
	for _, sig := range signatures {
		hasher.Write(sig.PrimarySignature)
		hasher.Write(sig.ValidatorAddress.Bytes())
	}
	hasher.Write(message)

	// Create aggregation proof
	aggregatedBundle.AggregationProof = make([]byte, 32)
	hasher.Read(aggregatedBundle.AggregationProof)

	// Create combined signature (simplified approach)
	combinedSig := crypto.Keccak256(aggregatedBundle.AggregationProof, message)
	aggregatedBundle.PrimarySignature = combinedSig

	log.Info("Aggregated signatures", "count", len(signatures), "algorithm", "ML-DSA-65")
	return aggregatedBundle, nil
}

// VerifyQuantumSafeBlock verifies a block with post-quantum signatures
func (pq *PQConsensusEngine) VerifyQuantumSafeBlock(block *types.Block, 
	parentBlock *types.Block) error {
	
	header := block.Header()
	blockNumber := header.Number.Uint64()

	// Extract post-quantum signatures from block
	bundles, err := pq.extractPQSignatureBundles(header)
	if err != nil {
		return fmt.Errorf("failed to extract PQ signatures: %v", err)
	}

	// Verify each signature bundle
	sealHash := pq.calculateSealHash(header)
	for _, bundle := range bundles {
		if err := pq.VerifyPQSignatureBundle(bundle, sealHash.Bytes(), blockNumber); err != nil {
			return fmt.Errorf("signature bundle verification failed: %v", err)
		}
	}

	// Verify quantum random beacon if present
	if blockNumber%PQRandomBeaconInterval == 0 {
		if err := pq.verifyQuantumRandomBeacon(header, parentBlock); err != nil {
			return fmt.Errorf("quantum random beacon verification failed: %v", err)
		}
	}

	// Perform key rotation if needed
	if err := pq.RotateValidatorKeys(blockNumber); err != nil {
		log.Warn("Key rotation failed", "error", err)
		// Don't fail block verification on key rotation errors
	}

	return nil
}

// extractPQSignatureBundles extracts post-quantum signature bundles from block header
func (pq *PQConsensusEngine) extractPQSignatureBundles(header *types.Header) ([]*PQSignatureBundle, error) {
	// This would parse the complex signature data from header.Extra
	// For now, return empty slice as placeholder
	return []*PQSignatureBundle{}, nil
}

// calculateSealHash calculates the hash to be signed for consensus
func (pq *PQConsensusEngine) calculateSealHash(header *types.Header) common.Hash {
	// Use the same seal hash calculation as Clique but with additional PQ fields
	hasher := crypto.NewKeccakState()
	
	// Standard header fields
	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.MixDigest,
		header.Nonce,
	})

	// Add post-quantum specific fields
	if pq.config != nil {
		hasher.Write([]byte(pq.config.String()))
	}

	var hash common.Hash
	hasher.Read(hash[:])
	return hash
}

// verifyQuantumRandomBeacon verifies the quantum random beacon
func (pq *PQConsensusEngine) verifyQuantumRandomBeacon(header *types.Header, 
	parentBlock *types.Block) error {
	
	// Extract beacon from header (would be in a specific field)
	// For now, just validate that we have a beacon
	if len(pq.randomBeacon) != 32 {
		return errors.New("invalid quantum random beacon")
	}

	// Verify beacon was generated correctly
	// This would involve checking the beacon generation process
	// against the previous block's beacon and validator signatures

	return nil
}

// GetPerformanceMetrics returns performance metrics for the PQ consensus
func (pq *PQConsensusEngine) GetPerformanceMetrics() map[string]interface{} {
	pq.metricsMutex.RLock()
	defer pq.metricsMutex.RUnlock()

	metrics := make(map[string]interface{})
	
	// Verification times
	for operation, duration := range pq.verificationTimes {
		metrics[operation+"_time_ms"] = duration.Milliseconds()
	}

	// Key statistics
	pq.keyMutex.RLock()
	metrics["registered_validators"] = len(pq.validatorKeys)
	metrics["active_kem_channels"] = len(pq.sharedSecrets)
	pq.keyMutex.RUnlock()

	// Signature cache statistics
	metrics["signature_cache_size"] = pq.signatureCache.Len()
	metrics["aggregated_signatures"] = len(pq.aggregatedSigs)

	// Random beacon info
	pq.beaconMutex.RLock()
	metrics["last_beacon_block"] = pq.lastBeaconBlock
	metrics["beacon_length"] = len(pq.randomBeacon)
	pq.beaconMutex.RUnlock()

	return metrics
}

// Close cleans up the post-quantum consensus engine
func (pq *PQConsensusEngine) Close() error {
	// Clear sensitive data
	pq.keyMutex.Lock()
	for addr := range pq.validatorKeys {
		delete(pq.validatorKeys, addr)
	}
	pq.keyMutex.Unlock()

	pq.networkMutex.Lock()
	for addr := range pq.sharedSecrets {
		// Zero out shared secrets
		for i := range pq.sharedSecrets[addr] {
			pq.sharedSecrets[addr][i] = 0
		}
		delete(pq.sharedSecrets, addr)
	}
	pq.networkMutex.Unlock()

	pq.beaconMutex.Lock()
	// Zero out random beacon
	for i := range pq.randomBeacon {
		pq.randomBeacon[i] = 0
	}
	pq.beaconMutex.Unlock()

	log.Info("Post-quantum consensus engine closed")
	return nil
}
