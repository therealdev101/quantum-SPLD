//go:build cgo && gpu

package gpu

import (
	"bytes"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func newTestGPUProcessor() *GPUProcessor {
	p := &GPUProcessor{
		gpuType: GPUTypeCUDA,
	}
	p.memoryPool = sync.Pool{New: func() interface{} { return make([]byte, 1024*64) }}
	return p
}

func TestGPUHashParity(t *testing.T) {
	p := newTestGPUProcessor()

	inputs := [][]byte{
		[]byte(""),
		[]byte("ethereum"),
		bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 40),
	}

	batch := &HashBatch{
		Hashes:  inputs,
		Results: make([][]byte, len(inputs)),
	}

	p.processHashesGPU(batch)

	for i, input := range inputs {
		expected := crypto.Keccak256(input)
		if !bytes.Equal(batch.Results[i], expected) {
			t.Fatalf("hash %d mismatch: have %x want %x", i, batch.Results[i], expected)
		}
	}
}

func TestGPUSignatureParity(t *testing.T) {
	p := newTestGPUProcessor()

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	msg := crypto.Keccak256([]byte("gpu-signature-test"))
	sig, err := crypto.Sign(msg, key)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	pub := crypto.FromECDSAPub(&key.PublicKey)
	invalidSig := append([]byte{}, sig...)
	invalidSig[10] ^= 0xFF

	if !crypto.VerifySignature(pub, msg, sig[:64]) {
		t.Fatalf("pre-check signature failed")
	}

	batch := &SignatureBatch{
		Signatures: [][]byte{sig, invalidSig},
		Messages:   [][]byte{msg, msg},
		PublicKeys: [][]byte{pub, pub},
		Results:    make([]bool, 2),
	}

	p.processSignaturesCPU(batch)
	if !batch.Results[0] {
		t.Fatalf("cpu valid signature reported invalid")
	}
	if batch.Results[1] {
		t.Fatalf("cpu invalid signature reported valid")
	}

	p.processSignaturesGPU(batch)

	if !batch.Results[0] {
		t.Fatalf("valid signature reported invalid")
	}
	if batch.Results[1] {
		t.Fatalf("invalid signature reported valid")
	}
}

func TestGPUTransactionParity(t *testing.T) {
	p := newTestGPUProcessor()

	chainID := big.NewInt(1337)
	signer := types.LatestSignerForChainID(chainID)

	key1, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("key1: %v", err)
	}
	key2, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("key2: %v", err)
	}

	txs := make([]*types.Transaction, 0, 2)

	tx1 := types.NewTransaction(1, crypto.PubkeyToAddress(key2.PublicKey), big.NewInt(1_000_000_000_000000000), 21000, big.NewInt(42_000_000_000), nil)
	signed1, err := types.SignTx(tx1, signer, key1)
	if err != nil {
		t.Fatalf("sign tx1: %v", err)
	}
	txs = append(txs, signed1)

	payload := bytes.Repeat([]byte{0xAA}, 48)
	tx2 := types.NewTransaction(2, crypto.PubkeyToAddress(key1.PublicKey), big.NewInt(12345), 90000, big.NewInt(100_000_000), payload)
	signed2, err := types.SignTx(tx2, signer, key2)
	if err != nil {
		t.Fatalf("sign tx2: %v", err)
	}
	txs = append(txs, signed2)

	batch := &TransactionBatch{
		Transactions: txs,
		Results:      make([]*TxResult, len(txs)),
	}

	p.processTransactionsGPU(batch)

	for i, tx := range txs {
		res := batch.Results[i]
		if res == nil {
			t.Fatalf("tx %d result missing", i)
		}
		if !res.Valid {
			t.Fatalf("tx %d reported invalid", i)
		}
		if res.GasUsed != tx.Gas() {
			t.Fatalf("tx %d gas mismatch: have %d want %d", i, res.GasUsed, tx.Gas())
		}
		if res.Hash != tx.Hash() {
			t.Fatalf("tx %d hash mismatch", i)
		}
	}
}
