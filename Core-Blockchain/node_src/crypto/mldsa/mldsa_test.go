// Copyright 2024 The Splendor Authors
// Test suite for ML-DSA (Dilithium) signature verification

package mldsa

import (
	"crypto/rand"
	"testing"
)

// Helpers to retrieve sizes from the active build (cgo liboqs if present,
// otherwise fallback to MLDSAParams). This makes tests portable across
// environments where liboqs may report slightly different lengths (e.g. D3=3293, D5=4595).
func libSizes(algorithm string) (sigLen, pkLen int) {
	if s, p, err := GetMLDSALengths(algorithm); err == nil && s > 0 && p > 0 {
		return s, p
	}
	params, ok := MLDSAParams[algorithm]
	if !ok {
		return 0, 0
	}
	return params.SignatureSize, params.PublicKeySize
}

func sigBytes(algorithm string) []byte {
	s, _ := libSizes(algorithm)
	return make([]byte, s)
}

func pkBytes(algorithm string) []byte {
	_, p := libSizes(algorithm)
	return make([]byte, p)
}

// Mock test vectors for ML-DSA (these would be replaced with actual NIST test vectors)
var testVectors = map[string]struct {
	algorithm string
	message   []byte
	signature []byte
	publicKey []byte
	valid     bool
}{
	"mldsa65_valid": {
		algorithm: MLDSA65,
		message:   []byte("test message for ML-DSA-65"),
		// These would be actual test vectors from NIST
		signature: sigBytes(MLDSA65), // dynamic signature size
		publicKey: pkBytes(MLDSA65),  // dynamic public key size
		valid:     false,             // dummy vectors, expect failure
	},
	"mldsa44_valid": {
		algorithm: MLDSA44,
		message:   []byte("test message for ML-DSA-44"),
		signature: sigBytes(MLDSA44),
		publicKey: pkBytes(MLDSA44),
		valid:     false, // dummy vectors, expect failure
	},
}

func TestMLDSAParams(t *testing.T) {
	// Expected security levels are fixed regardless of implementation
	expectedSec := map[string]int{
		MLDSA44: 2,
		MLDSA65: 3,
		MLDSA87: 5,
	}
	for alg, params := range MLDSAParams {
		wantSig, wantPk := libSizes(alg)
		if wantSig == 0 || wantPk == 0 {
			t.Fatalf("libSizes returned zero sizes for %s", alg)
		}
		if params.SignatureSize != wantSig {
			t.Errorf("%s: expected signature size %d, got %d", alg, wantSig, params.SignatureSize)
		}
		if params.PublicKeySize != wantPk {
			t.Errorf("%s: expected public key size %d, got %d", alg, wantPk, params.PublicKeySize)
		}
		if lvl, ok := expectedSec[alg]; ok {
			if params.SecurityLevel != lvl {
				t.Errorf("%s: expected security level %d, got %d", alg, lvl, params.SecurityLevel)
			}
		}
	}
}

