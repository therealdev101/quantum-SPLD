// Copyright 2024 The Splendor Authors
// This file implements post-quantum extensions for Clique consensus
// Supports dual-signing transition from ECDSA to ML-DSA

package clique

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/mldsa"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// Post-quantum signature constants
const (
	// PQ signature type identifiers in extraData
	PQSigTypeMLDSA65 = 0x01
	PQSigTypeMLDSA44 = 0x02
	PQSigTypeMLDSA87 = 0x03

	// TLV (Type-Length-Value) format for PQ signatures in extraData
	// Format: [type(1)] + [length(4)] + [signature(length)] + [pubkey_length(4)] + [pubkey(pubkey_length)]
	PQSigTLVHeaderSize = 9 // 1 + 4 + 4 bytes
)

// PQSignature represents a post-quantum signature with metadata
type PQSignature struct {
	Type      byte   // Algorithm type (ML-DSA variant)
	Signature []byte // The actual signature
	PublicKey []byte // Public key for verification
}

// Encode encodes a PQ signature into TLV format for extraData
func (pq *PQSignature) Encode() []byte {
	sigLen := len(pq.Signature)
	pkLen := len(pq.PublicKey)
	
	buf := make([]byte, PQSigTLVHeaderSize+sigLen+pkLen)
	offset := 0
	
	// Type
	buf[offset] = pq.Type
	offset++
	
	// Signature length
	binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(sigLen))
	offset += 4
	
	// Public key length
	binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(pkLen))
	offset += 4
	
	// Signature
	copy(buf[offset:offset+sigLen], pq.Signature)
	offset += sigLen
	
	// Public key
	copy(buf[offset:offset+pkLen], pq.PublicKey)
	
	return buf
}

// DecodePQSignature decodes a PQ signature from TLV format
func DecodePQSignature(data []byte) (*PQSignature, error) {
	if len(data) < PQSigTLVHeaderSize {
		return nil, errors.New("PQ signature data too short")
	}
	
	offset := 0
	
	// Type
	sigType := data[offset]
	offset++
	
	// Signature length
	sigLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	
	// Public key length
	pkLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	
	// Validate total length
	expectedLen := PQSigTLVHeaderSize + int(sigLen) + int(pkLen)
	if len(data) < expectedLen {
		return nil, errors.New("PQ signature data length mismatch")
	}
	
	// Extract signature
	signature := make([]byte, sigLen)
	copy(signature, data[offset:offset+int(sigLen)])
	offset += int(sigLen)
	
	// Extract public key
	publicKey := make([]byte, pkLen)
	copy(publicKey, data[offset:offset+int(pkLen)])
	
	return &PQSignature{
		Type:      sigType,
		Signature: signature,
		PublicKey: publicKey,
	}, nil
}

// GetMLDSAAlgorithm returns the ML-DSA algorithm name for the signature type
func (pq *PQSignature) GetMLDSAAlgorithm() string {
	switch pq.Type {
	case PQSigTypeMLDSA44:
		return mldsa.MLDSA44
	case PQSigTypeMLDSA65:
		return mldsa.MLDSA65
	case PQSigTypeMLDSA87:
		return mldsa.MLDSA87
	default:
		return mldsa.MLDSA65 // Default fallback
	}
}

// extractPQSignature extracts the post-quantum signature from block header extraData
func extractPQSignature(header *types.Header) (*PQSignature, error) {
	if len(header.Extra) < extraVanity+extraSeal {
		return nil, errors.New("header extraData too short for PQ signature")
	}
	
	// Look for PQ signature after the ECDSA signature
	ecdsaEnd := len(header.Extra) - extraSeal
	if ecdsaEnd <= extraVanity {
		return nil, errors.New("no space for PQ signature")
	}
	
	// Check if there's additional data after vanity but before ECDSA seal
	pqStart := extraVanity
	pqEnd := ecdsaEnd
	
	// Skip signer list in checkpoint blocks
	if (header.Number.Uint64() % epochLength) == 0 {
		signersBytes := pqEnd - pqStart
		if signersBytes%common.AddressLength != 0 {
			return nil, errors.New("invalid signer list length")
		}
		pqStart += signersBytes
	}
	
	if pqStart >= pqEnd {
		return nil, errors.New("no PQ signature found")
	}
	
	pqData := header.Extra[pqStart:pqEnd]
	return DecodePQSignature(pqData)
}

