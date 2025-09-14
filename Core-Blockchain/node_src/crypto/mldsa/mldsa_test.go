// Copyright 2024 The Splendor Authors
// Test suite for ML-DSA (Dilithium) signature verification

package mldsa

import (
	"crypto/rand"
	"testing"
)

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
		signature: make([]byte, 3309), // ML-DSA-65 signature size
		publicKey: make([]byte, 1952), // ML-DSA-65 public key size
		valid:     false, // Set to false since these are dummy vectors
	},
	"mldsa44_valid": {
		algorithm: MLDSA44,
		message:   []byte("test message for ML-DSA-44"),
		signature: make([]byte, 2420), // ML-DSA-44 signature size
		publicKey: make([]byte, 1312), // ML-DSA-44 public key size
		valid:     false, // Set to false since these are dummy vectors
	},
}

func TestMLDSAParams(t *testing.T) {
	// Test parameter constants
	expectedParams := map[string]struct {
		pubKeySize int
		sigSize    int
		secLevel   int
	}{
		MLDSA44: {1312, 2420, 2},
		MLDSA65: {1952, 3309, 3},
		MLDSA87: {2592, 4627, 5},
	}

	for alg, expected := range expectedParams {
		params, exists := MLDSAParams[alg]
		if !exists {
			t.Errorf("Algorithm %s not found in MLDSAParams", alg)
			continue
		}

		if params.PublicKeySize != expected.pubKeySize {
			t.Errorf("Algorithm %s: expected public key size %d, got %d",
				alg, expected.pubKeySize, params.PublicKeySize)
		}

		if params.SignatureSize != expected.sigSize {
			t.Errorf("Algorithm %s: expected signature size %d, got %d",
				alg, expected.sigSize, params.SignatureSize)
		}

		if params.SecurityLevel != expected.secLevel {
			t.Errorf("Algorithm %s: expected security level %d, got %d",
				alg, expected.secLevel, params.SecurityLevel)
		}
	}
}

func TestValidateMLDSAParams(t *testing.T) {
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
			signature: make([]byte, 3309),
			publicKey: make([]byte, 1952),
			wantErr:   false,
		},
		{
			name:      "invalid signature length",
			algorithm: MLDSA65,
			signature: make([]byte, 100),
			publicKey: make([]byte, 1952),
			wantErr:   true,
		},
		{
			name:      "invalid public key length",
			algorithm: MLDSA65,
			signature: make([]byte, 3309),
			publicKey: make([]byte, 100),
			wantErr:   true,
		},
		{
			name:      "invalid algorithm",
			algorithm: "invalid",
			signature: make([]byte, 3309),
			publicKey: make([]byte, 1952),
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
		wantSig   int
		wantPk    int
		wantErr   bool
	}{
		{MLDSA44, 2420, 1312, false},
		{MLDSA65, 3309, 1952, false},
		{MLDSA87, 4627, 2592, false},
		{"invalid", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.algorithm, func(t *testing.T) {
			sigLen, pkLen, err := GetMLDSALengths(tt.algorithm)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMLDSALengths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if sigLen != tt.wantSig {
					t.Errorf("GetMLDSALengths() sigLen = %v, want %v", sigLen, tt.wantSig)
				}
				if pkLen != tt.wantPk {
					t.Errorf("GetMLDSALengths() pkLen = %v, want %v", pkLen, tt.wantPk)
				}
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
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
			signature: make([]byte, 3309),
			publicKey: make([]byte, 1952),
			wantErr:   "empty message",
		},
		{
			name:      "empty signature",
			algorithm: MLDSA65,
			message:   []byte("test"),
			signature: []byte{},
			publicKey: make([]byte, 1952),
			wantErr:   "invalid signature",
		},
		{
			name:      "empty public key",
			algorithm: MLDSA65,
			message:   []byte("test"),
			signature: make([]byte, 3309),
			publicKey: []byte{},
			wantErr:   "invalid public key",
		},
		{
			name:      "invalid algorithm",
			algorithm: "invalid",
			message:   []byte("test"),
			signature: make([]byte, 3309),
			publicKey: make([]byte, 1952),
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
	signature := make([]byte, 2420)
	publicKey := make([]byte, 1312)
	
	// Fill with random data
	rand.Read(message)
	rand.Read(signature)
	rand.Read(publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail verification but tests the performance
		VerifySignature(MLDSA44, message, signature, publicKey)
	}
}

func BenchmarkVerifySignatureMLDSA65(b *testing.B) {
	message := make([]byte, 32)
	signature := make([]byte, 3309)
	publicKey := make([]byte, 1952)
	
	// Fill with random data
	rand.Read(message)
	rand.Read(signature)
	rand.Read(publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail verification but tests the performance
		VerifySignature(MLDSA65, message, signature, publicKey)
	}
}

func BenchmarkVerifySignatureMLDSA87(b *testing.B) {
	message := make([]byte, 32)
	signature := make([]byte, 4627)
	publicKey := make([]byte, 2592)
	
	// Fill with random data
	rand.Read(message)
	rand.Read(signature)
	rand.Read(publicKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail verification but tests the performance
		VerifySignature(MLDSA87, message, signature, publicKey)
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
				return VerifySignature(MLDSA65, []byte("test"), []byte{}, make([]byte, 1952))
			},
			wantErr: ErrInvalidSignature,
		},
		{
			name: "invalid public key error",
			testFunc: func() error {
				return VerifySignature(MLDSA65, []byte("test"), make([]byte, 3309), []byte{})
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
	signature := make([]byte, 3309)
	publicKey := make([]byte, 1952)

	// Fill with random data
	rand.Read(signature)
	rand.Read(publicKey)

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numVerifications; j++ {
				// This will fail but tests concurrent access
				VerifySignature(MLDSA65, message, signature, publicKey)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
