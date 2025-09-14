//go:build !cgo || no_liboqs
// +build !cgo no_liboqs

// Copyright 2024 The Splendor Authors
// This file implements ML-DSA (Dilithium) signature verification for quantum resistance
// Based on FIPS 204 specification - Fallback implementation without liboqs

package mldsa

import (
	"errors"
)

// ML-DSA algorithm variants
const (
	MLDSA44 = "ML-DSA-44" // Compact variant for user transactions
	MLDSA65 = "ML-DSA-65" // Recommended variant for consensus
	MLDSA87 = "ML-DSA-87" // High security variant
)

// ML-DSA parameter sets (approximate sizes in bytes)
var MLDSAParams = map[string]struct {
	PublicKeySize  int
	SignatureSize  int
	SecurityLevel  int
}{
	MLDSA44: {PublicKeySize: 1312, SignatureSize: 2420, SecurityLevel: 2},
	MLDSA65: {PublicKeySize: 1952, SignatureSize: 3309, SecurityLevel: 3},
	MLDSA87: {PublicKeySize: 2592, SignatureSize: 4627, SecurityLevel: 5},
}

var (
	ErrInvalidAlgorithm    = errors.New("invalid ML-DSA algorithm")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrInvalidPublicKey    = errors.New("invalid public key")
	ErrInvalidLength       = errors.New("invalid signature or public key length")
	ErrVerificationFailed  = errors.New("signature verification failed")
	ErrLibOQSNotAvailable  = errors.New("liboqs library not available")
)

// VerifySignature verifies an ML-DSA signature (fallback implementation)
func VerifySignature(algorithm string, message, signature, publicKey []byte) error {
	if len(message) == 0 {
		return errors.New("empty message")
	}
	if len(signature) == 0 {
		return ErrInvalidSignature
	}
	if len(publicKey) == 0 {
		return ErrInvalidPublicKey
	}

	// Validate algorithm
	if _, exists := MLDSAParams[algorithm]; !exists {
		return ErrInvalidAlgorithm
	}

	// Validate lengths against expected parameters
	params := MLDSAParams[algorithm]
	if len(signature) != params.SignatureSize {
		return ErrInvalidLength
	}
	if len(publicKey) != params.PublicKeySize {
		return ErrInvalidLength
	}

	// In fallback mode, we cannot actually verify the signature
	// This would need to be replaced with a pure Go implementation
	// or return an error indicating liboqs is required
	return ErrLibOQSNotAvailable
}

// GetMLDSALengths returns the expected signature and public key lengths for an algorithm
func GetMLDSALengths(algorithm string) (sigLen, pkLen int, err error) {
	params, exists := MLDSAParams[algorithm]
	if !exists {
		return 0, 0, ErrInvalidAlgorithm
	}

	return params.SignatureSize, params.PublicKeySize, nil
}

// ValidateMLDSAParams validates ML-DSA signature and public key lengths
func ValidateMLDSAParams(algorithm string, signature, publicKey []byte) error {
	expectedSigLen, expectedPkLen, err := GetMLDSALengths(algorithm)
	if err != nil {
		return err
	}

	if len(signature) != expectedSigLen {
		return ErrInvalidLength
	}

	if len(publicKey) != expectedPkLen {
		return ErrInvalidLength
	}

	return nil
}

// IsMLDSASupported checks if ML-DSA is supported (always false in fallback mode)
func IsMLDSASupported(algorithm string) bool {
	return false // liboqs not available in fallback mode
}

// GenerateKeyPair generates an ML-DSA key pair (not available in fallback mode)
func GenerateKeyPair(algorithm string) (publicKey, secretKey []byte, err error) {
	return nil, nil, ErrLibOQSNotAvailable
}

// SignMessage signs a message with ML-DSA (not available in fallback mode)
func SignMessage(algorithm string, message, secretKey []byte) (signature []byte, err error) {
	return nil, ErrLibOQSNotAvailable
}

// BatchVerifySignatures verifies multiple ML-DSA signatures (not available in fallback mode)
func BatchVerifySignatures(algorithm string, messages [][]byte, signatures [][]byte, publicKeys [][]byte) ([]bool, error) {
	return nil, ErrLibOQSNotAvailable
}
