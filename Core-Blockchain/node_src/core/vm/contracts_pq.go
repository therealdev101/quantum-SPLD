// Copyright 2024 The Splendor Authors
// This file implements post-quantum cryptographic precompiles for the EVM
// Includes ML-DSA signature verification as specified in FIPS 204

package vm

import (
    "encoding/binary"
    "errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/mldsa"
	"github.com/ethereum/go-ethereum/params"
)

// Post-quantum precompile addresses
var (
	// ML-DSA signature verification precompile at 0x0100
	MLDSAVerifyAddress = common.BytesToAddress([]byte{0x01, 0x00})
)

// PostQuantumPrecompiles contains the post-quantum precompiled contracts
var PostQuantumPrecompiles = map[common.Address]PrecompiledContract{
    MLDSAVerifyAddress: &mldsaVerify{},
}

// Register post-quantum precompiles into the standard precompile sets so they are
// available without requiring external wiring. This keeps activation simple while
// allowing gas costs to be tuned via params.
func init() {
    // Attach ML-DSA verify to all standard precompile sets so calls to 0x0100 work
    // across forks. If you prefer conditional activation, guard this with chain
    // config flags and append conditionally.
    for addr, pc := range PostQuantumPrecompiles {
        // Homestead
        if PrecompiledContractsHomestead[addr] == nil {
            PrecompiledContractsHomestead[addr] = pc
            PrecompiledAddressesHomestead = append(PrecompiledAddressesHomestead, addr)
        }
        // Byzantium
        if PrecompiledContractsByzantium[addr] == nil {
            PrecompiledContractsByzantium[addr] = pc
            PrecompiledAddressesByzantium = append(PrecompiledAddressesByzantium, addr)
        }
        // Istanbul
        if PrecompiledContractsIstanbul[addr] == nil {
            PrecompiledContractsIstanbul[addr] = pc
            PrecompiledAddressesIstanbul = append(PrecompiledAddressesIstanbul, addr)
        }
        // Berlin
        if PrecompiledContractsBerlin[addr] == nil {
            PrecompiledContractsBerlin[addr] = pc
            PrecompiledAddressesBerlin = append(PrecompiledAddressesBerlin, addr)
        }
    }
}

// mldsaVerify implements ML-DSA signature verification precompile
type mldsaVerify struct{}

// RequiredGas calculates the gas cost for ML-DSA verification
// Base cost: 15,000 gas + 3 gas per byte of input data
func (c *mldsaVerify) RequiredGas(input []byte) uint64 {
	baseGas := params.MLDSAVerifyBaseGas
	if baseGas == 0 {
		baseGas = 15000 // Default base gas cost
	}
	
	// Per-byte cost for message, signature, and public key
	perByteGas := params.MLDSAVerifyPerByteGas
	if perByteGas == 0 {
		perByteGas = 3 // Default per-byte gas cost
	}
	
	return baseGas + uint64(len(input))*perByteGas
}

// Run executes the ML-DSA signature verification
// Input format: [algorithm_id(1)] + [message_len(4)] + [signature_len(4)] + [pubkey_len(4)] + [message] + [signature] + [pubkey]
func (c *mldsaVerify) Run(input []byte) ([]byte, error) {
	// Minimum input: 1 + 4 + 4 + 4 = 13 bytes for headers
	if len(input) < 13 {
		return nil, errors.New("input too short")
	}

	// Parse algorithm ID
	algorithmID := input[0]
	var algorithm string
	switch algorithmID {
	case 0x44: // ML-DSA-44
		algorithm = mldsa.MLDSA44
	case 0x65: // ML-DSA-65
		algorithm = mldsa.MLDSA65
	case 0x87: // ML-DSA-87
		algorithm = mldsa.MLDSA87
	default:
		return nil, errors.New("unsupported ML-DSA algorithm")
	}

	// Parse lengths
	messageLen := binary.BigEndian.Uint32(input[1:5])
	signatureLen := binary.BigEndian.Uint32(input[5:9])
	pubkeyLen := binary.BigEndian.Uint32(input[9:13])

	// Validate total length
	expectedLen := 13 + messageLen + signatureLen + pubkeyLen
	if uint32(len(input)) != expectedLen {
		return nil, errors.New("input length mismatch")
	}

	// Extract components
	offset := uint32(13)
	message := input[offset : offset+messageLen]
	offset += messageLen
	signature := input[offset : offset+signatureLen]
	offset += signatureLen
	publicKey := input[offset : offset+pubkeyLen]

	// Verify signature
	err := mldsa.VerifySignature(algorithm, message, signature, publicKey)
	if err != nil {
		// Return false (32 bytes of zeros) for verification failure
		return make([]byte, 32), nil
	}

	// Return true (32 bytes with last byte = 1) for successful verification
	result := make([]byte, 32)
	result[31] = 1
	return result, nil
}

