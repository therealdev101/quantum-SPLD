//go:build !cgo || no_liboqs
// +build !cgo no_liboqs

// Copyright 2024 The Splendor Authors
// This file implements ML-KEM (Kyber) key encapsulation for quantum resistance
// Based on FIPS 203 specification - Fallback implementation

package mlkem

import (
	"errors"
)

// ML-KEM algorithm variants
const (
	MLKEM512  = "ML-KEM-512"  // 128-bit security level
	MLKEM768  = "ML-KEM-768"  // 192-bit security level (recommended)
	MLKEM1024 = "ML-KEM-1024" // 256-bit security level
)

// ML-KEM parameter sets
var MLKEMParams = map[string]struct {
	PublicKeySize    int
	SecretKeySize    int
	CiphertextSize   int
	SharedSecretSize int
	SecurityLevel    int
}{
	MLKEM512:  {PublicKeySize: 800, SecretKeySize: 1632, CiphertextSize: 768, SharedSecretSize: 32, SecurityLevel: 1},
	MLKEM768:  {PublicKeySize: 1184, SecretKeySize: 2400, CiphertextSize: 1088, SharedSecretSize: 32, SecurityLevel: 3},
	MLKEM1024: {PublicKeySize: 1568, SecretKeySize: 3168, CiphertextSize: 1568, SharedSecretSize: 32, SecurityLevel: 5},
}

var (
	ErrInvalidKEMAlgorithm = errors.New("invalid ML-KEM algorithm")
	ErrInvalidCiphertext   = errors.New("invalid ciphertext")
	ErrInvalidSecretKey    = errors.New("invalid secret key")
	ErrInvalidLength       = errors.New("invalid parameter length")
	ErrKEMNotAvailable     = errors.New("ML-KEM not available without liboqs")
)

// GenerateKeyPair generates an ML-KEM key pair (not available in fallback mode)
func GenerateKeyPair(algorithm string) (publicKey, secretKey []byte, err error) {
	return nil, nil, ErrKEMNotAvailable
}

// Encapsulate performs key encapsulation (not available in fallback mode)
func Encapsulate(algorithm string, publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	return nil, nil, ErrKEMNotAvailable
}

// Decapsulate performs key decapsulation (not available in fallback mode)
func Decapsulate(algorithm string, ciphertext, secretKey []byte) (sharedSecret []byte, err error) {
	return nil, ErrKEMNotAvailable
}

// ValidateMLKEMParams validates ML-KEM parameters
func ValidateMLKEMParams(algorithm string, publicKey []byte) error {
	params, exists := MLKEMParams[algorithm]
	if !exists {
		return ErrInvalidKEMAlgorithm
	}

	if len(publicKey) != params.PublicKeySize {
		return ErrInvalidLength
	}

	return nil
}

// GetMLKEMSizes returns the sizes for ML-KEM parameters
func GetMLKEMSizes(algorithm string) (pkSize, skSize, ctSize, ssSize int, err error) {
	params, exists := MLKEMParams[algorithm]
	if !exists {
		return 0, 0, 0, 0, ErrInvalidKEMAlgorithm
	}

	return params.PublicKeySize, params.SecretKeySize, params.CiphertextSize, params.SharedSecretSize, nil
}

// IsMLKEMSupported checks if ML-KEM is supported (always false in fallback mode)
func IsMLKEMSupported(algorithm string) bool {
	return false
}
