//go:build cgo && gpu

package gpu

/*
#include <stdint.h>
*/
import "C"

import (
	"encoding/binary"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	hashOutputSize  = 32
	signatureSize   = 65
	messageSize     = 32
	publicKeySize   = 65
	txResultSize    = 64
	txGasOffset     = 40
	txChainIDOffset = 48
	txNonceOffset   = 56
)

func sliceFromCPtr(ptr *C.uchar, length C.int) []byte {
	if length == 0 {
		return []byte{}
	}
	if ptr == nil || length < 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(length))
}

func fixedSlice(ptr *C.uchar, length int) []byte {
	if length == 0 {
		return []byte{}
	}
	if ptr == nil || length < 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)
}

//export go_keccak256
func go_keccak256(input *C.uchar, length C.int, output *C.uchar) {
	out := fixedSlice(output, hashOutputSize)
	if len(out) != hashOutputSize {
		return
	}
	in := sliceFromCPtr(input, length)
	if in == nil {
		for i := range out {
			out[i] = 0
		}
		return
	}
	hash := crypto.Keccak256(in)
	copy(out, hash)
}

//export go_verify_signature
func go_verify_signature(signature *C.uchar, message *C.uchar, publicKey *C.uchar) C.int {
	sig := fixedSlice(signature, signatureSize)
	msg := fixedSlice(message, messageSize)
	key := fixedSlice(publicKey, publicKeySize)
	if len(sig) != signatureSize || len(msg) != messageSize || len(key) != publicKeySize {
		return 0
	}
	if crypto.VerifySignature(key, msg, sig[:64]) {
		return 1
	}
	return 0
}

//export go_process_transaction
func go_process_transaction(txPtr *C.uchar, length C.int, output *C.uchar) C.int {
	out := fixedSlice(output, txResultSize)
	if len(out) != txResultSize {
		return -1
	}
	for i := range out {
		out[i] = 0
	}
	if txPtr == nil || length <= 0 {
		out[32] = 0
		out[33] = 1 // malformed input
		return 0
	}
	raw := sliceFromCPtr(txPtr, length)
	if raw == nil {
		out[33] = 1
		return 0
	}
	// Make a defensive copy as types.Transaction expects the slice to remain valid.
	encoded := make([]byte, len(raw))
	copy(encoded, raw)

	var tx types.Transaction
	if err := tx.UnmarshalBinary(encoded); err != nil {
		out[33] = 1
		return 0
	}

	hash := tx.Hash()
	copy(out[:hashOutputSize], hash[:])
	out[32] = 1
	out[33] = 0
	out[34] = byte(tx.Type())

	binary.LittleEndian.PutUint64(out[txGasOffset:txGasOffset+8], tx.Gas())
	if chainID := tx.ChainId(); chainID != nil && chainID.BitLen() <= 64 {
		binary.LittleEndian.PutUint64(out[txChainIDOffset:txChainIDOffset+8], chainID.Uint64())
	}
	binary.LittleEndian.PutUint64(out[txNonceOffset:txNonceOffset+8], tx.Nonce())

	return 0
}

// helper to silence unused import in non-test builds when compiled without GPU tag
var _ = common.Hash{}