// mldsaVerifyCompact implements a compact ML-DSA verification precompile
// This version assumes ML-DSA-65 and uses a simpler input format
type mldsaVerifyCompact struct{}

// RequiredGas for compact version
func (c *mldsaVerifyCompact) RequiredGas(input []byte) uint64 {
	baseGas := params.MLDSAVerifyBaseGas
	if baseGas == 0 {
		baseGas = 12000 // Slightly lower base cost for compact version
	}
	
	perByteGas := params.MLDSAVerifyPerByteGas
	if perByteGas == 0 {
		perByteGas = 2 // Lower per-byte cost
	}
	
	return baseGas + uint64(len(input))*perByteGas
}

// Run executes compact ML-DSA-65 verification
// Input format: [message_len(4)] + [message] + [signature(3309)] + [pubkey(1952)]
// Fixed sizes for ML-DSA-65: signature=3309 bytes, pubkey=1952 bytes
func (c *mldsaVerifyCompact) Run(input []byte) ([]byte, error) {
	const (
		ML_DSA_65_SIG_LEN = 3309
		ML_DSA_65_PK_LEN  = 1952
		MIN_INPUT_LEN     = 4 + ML_DSA_65_SIG_LEN + ML_DSA_65_PK_LEN
	)

	if len(input) < MIN_INPUT_LEN {
		return nil, errors.New("input too short for ML-DSA-65")
	}

	// Parse message length
	messageLen := binary.BigEndian.Uint32(input[0:4])
	expectedLen := 4 + messageLen + ML_DSA_65_SIG_LEN + ML_DSA_65_PK_LEN
	
	if uint32(len(input)) != expectedLen {
		return nil, errors.New("input length mismatch")
	}

	// Extract components
	message := input[4 : 4+messageLen]
	signature := input[4+messageLen : 4+messageLen+ML_DSA_65_SIG_LEN]
	publicKey := input[4+messageLen+ML_DSA_65_SIG_LEN : 4+messageLen+ML_DSA_65_SIG_LEN+ML_DSA_65_PK_LEN]

	// Verify signature using ML-DSA-65
	err := mldsa.VerifySignature(mldsa.MLDSA65, message, signature, publicKey)
	if err != nil {
		// Return false for verification failure
		return make([]byte, 32), nil
	}

	// Return true for successful verification
	result := make([]byte, 32)
	result[31] = 1
	return result, nil
}

// Helper function to get all post-quantum precompile addresses
func PostQuantumPrecompileAddresses() []common.Address {
	addresses := make([]common.Address, 0, len(PostQuantumPrecompiles))
	for addr := range PostQuantumPrecompiles {
		addresses = append(addresses, addr)
	}
	return addresses
}

// Helper function to check if an address is a post-quantum precompile
func IsPostQuantumPrecompile(addr common.Address) bool {
	_, exists := PostQuantumPrecompiles[addr]
	return exists
}

// RunPostQuantumPrecompile executes a post-quantum precompiled contract
func RunPostQuantumPrecompile(addr common.Address, input []byte, suppliedGas uint64) (ret []byte, remainingGas uint64, err error) {
	precompile, exists := PostQuantumPrecompiles[addr]
	if !exists {
		return nil, suppliedGas, errors.New("precompile not found")
	}
	
	return RunPrecompiledContract(precompile, input, suppliedGas)
}
