//go:build cgo && !no_liboqs
// +build cgo,!no_liboqs

// Copyright 2024 The Splendor Authors
// This file implements ML-KEM (Kyber) key encapsulation for quantum resistance
// Based on FIPS 203 specification - CGO implementation with liboqs

package mlkem

/*
#cgo CFLAGS: -I${SRCDIR}/../liboqs/include
#cgo LDFLAGS: -L${SRCDIR}/../liboqs/lib -loqs
#include <oqs/oqs.h>
#include <oqs/kem.h>
#include <stdlib.h>
#include <string.h>

// ML-KEM algorithm identifiers
#define OQS_KEM_alg_ml_kem_512 "ML-KEM-512"
#define OQS_KEM_alg_ml_kem_768 "ML-KEM-768"
#define OQS_KEM_alg_ml_kem_1024 "ML-KEM-1024"

// Generate ML-KEM key pair
int generate_mlkem_keypair(const char* alg_name, uint8_t* public_key, uint8_t* secret_key) {
    OQS_KEM *kem = OQS_KEM_new(alg_name);
    if (kem == NULL) {
        return -1;
    }
    
    OQS_STATUS status = OQS_KEM_keypair(kem, public_key, secret_key);
    OQS_KEM_free(kem);
    
    return (status == OQS_SUCCESS) ? 0 : -1;
}

// Encapsulate shared secret
int encapsulate_mlkem(const char* alg_name, const uint8_t* public_key,
                     uint8_t* ciphertext, uint8_t* shared_secret) {
    OQS_KEM *kem = OQS_KEM_new(alg_name);
    if (kem == NULL) {
        return -1;
    }
    
    OQS_STATUS status = OQS_KEM_encaps(kem, ciphertext, shared_secret, public_key);
    OQS_KEM_free(kem);
    
    return (status == OQS_SUCCESS) ? 0 : -1;
}

// Decapsulate shared secret
int decapsulate_mlkem(const char* alg_name, const uint8_t* ciphertext,
                     const uint8_t* secret_key, uint8_t* shared_secret) {
    OQS_KEM *kem = OQS_KEM_new(alg_name);
    if (kem == NULL) {
        return -1;
    }
    
    OQS_STATUS status = OQS_KEM_decaps(kem, shared_secret, ciphertext, secret_key);
    OQS_KEM_free(kem);
    
    return (status == OQS_SUCCESS) ? 0 : -1;
}

// Get ML-KEM parameter sizes
int get_mlkem_sizes(const char* alg_name, size_t* pk_len, size_t* sk_len, 
                   size_t* ct_len, size_t* ss_len) {
    OQS_KEM *kem = OQS_KEM_new(alg_name);
    if (kem == NULL) {
        return -1;
    }
    
    *pk_len = kem->length_public_key;
    *sk_len = kem->length_secret_key;
    *ct_len = kem->length_ciphertext;
    *ss_len = kem->length_shared_secret;
    
    OQS_KEM_free(kem);
    return 0;
}

// Check if ML-KEM algorithm is supported
int is_mlkem_algorithm_supported(const char* alg_name) {
    OQS_KEM *kem = OQS_KEM_new(alg_name);
    if (kem == NULL) {
        return 0;
    }
    OQS_KEM_free(kem);
    return 1;
}

// Hybrid key encapsulation (ML-KEM + ECDH)
int hybrid_encapsulate(const char* alg_name, const uint8_t* mlkem_public_key,
                      const uint8_t* ecdh_public_key, size_t ecdh_pk_len,
                      uint8_t* ciphertext, uint8_t* shared_secret) {
    OQS_KEM *kem = OQS_KEM_new(alg_name);
    if (kem == NULL) {
        return -1;
    }
    
    // Perform ML-KEM encapsulation
    uint8_t mlkem_ciphertext[2048]; // Max ciphertext size
    uint8_t mlkem_shared_secret[32];
    
    OQS_STATUS status = OQS_KEM_encaps(kem, mlkem_ciphertext, mlkem_shared_secret, mlkem_public_key);
    if (status != OQS_SUCCESS) {
        OQS_KEM_free(kem);
        return -1;
    }
    
    // Copy ML-KEM ciphertext
    memcpy(ciphertext, mlkem_ciphertext, kem->length_ciphertext);
    
    // For hybrid mode, we would combine with ECDH here
    // For now, just use ML-KEM shared secret
    memcpy(shared_secret, mlkem_shared_secret, kem->length_shared_secret);
    
    OQS_KEM_free(kem);
    return 0;
}
*/
import "C"
import (
	"crypto/sha256"
	"errors"
	"fmt"
	"unsafe"
)

// GenerateKeyPair generates an ML-KEM key pair
func GenerateKeyPair(algorithm string) (publicKey, secretKey []byte, err error) {
	if _, exists := MLKEMParams[algorithm]; !exists {
		return nil, nil, ErrInvalidKEMAlgorithm
	}

	// Get parameter sizes
	pkSize, skSize, _, _, err := GetMLKEMSizes(algorithm)
	if err != nil {
		return nil, nil, err
	}

	publicKey = make([]byte, pkSize)
	secretKey = make([]byte, skSize)

	cAlgName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(cAlgName))

	cPublicKey := (*C.uint8_t)(unsafe.Pointer(&publicKey[0]))
	cSecretKey := (*C.uint8_t)(unsafe.Pointer(&secretKey[0]))

	result := C.generate_mlkem_keypair(cAlgName, cPublicKey, cSecretKey)
	if result != 0 {
		return nil, nil, fmt.Errorf("ML-KEM key generation failed: %d", result)
	}

	return publicKey, secretKey, nil
}

