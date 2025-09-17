//go:build cgo && !no_liboqs
// +build cgo,!no_liboqs

// Copyright 2024 The Splendor Authors
// This file implements ML-DSA (Dilithium) signature verification for quantum resistance
// Based on FIPS 204 specification - CGO implementation with liboqs

package mldsa

/*
#cgo CFLAGS: -I${SRCDIR}/../liboqs/include
#cgo LDFLAGS: -L${SRCDIR}/../liboqs/lib -loqs
#include <oqs/oqs.h>
#include <oqs/sig.h>
#include <stdlib.h>
#include <string.h>

// ML-DSA algorithm identifiers
#define OQS_SIG_alg_ml_dsa_44 "ML-DSA-44"
#define OQS_SIG_alg_ml_dsa_65 "ML-DSA-65"
#define OQS_SIG_alg_ml_dsa_87 "ML-DSA-87"

// Wrapper function for ML-DSA verification
int verify_mldsa_signature(const char* alg_name, const uint8_t* message, size_t message_len,
                          const uint8_t* signature, size_t signature_len,
                          const uint8_t* public_key, size_t public_key_len) {
    OQS_SIG *sig = OQS_SIG_new(alg_name);
    if (sig == NULL) {
        return -1; // Algorithm not supported
    }
    
    // Verify signature lengths match expected values
    if (signature_len != sig->length_signature || public_key_len != sig->length_public_key) {
        OQS_SIG_free(sig);
        return -2; // Invalid length
    }
    
    OQS_STATUS status = OQS_SIG_verify(sig, message, message_len, signature, signature_len, public_key);
    OQS_SIG_free(sig);
    
    return (status == OQS_SUCCESS) ? 1 : 0;
}

// Get signature and public key lengths for ML-DSA variants
int get_mldsa_lengths(const char* alg_name, size_t* sig_len, size_t* pk_len) {
    OQS_SIG *sig = OQS_SIG_new(alg_name);
    if (sig == NULL) {
        return -1;
    }
    
    *sig_len = sig->length_signature;
    *pk_len = sig->length_public_key;
    
    OQS_SIG_free(sig);
    return 0;
}

// Generate ML-DSA key pair
int generate_mldsa_keypair(const char* alg_name, uint8_t* public_key, uint8_t* secret_key) {
    OQS_SIG *sig = OQS_SIG_new(alg_name);
    if (sig == NULL) {
        return -1;
    }
    
    OQS_STATUS status = OQS_SIG_keypair(sig, public_key, secret_key);
    OQS_SIG_free(sig);
    
    return (status == OQS_SUCCESS) ? 0 : -1;
}

// Sign message with ML-DSA
int sign_mldsa_message(const char* alg_name, const uint8_t* message, size_t message_len,
                      uint8_t* signature, size_t* signature_len,
                      const uint8_t* secret_key) {
    OQS_SIG *sig = OQS_SIG_new(alg_name);
    if (sig == NULL) {
        return -1;
    }
    
    OQS_STATUS status = OQS_SIG_sign(sig, signature, signature_len, message, message_len, secret_key);
    OQS_SIG_free(sig);
    
    return (status == OQS_SUCCESS) ? 0 : -1;
}

// Batch verification for multiple signatures
int batch_verify_mldsa_signatures(const char* alg_name, 
                                 const uint8_t** messages, const size_t* message_lens,
                                 const uint8_t** signatures, const size_t* signature_lens,
                                 const uint8_t** public_keys, const size_t* public_key_lens,
                                 size_t batch_size, int* results) {
    OQS_SIG *sig = OQS_SIG_new(alg_name);
    if (sig == NULL) {
        return -1;
    }
    
    int overall_result = 0;
    for (size_t i = 0; i < batch_size; i++) {
        // Verify signature lengths
        if (signature_lens[i] != sig->length_signature || public_key_lens[i] != sig->length_public_key) {
            results[i] = -2; // Invalid length
            overall_result = -2;
            continue;
        }
        
        OQS_STATUS status = OQS_SIG_verify(sig, messages[i], message_lens[i], 
                                          signatures[i], signature_lens[i], public_keys[i]);
        results[i] = (status == OQS_SUCCESS) ? 1 : 0;
        if (status != OQS_SUCCESS) {
            overall_result = -3; // At least one verification failed
        }
    }
    
    OQS_SIG_free(sig);
    return overall_result;
}

// Check if algorithm is supported
int is_mldsa_algorithm_supported(const char* alg_name) {
    OQS_SIG *sig = OQS_SIG_new(alg_name);
    if (sig == NULL) {
        return 0;
    }
    OQS_SIG_free(sig);
    return 1;
}
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// mapToOQSName maps Splendor ML-DSA identifiers to liboqs algorithm names.
func mapToOQSName(alg string) string {
	switch alg {
	case MLDSA44:
		return "Dilithium2"
	case MLDSA65:
		return "Dilithium3"
	case MLDSA87:
		return "Dilithium5"
	default:
		return alg
	}
}

// VerifySignature verifies an ML-DSA signature using liboqs
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

	// Convert Go strings and slices to C types
	cAlgName := C.CString(mapToOQSName(algorithm))
	defer C.free(unsafe.Pointer(cAlgName))

	var cMessage *C.uint8_t
	if len(message) > 0 {
		cMessage = (*C.uint8_t)(unsafe.Pointer(&message[0]))
	}

	cSignature := (*C.uint8_t)(unsafe.Pointer(&signature[0]))
	cPublicKey := (*C.uint8_t)(unsafe.Pointer(&publicKey[0]))

	// Call C verification function
	result := C.verify_mldsa_signature(
		cAlgName,
		cMessage, C.size_t(len(message)),
		cSignature, C.size_t(len(signature)),
		cPublicKey, C.size_t(len(publicKey)),
	)

	switch result {
	case 1:
		return nil // Verification successful
	case 0:
		return ErrVerificationFailed
	case -1:
		return ErrInvalidAlgorithm
	case -2:
		return ErrInvalidLength
	default:
		return ErrLibOQSNotAvailable
	}
}

// GetMLDSALengths returns the expected signature and public key lengths for an algorithm
func GetMLDSALengths(algorithm string) (sigLen, pkLen int, err error) {
	if _, exists := MLDSAParams[algorithm]; !exists {
		return 0, 0, ErrInvalidAlgorithm
	}

	cAlgName := C.CString(mapToOQSName(algorithm))
	defer C.free(unsafe.Pointer(cAlgName))

	var cSigLen, cPkLen C.size_t
	result := C.get_mldsa_lengths(cAlgName, &cSigLen, &cPkLen)

	if result != 0 {
		// Fallback to static sizes when liboqs can't provide them at runtime
		if p, ok := MLDSAParams[algorithm]; ok {
			return p.SignatureSize, p.PublicKeySize, nil
		}
		return 0, 0, ErrInvalidAlgorithm
	}

	return int(cSigLen), int(cPkLen), nil
}

// IsMLDSASupported checks if ML-DSA is supported by the linked liboqs
func IsMLDSASupported(algorithm string) bool {
	cAlgName := C.CString(mapToOQSName(algorithm))
	defer C.free(unsafe.Pointer(cAlgName))

	result := C.is_mldsa_algorithm_supported(cAlgName)
	return result == 1
}

// GenerateKeyPair generates an ML-DSA key pair
func GenerateKeyPair(algorithm string) (publicKey, secretKey []byte, err error) {
	if _, exists := MLDSAParams[algorithm]; !exists {
		return nil, nil, ErrInvalidAlgorithm
	}

	// Get key sizes
	sigLen, pkLen, err := GetMLDSALengths(algorithm)
	if err != nil {
		return nil, nil, err
	}

	// Calculate secret key size (typically 2.5x signature size for ML-DSA)
	skLen := sigLen + pkLen // Approximate secret key size

	publicKey = make([]byte, pkLen)
	secretKey = make([]byte, skLen)

	cAlgName := C.CString(mapToOQSName(algorithm))
	defer C.free(unsafe.Pointer(cAlgName))

	cPublicKey := (*C.uint8_t)(unsafe.Pointer(&publicKey[0]))
	cSecretKey := (*C.uint8_t)(unsafe.Pointer(&secretKey[0]))

	result := C.generate_mldsa_keypair(cAlgName, cPublicKey, cSecretKey)
	if result != 0 {
		return nil, nil, fmt.Errorf("key generation failed: %d", result)
	}

	return publicKey, secretKey, nil
}

// SignMessage signs a message with ML-DSA
func SignMessage(algorithm string, message, secretKey []byte) (signature []byte, err error) {
	if len(message) == 0 {
		return nil, errors.New("empty message")
	}
	if len(secretKey) == 0 {
		return nil, errors.New("empty secret key")
	}

	// Get signature size
	sigLen, _, err := GetMLDSALengths(algorithm)
	if err != nil {
		return nil, err
	}

	signature = make([]byte, sigLen)

	cAlgName := C.CString(mapToOQSName(algorithm))
	defer C.free(unsafe.Pointer(cAlgName))

	cMessage := (*C.uint8_t)(unsafe.Pointer(&message[0]))
	cSignature := (*C.uint8_t)(unsafe.Pointer(&signature[0]))
	cSecretKey := (*C.uint8_t)(unsafe.Pointer(&secretKey[0]))
	cSigLen := C.size_t(sigLen)

	result := C.sign_mldsa_message(
		cAlgName,
		cMessage, C.size_t(len(message)),
		cSignature, &cSigLen,
		cSecretKey,
	)

	if result != 0 {
		return nil, fmt.Errorf("signing failed: %d", result)
	}

	// Resize signature to actual length
	signature = signature[:int(cSigLen)]
	return signature, nil
}

// BatchVerifySignatures verifies multiple ML-DSA signatures efficiently
func BatchVerifySignatures(algorithm string, messages [][]byte, signatures [][]byte, publicKeys [][]byte) ([]bool, error) {
	if len(messages) != len(signatures) || len(signatures) != len(publicKeys) {
		return nil, errors.New("mismatched batch sizes")
	}

	batchSize := len(messages)
	if batchSize == 0 {
		return []bool{}, nil
	}

	// Prepare C arrays
	cMessages := make([]*C.uint8_t, batchSize)
	cMessageLens := make([]C.size_t, batchSize)
	cSignatures := make([]*C.uint8_t, batchSize)
	cSignatureLens := make([]C.size_t, batchSize)
	cPublicKeys := make([]*C.uint8_t, batchSize)
	cPublicKeyLens := make([]C.size_t, batchSize)
	cResults := make([]C.int, batchSize)

	for i := 0; i < batchSize; i++ {
		if len(messages[i]) > 0 {
			cMessages[i] = (*C.uint8_t)(unsafe.Pointer(&messages[i][0]))
		}
		cMessageLens[i] = C.size_t(len(messages[i]))

		if len(signatures[i]) > 0 {
			cSignatures[i] = (*C.uint8_t)(unsafe.Pointer(&signatures[i][0]))
		}
		cSignatureLens[i] = C.size_t(len(signatures[i]))

		if len(publicKeys[i]) > 0 {
			cPublicKeys[i] = (*C.uint8_t)(unsafe.Pointer(&publicKeys[i][0]))
		}
		cPublicKeyLens[i] = C.size_t(len(publicKeys[i]))
	}

	cAlgName := C.CString(mapToOQSName(algorithm))
	defer C.free(unsafe.Pointer(cAlgName))

	// Call batch verification
	C.batch_verify_mldsa_signatures(
		cAlgName,
		(**C.uint8_t)(unsafe.Pointer(&cMessages[0])),
		(*C.size_t)(unsafe.Pointer(&cMessageLens[0])),
		(**C.uint8_t)(unsafe.Pointer(&cSignatures[0])),
		(*C.size_t)(unsafe.Pointer(&cSignatureLens[0])),
		(**C.uint8_t)(unsafe.Pointer(&cPublicKeys[0])),
		(*C.size_t)(unsafe.Pointer(&cPublicKeyLens[0])),
		C.size_t(batchSize),
		(*C.int)(unsafe.Pointer(&cResults[0])),
	)

	// Convert results
	results := make([]bool, batchSize)
	for i := 0; i < batchSize; i++ {
		results[i] = cResults[i] == 1
	}

	return results, nil
}