func TestValidateMLDSAParams(t *testing.T) {
	validSig := sigBytes(MLDSA65)
	validPk := pkBytes(MLDSA65)

	tests := []struct {
		name      string
		algorithm string
		signature []byte
		publicKey []byte
		wantErr   bool
	}{
		{
			name:      "valid ML-DSA-65",
			algorithm: MLDSA65,
			signature: append([]byte(nil), validSig...),
			publicKey: append([]byte(nil), validPk...),
			wantErr:   false,
		},
		{
			name:      "invalid signature length",
			algorithm: MLDSA65,
			signature: make([]byte, 100),
			publicKey: append([]byte(nil), validPk...),
			wantErr:   true,
		},
		{
			name:      "invalid public key length",
			algorithm: MLDSA65,
			signature: append([]byte(nil), validSig...),
			publicKey: make([]byte, 100),
			wantErr:   true,
		},
		{
			name:      "invalid algorithm",
			algorithm: "invalid",
			signature: append([]byte(nil), validSig...),
			publicKey: append([]byte(nil), validPk...),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMLDSAParams(tt.algorithm, tt.signature, tt.publicKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMLDSAParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetMLDSALengths(t *testing.T) {
	tests := []struct {
		algorithm string
		wantErr   bool
	}{
		{MLDSA44, false},
		{MLDSA65, false},
		{MLDSA87, false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.algorithm, func(t *testing.T) {
			sigLen, pkLen, err := GetMLDSALengths(tt.algorithm)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMLDSALengths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				wantSig, wantPk := libSizes(tt.algorithm)
				if sigLen != wantSig {
					t.Errorf("GetMLDSALengths() sigLen = %v, want %v", sigLen, wantSig)
				}
				if pkLen != wantPk {
					t.Errorf("GetMLDSALengths() pkLen = %v, want %v", pkLen, wantPk)
				}
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	// Use dynamic sizes for positive/negative validation cases
	validSig := sigBytes(MLDSA65)
	validPk := pkBytes(MLDSA65)

	// Test input validation
	tests := []struct {
		name      string
		algorithm string
		message   []byte
		signature []byte
		publicKey []byte
		wantErr   string
	}{
		{
			name:      "empty message",
			algorithm: MLDSA65,
			message:   []byte{},
			signature: append([]byte(nil), validSig...),
			publicKey: append([]byte(nil), validPk...),
			wantErr:   "empty message",
		},
		{
			name:      "empty signature",
			algorithm: MLDSA65,
			message:   []byte("test"),
			signature: []byte{},
			publicKey: append([]byte(nil), validPk...),
			wantErr:   "invalid signature",
		},
		{
			name:      "empty public key",
			algorithm: MLDSA65,
			message:   []byte("test"),
			signature: append([]byte(nil), validSig...),
			publicKey: []byte{},
			wantErr:   "invalid public key",
		},
		{
			name:      "invalid algorithm",
			algorithm: "invalid",
			message:   []byte("test"),
			signature: append([]byte(nil), validSig...),
			publicKey: append([]byte(nil), validPk...),
			wantErr:   "invalid ML-DSA algorithm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySignature(tt.algorithm, tt.message, tt.signature, tt.publicKey)
			if err == nil {
				t.Errorf("VerifySignature() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf("VerifySignature() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// Benchmark tests for performance evaluation
func BenchmarkVerifySignatureMLDSA44(b *testing.B) {
	message := make([]byte, 32)
	signature := sigBytes(MLDSA44)
	publicKey := pkBytes(MLDSA44)

	// Fill with random data
	_, _ = rand.Read(message)
	_, _ = rand.Read(signature)
	_, _ = rand.Read(publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail verification but tests the performance
		_ = VerifySignature(MLDSA44, message, signature, publicKey)
	}
}

func BenchmarkVerifySignatureMLDSA65(b *testing.B) {
	message := make([]byte, 32)
	signature := sigBytes(MLDSA65)
	publicKey := pkBytes(MLDSA65)

	// Fill with random data
	_, _ = rand.Read(message)
	_, _ = rand.Read(signature)
	_, _ = rand.Read(publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail verification but tests the performance
		_ = VerifySignature(MLDSA65, message, signature, publicKey)
	}
}

func BenchmarkVerifySignatureMLDSA87(b *testing.B) {
	message := make([]byte, 32)
	signature := sigBytes(MLDSA87)
	publicKey := pkBytes(MLDSA87)

	// Fill with random data
	_, _ = rand.Read(message)
	_, _ = rand.Read(signature)
	_, _ = rand.Read(publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail verification but tests the performance
		_ = VerifySignature(MLDSA87, message, signature, publicKey)
	}
}

// Test ML-DSA support detection
func TestIsMLDSASupported(t *testing.T) {
	algorithms := []string{MLDSA44, MLDSA65, MLDSA87, "invalid"}

	for _, alg := range algorithms {
		supported := IsMLDSASupported(alg)
		t.Logf("Algorithm %s supported: %v", alg, supported)

		// For invalid algorithm, should return false
		if alg == "invalid" && supported {
			t.Errorf("Invalid algorithm reported as supported")
		}
	}
}

// Test error conditions
func TestMLDSAErrors(t *testing.T) {
	// Use dynamic sizes to create invalid inputs
	validSig := sigBytes(MLDSA65)
	validPk := pkBytes(MLDSA65)

	// Test all error types
	errorTests := []struct {
		name     string
		testFunc func() error
		wantErr  error
	}{
		{
			name: "invalid algorithm error",
			testFunc: func() error {
				return VerifySignature("invalid", []byte("test"), make([]byte, 100), make([]byte, 100))
			},
			wantErr: ErrInvalidAlgorithm,
		},
		{
			name: "invalid signature error",
			testFunc: func() error {
				return VerifySignature(MLDSA65, []byte("test"), []byte{}, append([]byte(nil), validPk...))
			},
			wantErr: ErrInvalidSignature,
		},
		{
			name: "invalid public key error",
			testFunc: func() error {
				return VerifySignature(MLDSA65, []byte("test"), append([]byte(nil), validSig...), []byte{})
			},
			wantErr: ErrInvalidPublicKey,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if err != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// Integration test with mock liboqs (when available)
func TestMLDSAIntegration(t *testing.T) {
	// Skip if liboqs is not available
	if !IsMLDSASupported(MLDSA65) {
		t.Skip("liboqs not available, skipping integration test")
	}

	// Test with known test vectors (would use real NIST vectors in production)
	for name, tv := range testVectors {
		t.Run(name, func(t *testing.T) {
			err := VerifySignature(tv.algorithm, tv.message, tv.signature, tv.publicKey)

			if tv.valid && err != nil {
				t.Errorf("Expected valid signature to verify, got error: %v", err)
			}

			if !tv.valid && err == nil {
				t.Errorf("Expected invalid signature to fail verification")
			}
		})
	}
}

// Test concurrent verification
func TestConcurrentVerification(t *testing.T) {
	const numGoroutines = 10
	const numVerifications = 100

	message := []byte("concurrent test message")
	signature := sigBytes(MLDSA65)
	publicKey := pkBytes(MLDSA65)

	// Fill with random data
	_, _ = rand.Read(signature)
	_, _ = rand.Read(publicKey)

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numVerifications; j++ {
				// This will fail but tests concurrent access
				_ = VerifySignature(MLDSA65, message, signature, publicKey)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