// verifyPQSignature verifies a post-quantum signature against the header
func (c *Clique) verifyPQSignature(header *types.Header, pqSig *PQSignature) error {
	// Get the seal hash (same as ECDSA)
	sealHash := SealHash(header)
	
	// Verify the ML-DSA signature
	algorithm := pqSig.GetMLDSAAlgorithm()
	err := mldsa.VerifySignature(algorithm, sealHash.Bytes(), pqSig.Signature, pqSig.PublicKey)
	if err != nil {
		return fmt.Errorf("PQ signature verification failed: %v", err)
	}
	
	return nil
}

// recoverPQSigner recovers the signer address from a post-quantum signature
// For ML-DSA, we derive the address from the public key using Keccak256
func recoverPQSigner(pqSig *PQSignature) (common.Address, error) {
	// Derive address from public key: address = keccak256(pubkey)[12:]
	hash := crypto.Keccak256(pqSig.PublicKey)
	var addr common.Address
	copy(addr[:], hash[12:])
	return addr, nil
}

// isPQTFork checks if the block number is at or after the PQT fork
func (c *Clique) isPQTFork(number uint64, config *params.ChainConfig) bool {
	if config.PostQuantum == nil || config.PostQuantum.PQTBlock == nil {
		return false
	}
	return number >= config.PostQuantum.PQTBlock.Uint64()
}

// isPQTTransition checks if we're in the dual-signing transition period
func (c *Clique) isPQTTransition(number uint64, config *params.ChainConfig) bool {
	if !c.isPQTFork(number, config) {
		return false
	}
	
	transitionBlocks := config.PostQuantum.TransitionBlocks
	if transitionBlocks == 0 {
		transitionBlocks = params.PQTTransitionBlocks
	}
	
	enforceBlock := config.PostQuantum.PQTBlock.Uint64() + transitionBlocks
	return number < enforceBlock
}

// isPQTEnforced checks if we're in the ML-DSA-only period
func (c *Clique) isPQTEnforced(number uint64, config *params.ChainConfig) bool {
	if !c.isPQTFork(number, config) {
		return false
	}
	
	transitionBlocks := config.PostQuantum.TransitionBlocks
	if transitionBlocks == 0 {
		transitionBlocks = params.PQTTransitionBlocks
	}
	
	enforceBlock := config.PostQuantum.PQTBlock.Uint64() + transitionBlocks
	return number >= enforceBlock
}

// verifyPQSeal verifies post-quantum signatures in block headers
func (c *Clique) verifyPQSeal(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	number := header.Number.Uint64()
	config := chain.Config()
	
	// Before PQT fork, no PQ verification needed
	if !c.isPQTFork(number, config) {
		return nil
	}
	
	// Extract PQ signature
	pqSig, err := extractPQSignature(header)
	if err != nil {
		if c.isPQTEnforced(number, config) {
			// In enforce mode, PQ signature is required
			return fmt.Errorf("PQ signature required but not found: %v", err)
		}
		// In transition mode, PQ signature is optional
		log.Debug("PQ signature not found in transition period", "number", number, "error", err)
		return nil
	}
	
	// Verify the PQ signature
	if err := c.verifyPQSignature(header, pqSig); err != nil {
		return fmt.Errorf("PQ signature verification failed: %v", err)
	}
	
	// Recover signer from PQ signature
	pqSigner, err := recoverPQSigner(pqSig)
	if err != nil {
		return fmt.Errorf("failed to recover PQ signer: %v", err)
	}
	
	// In transition mode, verify both ECDSA and PQ signers match
	if c.isPQTTransition(number, config) {
		ecdsaSigner, err := ecrecover(header, c.signatures)
		if err != nil {
			return fmt.Errorf("failed to recover ECDSA signer: %v", err)
		}
		
		if pqSigner != ecdsaSigner {
			return fmt.Errorf("PQ signer %v does not match ECDSA signer %v", pqSigner, ecdsaSigner)
		}
	}
	
	// Verify signer is authorized (same logic as ECDSA)
	snap, err := c.snapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}
	
	if _, ok := snap.Signers[pqSigner]; !ok {
		return errUnauthorizedSigner
	}
	
	// Check recent signers
	for seen, recent := range snap.Recents {
		if recent == pqSigner {
			if limit := uint64(len(snap.Signers)/2 + 1); seen > number-limit {
				return errRecentlySigned
			}
		}
	}
	
	log.Debug("PQ signature verified successfully", "number", number, "signer", pqSigner, "algorithm", pqSig.GetMLDSAAlgorithm())
	return nil
}