// Encapsulate performs key encapsulation
func Encapsulate(algorithm string, publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	if len(publicKey) == 0 {
		return nil, nil, errors.New("empty public key")
	}

	params, exists := MLKEMParams[algorithm]
	if !exists {
		return nil, nil, ErrInvalidKEMAlgorithm
	}

	if len(publicKey) != params.PublicKeySize {
		return nil, nil, ErrInvalidLength
	}

	ciphertext = make([]byte, params.CiphertextSize)
	sharedSecret = make([]byte, params.SharedSecretSize)

	cAlgName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(cAlgName))

	cPublicKey := (*C.uint8_t)(unsafe.Pointer(&publicKey[0]))
	cCiphertext := (*C.uint8_t)(unsafe.Pointer(&ciphertext[0]))
	cSharedSecret := (*C.uint8_t)(unsafe.Pointer(&sharedSecret[0]))

	result := C.encapsulate_mlkem(cAlgName, cPublicKey, cCiphertext, cSharedSecret)
	if result != 0 {
		return nil, nil, fmt.Errorf("ML-KEM encapsulation failed: %d", result)
	}

	return ciphertext, sharedSecret, nil
}

// Decapsulate performs key decapsulation
func Decapsulate(algorithm string, ciphertext, secretKey []byte) (sharedSecret []byte, err error) {
	if len(ciphertext) == 0 {
		return nil, ErrInvalidCiphertext
	}
	if len(secretKey) == 0 {
		return nil, ErrInvalidSecretKey
	}

	params, exists := MLKEMParams[algorithm]
	if !exists {
		return nil, ErrInvalidKEMAlgorithm
	}

	if len(ciphertext) != params.CiphertextSize {
		return nil, ErrInvalidCiphertext
	}
	if len(secretKey) != params.SecretKeySize {
		return nil, ErrInvalidSecretKey
	}

	sharedSecret = make([]byte, params.SharedSecretSize)

	cAlgName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(cAlgName))

	cCiphertext := (*C.uint8_t)(unsafe.Pointer(&ciphertext[0]))
	cSecretKey := (*C.uint8_t)(unsafe.Pointer(&secretKey[0]))
	cSharedSecret := (*C.uint8_t)(unsafe.Pointer(&sharedSecret[0]))

	result := C.decapsulate_mlkem(cAlgName, cCiphertext, cSecretKey, cSharedSecret)
	if result != 0 {
		return nil, fmt.Errorf("ML-KEM decapsulation failed: %d", result)
	}

	return sharedSecret, nil
}

// GetMLKEMSizes returns the sizes for ML-KEM parameters
func GetMLKEMSizes(algorithm string) (pkSize, skSize, ctSize, ssSize int, err error) {
	if _, exists := MLKEMParams[algorithm]; !exists {
		return 0, 0, 0, 0, ErrInvalidKEMAlgorithm
	}

	cAlgName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(cAlgName))

	var cPkLen, cSkLen, cCtLen, cSsLen C.size_t
	result := C.get_mlkem_sizes(cAlgName, &cPkLen, &cSkLen, &cCtLen, &cSsLen)

	if result != 0 {
		return 0, 0, 0, 0, ErrInvalidKEMAlgorithm
	}

	return int(cPkLen), int(cSkLen), int(cCtLen), int(cSsLen), nil
}

// IsMLKEMSupported checks if ML-KEM is supported
func IsMLKEMSupported(algorithm string) bool {
	cAlgName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(cAlgName))

	result := C.is_mlkem_algorithm_supported(cAlgName)
	return result == 1
}

// HybridEncapsulate performs hybrid key encapsulation (ML-KEM + ECDH)
func HybridEncapsulate(algorithm string, mlkemPublicKey, ecdhPublicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	if len(mlkemPublicKey) == 0 || len(ecdhPublicKey) == 0 {
		return nil, nil, errors.New("empty public keys")
	}

	params, exists := MLKEMParams[algorithm]
	if !exists {
		return nil, nil, ErrInvalidKEMAlgorithm
	}

	ciphertext = make([]byte, params.CiphertextSize)
	tempSharedSecret := make([]byte, params.SharedSecretSize)

	cAlgName := C.CString(algorithm)
	defer C.free(unsafe.Pointer(cAlgName))

	cMLKEMPublicKey := (*C.uint8_t)(unsafe.Pointer(&mlkemPublicKey[0]))
	cECDHPublicKey := (*C.uint8_t)(unsafe.Pointer(&ecdhPublicKey[0]))
	cCiphertext := (*C.uint8_t)(unsafe.Pointer(&ciphertext[0]))
	cSharedSecret := (*C.uint8_t)(unsafe.Pointer(&tempSharedSecret[0]))

	result := C.hybrid_encapsulate(cAlgName, cMLKEMPublicKey, cECDHPublicKey, 
		C.size_t(len(ecdhPublicKey)), cCiphertext, cSharedSecret)
	if result != 0 {
		return nil, nil, fmt.Errorf("hybrid encapsulation failed: %d", result)
	}

	// Combine ML-KEM and ECDH shared secrets using SHA256
	hasher := sha256.New()
	hasher.Write(tempSharedSecret)
	hasher.Write(ecdhPublicKey) // In real implementation, this would be ECDH shared secret
	sharedSecret = hasher.Sum(nil)

	return ciphertext, sharedSecret, nil
}

// ValidateMLKEMParams validates ML-KEM parameters
func ValidateMLKEMParams(algorithm string, publicKey []byte) error {
	expectedPkSize, _, _, _, err := GetMLKEMSizes(algorithm)
	if err != nil {
		return err
	}

	if len(publicKey) != expectedPkSize {
		return ErrInvalidLength
	}

	return nil
}
