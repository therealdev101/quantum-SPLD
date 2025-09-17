//go:build cgo && liboqs
// +build cgo,liboqs

// Shared ML-DSA constants, parameters, and validation for CGO-enabled builds
// This complements mldsa_cgo.go. The fallback implementation with build tag
// !cgo || no_liboqs defines its own copies to avoid duplicate symbols.

package mldsa

import "errors"

// ML-DSA algorithm variants
const (
	MLDSA44 = "ML-DSA-44" // Compact variant for user transactions
	MLDSA65 = "ML-DSA-65" // Recommended variant for consensus
	MLDSA87 = "ML-DSA-87" // High security variant
)

// ML-DSA parameter sets (approximate sizes in bytes)
var MLDSAParams = map[string]struct {
	PublicKeySize int
	SignatureSize int
	SecurityLevel int
}{
	MLDSA44: {PublicKeySize: 1312, SignatureSize: 2420, SecurityLevel: 2},
	MLDSA65: {PublicKeySize: 1952, SignatureSize: 3293, SecurityLevel: 3},
	MLDSA87: {PublicKeySize: 2592, SignatureSize: 4595, SecurityLevel: 5},
}

var (
	ErrInvalidAlgorithm   = errors.New("invalid ML-DSA algorithm")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrInvalidPublicKey   = errors.New("invalid public key")
	ErrInvalidLength      = errors.New("invalid signature or public key length")
	ErrVerificationFailed = errors.New("signature verification failed")
	ErrLibOQSNotAvailable = errors.New("liboqs library not available")
)

// ValidateMLDSAParams validates ML-DSA signature and public key lengths.
// In CGO-enabled builds, this uses sizes provided by GetMLDSALengths from cgo.
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