// preparePQExtraData prepares extraData with space for both ECDSA and PQ signatures
func (c *Clique) preparePQExtraData(header *types.Header, config *params.ChainConfig) error {
	number := header.Number.Uint64()
	
	// Before PQT fork, use standard extraData preparation
	if !c.isPQTFork(number, config) {
		return nil
	}
	
	// Estimate space needed for PQ signature
	algorithm := config.GetDefaultMLDSAAlgorithm()
	sigSize, pkSize := params.GetMLDSASizes(algorithm)
	pqSigSize := PQSigTLVHeaderSize + sigSize + pkSize
	
	// Ensure extraData has enough space
	currentSize := len(header.Extra)
	requiredSize := currentSize + pqSigSize
	
	if requiredSize > params.MaxExtraDataSizePQ {
		return fmt.Errorf("extraData would exceed maximum size: %d > %d", requiredSize, params.MaxExtraDataSizePQ)
	}
	
	// Extend extraData to accommodate PQ signature
	// The PQ signature will be inserted before the ECDSA seal
	newExtra := make([]byte, requiredSize)
	copy(newExtra, header.Extra[:len(header.Extra)-extraSeal])
	// Leave space for PQ signature (will be filled during sealing)
	copy(newExtra[len(newExtra)-extraSeal:], header.Extra[len(header.Extra)-extraSeal:])
	
	header.Extra = newExtra
	return nil
}

// sealPQSignature adds a post-quantum signature to the block header
func (c *Clique) sealPQSignature(header *types.Header, pqSig *PQSignature) error {
	if len(header.Extra) < extraSeal {
		return errors.New("header extraData too short for sealing")
	}
	
	// Encode PQ signature
	pqData := pqSig.Encode()
	
	// Insert PQ signature before ECDSA seal
	ecdsaSeal := header.Extra[len(header.Extra)-extraSeal:]
	insertPos := len(header.Extra) - extraSeal - len(pqData)
	
	if insertPos < extraVanity {
		return errors.New("not enough space for PQ signature")
	}
	
	// Create new extraData with PQ signature
	newExtra := make([]byte, len(header.Extra))
	copy(newExtra, header.Extra[:insertPos])
	copy(newExtra[insertPos:insertPos+len(pqData)], pqData)
	copy(newExtra[insertPos+len(pqData):], ecdsaSeal)
	
	header.Extra = newExtra
	return nil
}

// PQSignerFn is a function type for post-quantum signing
type PQSignerFn func(algorithm string, message []byte) (signature, publicKey []byte, err error)

// sealWithPQ performs sealing with post-quantum signatures
func (c *Clique) sealWithPQ(chain consensus.ChainHeaderReader, header *types.Header, pqSignFn PQSignerFn) error {
	number := header.Number.Uint64()
	config := chain.Config()
	
	// Before PQT fork, no PQ sealing needed
	if !c.isPQTFork(number, config) {
		return nil
	}
	
	// Prepare extraData for PQ signature
	if err := c.preparePQExtraData(header, config); err != nil {
		return err
	}
	
	// Get seal hash
	sealHash := SealHash(header)
	
	// Create PQ signature
	algorithm := config.GetDefaultMLDSAAlgorithm()
	signature, publicKey, err := pqSignFn(algorithm, sealHash.Bytes())
	if err != nil {
		return fmt.Errorf("PQ signing failed: %v", err)
	}
	
	// Determine signature type
	var sigType byte
	switch algorithm {
	case mldsa.MLDSA44:
		sigType = PQSigTypeMLDSA44
	case mldsa.MLDSA65:
		sigType = PQSigTypeMLDSA65
	case mldsa.MLDSA87:
		sigType = PQSigTypeMLDSA87
	default:
		sigType = PQSigTypeMLDSA65
	}
	
	pqSig := &PQSignature{
		Type:      sigType,
		Signature: signature,
		PublicKey: publicKey,
	}
	
	// Add PQ signature to header
	if err := c.sealPQSignature(header, pqSig); err != nil {
		return fmt.Errorf("failed to seal PQ signature: %v", err)
	}
	
	log.Debug("PQ signature sealed", "number", number, "algorithm", algorithm, "sigSize", len(signature), "pkSize", len(publicKey))
	return nil
}
