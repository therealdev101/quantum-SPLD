//go:build !cgo || no_liboqs
// +build !cgo no_liboqs

// Copyright 2024 The Splendor Authors
// This file implements SLH-DSA (SPHINCS+) signature verification for quantum resistance
// Based on FIPS 205 specification - Fallback implementation

package slhdsa

import (
	"errors"
)

// SLH-DSA algorithm variants
const (
	SLHDSA128S = "SLH-DSA-128s" // 128-bit security, small signatures
	SLHDSA128F = "SLH-DSA-128f" // 128-bit security, fast verification
	SLHDSA192S = "SLH-DSA-192s" // 192-bit security, small signatures
	SLHDSA192F = "SLH-DSA-192f" // 192-bit security, fast verification
	SLHDSA256S = "SLH-DSA-256s" // 256-bit security, small signatures
	SLHDSA256F = "SLH-DSA-256f" // 256-bit security, fast verification
)

// SLH-DSA parameter sets (approximate sizes in bytes)
var SLHDSAParams = map[string]struct {
	PublicKeySize  int
	SecretKeySize  int
	SignatureSize  int
	SecurityLevel  int
	FastVerify     bool
}{
	SLHDSA128S: {PublicKeySize: 32, SecretKeySize: 64, SignatureSize: 7856, SecurityLevel: 1, FastVerify: false},
	SLHDSA128F: {PublicKeySize: 32, SecretKeySize: 64, SignatureSize: 17088, SecurityLevel: 1, FastVerify: true},
	SLHDSA192S: {PublicKeySize: 48, SecretKeySize: 96, SignatureSize: 16224, SecurityLevel: 3, FastVerify: false},
	SLHDSA192F: {PublicKeySize: 48, SecretKeySize: 96, SignatureSize: 35664, SecurityLevel: 3, FastVerify: true},
	SLHDSA256S: {PublicKeySize: 64, SecretKeySize: 128, SignatureSize: 29792, SecurityLevel: 5, FastVerify: false},
	SLHDSA256F: {PublicKeySize: 64, SecretKeySize: 128, SignatureSize: 49856, SecurityLevel: 5, FastVerify: true},
}

var (
	ErrInvalidSLHDSAAlgorithm = errors.New("invalid SLH-DSA algorithm")
	ErrSLHDSANotAvailable     = errors.New("SLH-DSA not available without liboqs")
)

// VerifySignature verifies an SLH-DSA signature (fallback implementation)
func VerifySignature(algorithm string, message, signature, publicKey []byte) error {
	if len(message) == 0 {
		return errors.New("empty message")
	}
	if len(signature) == 0 {
		return errors.New("invalid signature")
	}
	if len(publicKey) == 0 {
		return errors.New("invalid public key")
	}

	// Validate algorithm
	if _, exists := SLHDSAParams[algorithm]; !exists {
		return ErrInvalidSLHDSAAlgorithm
	}

	// In fallback mode, we cannot actually verify the signature
	return ErrSLHDSANotAvailable
}

// GetSLHDSALengths returns the expected signature and public key lengths
func GetSLHDSALengths(algorithm string) (sigLen, pkLen, skLen int, err error) {
	params, exists := SLHDSAParams[algorithm]
	if !exists {
		return 0, 0, 0, ErrInvalidSLHDSAAlgorithm
	}

	return params.SignatureSize, params.PublicKeySize, params.SecretKeySize, nil
}

// GenerateKeyPair generates an SLH-DSA key pair (not available in fallback mode)
func GenerateKeyPair(algorithm string) (publicKey, secretKey []byte, err error) {
	return nil, nil, ErrSLHDSANotAvailable
}

// SignMessage signs a message with SLH-DSA (not available in fallback mode)
func SignMessage(algorithm string, message, secretKey []byte) (signature []byte, err error) {
	return nil, ErrSLHDSANotAvailable
}

// IsSLHDSASupported checks if SLH-DSA is supported (always false in fallback mode)
func IsSLHDSASupported(algorithm string) bool {
	return false
}

// ValidateSLHDSAParams validates SLH-DSA parameters
func ValidateSLHDSAParams(algorithm string, signature, publicKey []byte) error {
	expectedSigLen, expectedPkLen, _, err := GetSLHDSALengths(algorithm)
	if err != nil {
		return err
	}

	if len(signature) != expectedSigLen {
		return errors.New("invalid signature length")
	}

	if len(publicKey) != expectedPkLen {
		return errors.New("invalid public key length")
	}

	return nil
}
