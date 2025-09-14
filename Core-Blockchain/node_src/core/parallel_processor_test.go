package core

import (
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/gopool"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

// TestParallelProcessorInitialization tests the initialization of parallel processor
func TestParallelProcessorInitialization(t *testing.T) {
	config := DefaultParallelProcessorConfig()
	
	// Test default configuration
	if config.MaxTxConcurrency != runtime.NumCPU()*4 {
		t.Errorf("Expected MaxTxConcurrency to be %d, got %d", runtime.NumCPU()*4, config.MaxTxConcurrency)
	}
	
	if config.TxBatchSize != 100 {
		t.Errorf("Expected TxBatchSize to be 100, got %d", config.TxBatchSize)
	}
	
	if !config.EnablePipelining {
		t.Error("Expected EnablePipelining to be true")
	}
	
	if !config.EnableTxBatching {
		t.Error("Expected EnableTxBatching to be true")
	}
	
	if !config.EnableBloomParallel {
		t.Error("Expected EnableBloomParallel to be true")
	}
}

// TestParallelStateProcessorCreation tests creating a parallel state processor
func TestParallelStateProcessorCreation(t *testing.T) {
	// Create test blockchain
	db := rawdb.NewMemoryDatabase()
	gspec := &Genesis{
		Config: params.TestChainConfig,
		Alloc:  GenesisAlloc{},
	}
	gspec.MustCommit(db)
	blockchain, _ := NewBlockChain(db, nil, params.TestChainConfig, ethash.NewFaker(), vm.Config{}, nil, nil)
	defer blockchain.Stop()
	
	// Create parallel state processor
	config := DefaultParallelProcessorConfig()
	config.MaxTxConcurrency = 4 // Reduce for testing
	config.TxBatchSize = 10
	
	psp, err := NewParallelStateProcessor(params.TestChainConfig, blockchain, ethash.NewFaker(), config)
	if err != nil {
		t.Fatalf("Failed to create parallel state processor: %v", err)
	}
	defer psp.Close()
	
	if psp.config.MaxTxConcurrency != 4 {
		t.Errorf("Expected MaxTxConcurrency to be 4, got %d", psp.config.MaxTxConcurrency)
	}
	
	if psp.processor == nil {
		t.Error("Expected processor to be initialized")
	}
}

// TestParallelTransactionProcessing tests parallel transaction processing
func TestParallelTransactionProcessing(t *testing.T) {
	// Create test blockchain
	db := rawdb.NewMemoryDatabase()
	
	// Create sender account with sufficient balance
	senderKey, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	
	gspec := &Genesis{
		Config: params.TestChainConfig,
		Alloc: GenesisAlloc{
			senderAddr: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
			common.HexToAddress("0x2000"): {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
		},
	}
	gspec.MustCommit(db)
	blockchain, _ := NewBlockChain(db, nil, params.TestChainConfig, ethash.NewFaker(), vm.Config{}, nil, nil)
	defer blockchain.Stop()
	
	// Create parallel state processor
	config := DefaultParallelProcessorConfig()
	config.MaxTxConcurrency = 4
	config.TxBatchSize = 5
	config.EnableTxBatching = true
	
	psp, err := NewParallelStateProcessor(params.TestChainConfig, blockchain, ethash.NewFaker(), config)
	if err != nil {
		t.Fatalf("Failed to create parallel state processor: %v", err)
	}
	defer psp.Close()
	
	// Create test transactions
	txs := createTestTransactions(t, 20)
	
	// Create test block with base fee for EIP-1559
	header := &types.Header{
		Number:     big.NewInt(1),
		ParentHash: blockchain.Genesis().Hash(),
		Time:       uint64(time.Now().Unix()),
		GasLimit:   10000000,
		Difficulty: big.NewInt(1),
		BaseFee:    big.NewInt(1000000000), // 1 gwei base fee
	}
	
	block := types.NewBlock(header, txs, nil, nil, trie.NewStackTrie(nil))
	
	// Get state
	statedb, err := blockchain.StateAt(blockchain.Genesis().Root())
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	
	// Process block with parallel processor
	start := time.Now()
	receipts, logs, gasUsed, err := psp.ProcessParallel(block, statedb, vm.Config{})
	parallelDuration := time.Since(start)
	
	if err != nil {
		t.Fatalf("Parallel processing failed: %v", err)
	}
	
	if len(receipts) != len(txs) {
		t.Errorf("Expected %d receipts, got %d", len(txs), len(receipts))
	}
	
	if gasUsed == 0 {
		t.Error("Expected gas to be used")
	}
	
	t.Logf("Parallel processing took %v for %d transactions", parallelDuration, len(txs))
	t.Logf("Gas used: %d", gasUsed)
	t.Logf("Logs generated: %d", len(logs))
	
	// Verify receipts are properly formed
	for i, receipt := range receipts {
		if receipt == nil {
			t.Errorf("Receipt %d is nil", i)
			continue
		}
		
		if receipt.TxHash != txs[i].Hash() {
			t.Errorf("Receipt %d has wrong tx hash", i)
		}
		
		if receipt.BlockNumber.Cmp(header.Number) != 0 {
			t.Errorf("Receipt %d has wrong block number", i)
		}
	}
}

// TestGopoolIntegration tests integration with the gopool package
func TestGopoolIntegration(t *testing.T) {
	// Initialize global processor
	config := gopool.DefaultProcessorConfig()
	config.TxWorkers = 4
	config.ValidationWorkers = 2
	
	err := gopool.InitGlobalProcessor(config)
	if err != nil {
		t.Fatalf("Failed to initialize global processor: %v", err)
	}
	defer gopool.CloseGlobalProcessor()
	
	processor := gopool.GetGlobalProcessor()
	if processor == nil {
		t.Fatal("Global processor is nil")
	}
	
	// Test task submission
	done := make(chan bool, 1)
	
	err = processor.SubmitTxTask(func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, func(err error) {
		if err != nil {
			t.Errorf("Task failed: %v", err)
		}
		done <- true
	})
	
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}
	
	// Wait for task completion
	select {
	case <-done:
		// Task completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Task did not complete within timeout")
	}
	
	// Check stats
	stats := processor.GetStats()
	if stats.ProcessedTasks == 0 {
		t.Error("Expected at least 1 processed task")
	}
}

// Helper function to create test transactions
func createTestTransactions(tb testing.TB, count int) []*types.Transaction {
	// Use a fixed private key for the sender account
	key, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	signer := types.NewLondonSigner(big.NewInt(1))
	
	txs := make([]*types.Transaction, count)
	
	for i := 0; i < count; i++ {
		// Create EIP-1559 transaction with proper gas fee caps
		txData := &types.DynamicFeeTx{
			ChainID:   big.NewInt(1),
			Nonce:     uint64(i),
			GasTipCap: big.NewInt(1000000000), // 1 gwei tip
			GasFeeCap: big.NewInt(2000000000), // 2 gwei max fee
			Gas:       21000,
			To:        &common.Address{0x20, 0x00}, // 0x2000...
			Value:     big.NewInt(1000),
			Data:      nil,
		}
		
		tx := types.NewTx(txData)
		signedTx, err := types.SignTx(tx, signer, key)
		if err != nil {
			tb.Fatalf("Failed to sign transaction %d: %v", i, err)
		}
		
		txs[i] = signedTx
	}
	
	return txs
}
