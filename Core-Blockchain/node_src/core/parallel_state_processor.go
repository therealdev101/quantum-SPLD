package core

import (
	"context"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/gopool"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// ParallelStateProcessor extends StateProcessor with advanced parallel processing capabilities
type ParallelStateProcessor struct {
	*StateProcessor
	processor       *gopool.ParallelProcessor
	config          *ParallelProcessorConfig
	
	// Performance metrics
	mu              sync.RWMutex
	processedBlocks uint64
	avgBlockTime    time.Duration
	maxConcurrency  int32
	currentLoad     int32
}

// ParallelProcessorConfig holds configuration for parallel state processing
type ParallelProcessorConfig struct {
	// Transaction processing
	MaxTxConcurrency     int           `json:"maxTxConcurrency"`
	TxBatchSize          int           `json:"txBatchSize"`
	TxTimeout            time.Duration `json:"txTimeout"`
	
	// Validation settings
	MaxValidationWorkers int           `json:"maxValidationWorkers"`
	ValidationTimeout    time.Duration `json:"validationTimeout"`
	
	// State processing
	StateWorkers         int           `json:"stateWorkers"`
	StateTimeout         time.Duration `json:"stateTimeout"`
	
	// Performance tuning
	EnablePipelining     bool          `json:"enablePipelining"`
	EnableTxBatching     bool          `json:"enableTxBatching"`
	EnableBloomParallel  bool          `json:"enableBloomParallel"`
	AdaptiveScaling      bool          `json:"adaptiveScaling"`
	
	// Resource limits
	MaxMemoryUsage       uint64        `json:"maxMemoryUsage"`
	MaxGoroutines        int           `json:"maxGoroutines"`
}

// DefaultParallelProcessorConfig returns optimized default configuration
func DefaultParallelProcessorConfig() *ParallelProcessorConfig {
	numCPU := runtime.NumCPU()
	return &ParallelProcessorConfig{
		MaxTxConcurrency:     numCPU * 4,
		TxBatchSize:          100,
		TxTimeout:            30 * time.Second,
		MaxValidationWorkers: numCPU * 2,
		ValidationTimeout:    15 * time.Second,
		StateWorkers:         numCPU,
		StateTimeout:         20 * time.Second,
		EnablePipelining:     true,
		EnableTxBatching:     true,
		EnableBloomParallel:  true,
		AdaptiveScaling:      true,
		MaxMemoryUsage:       1024 * 1024 * 1024, // 1GB
		MaxGoroutines:        numCPU * 8,
	}
}

// NewParallelStateProcessor creates a new parallel state processor
func NewParallelStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine, parallelConfig *ParallelProcessorConfig) (*ParallelStateProcessor, error) {
	if parallelConfig == nil {
		parallelConfig = DefaultParallelProcessorConfig()
	}
	
	// Create base state processor
	baseProcessor := NewStateProcessor(config, bc, engine)
	
	// Initialize parallel processor
	processorConfig := &gopool.ProcessorConfig{
		MaxWorkers:        parallelConfig.MaxGoroutines,
		QueueSize:         10000,
		Timeout:           parallelConfig.TxTimeout,
		TxWorkers:         parallelConfig.MaxTxConcurrency,
		ValidationWorkers: parallelConfig.MaxValidationWorkers,
		StateWorkers:      parallelConfig.StateWorkers,
		ConsensusWorkers:  runtime.NumCPU() / 2,
		NetworkWorkers:    runtime.NumCPU(),
	}
	
	processor, err := gopool.NewParallelProcessor(processorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create parallel processor: %w", err)
	}
	
	psp := &ParallelStateProcessor{
		StateProcessor: baseProcessor,
		processor:      processor,
		config:         parallelConfig,
		maxConcurrency: int32(parallelConfig.MaxTxConcurrency),
	}
	
	log.Info("Parallel state processor initialized",
		"maxTxConcurrency", parallelConfig.MaxTxConcurrency,
		"txBatchSize", parallelConfig.TxBatchSize,
		"validationWorkers", parallelConfig.MaxValidationWorkers,
		"stateWorkers", parallelConfig.StateWorkers,
		"pipelining", parallelConfig.EnablePipelining,
		"batching", parallelConfig.EnableTxBatching,
	)
	
	return psp, nil
}

// ProcessParallel processes a block using advanced parallel processing techniques
func (psp *ParallelStateProcessor) ProcessParallel(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		psp.updateMetrics(duration)
		log.Debug("Parallel block processing completed",
			"number", block.Number(),
			"txs", len(block.Transactions()),
			"duration", duration,
			"tps", float64(len(block.Transactions()))/duration.Seconds(),
		)
	}()
	
	var (
		receipts    = make([]*types.Receipt, 0, len(block.Transactions()))
		usedGas     = new(uint64)
		header      = block.Header()
		blockHash   = block.Hash()
		blockNumber = block.Number()
		allLogs     []*types.Log
		gp          = new(GasPool).AddGas(block.GasLimit())
	)
	
	// Create EVM context
	blockContext := NewEVMBlockContext(header, psp.bc, nil)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, psp.StateProcessor.config, cfg)
	
	// Handle PoSA consensus if applicable
	posa, isPoSA := psp.engine.(consensus.PoSA)
	if isPoSA {
		if err := posa.PreHandle(psp.bc, header, statedb); err != nil {
			return nil, nil, 0, err
		}
		vmenv.Context.ExtraValidator = posa.CreateEvmExtraValidator(header, statedb)
	}
	
	// Preload accounts for better performance
	signer := types.MakeSigner(psp.StateProcessor.config, header.Number)
	statedb.PreloadAccounts(block, signer)
	
	// Separate system and regular transactions
	commonTxs, systemTxs, err := psp.separateTransactions(block.Transactions(), signer, header, isPoSA, posa)
	if err != nil {
		return nil, nil, 0, err
	}
	
	// Process transactions based on configuration
	if psp.config.EnableTxBatching && len(commonTxs) > psp.config.TxBatchSize {
		receipts, allLogs, err = psp.processBatchedTransactions(commonTxs, statedb, vmenv, gp, usedGas, blockNumber, blockHash)
	} else if psp.config.EnablePipelining {
		receipts, allLogs, err = psp.processPipelinedTransactions(commonTxs, statedb, vmenv, gp, usedGas, blockNumber, blockHash)
	} else {
		receipts, allLogs, err = psp.processSequentialTransactions(commonTxs, statedb, vmenv, gp, usedGas, blockNumber, blockHash)
	}
	
	if err != nil {
		return nil, nil, 0, err
	}
	
	// Finalize the block
	if err := psp.engine.Finalize(psp.bc, header, statedb, &commonTxs, block.Uncles(), &receipts, systemTxs); err != nil {
		return nil, nil, 0, err
	}
	
	return receipts, allLogs, *usedGas, nil
}

// separateTransactions separates system and regular transactions
func (psp *ParallelStateProcessor) separateTransactions(txs []*types.Transaction, signer types.Signer, header *types.Header, isPoSA bool, posa consensus.PoSA) ([]*types.Transaction, []*types.Transaction, error) {
	commonTxs := make([]*types.Transaction, 0, len(txs))
	systemTxs := make([]*types.Transaction, 0)
	
	for _, tx := range txs {
		if isPoSA {
			sender, err := types.Sender(signer, tx)
			if err != nil {
				return nil, nil, err
			}
			
			ok, err := posa.IsSysTransaction(sender, tx, header)
			if err != nil {
				return nil, nil, err
			}
			
			if ok {
				systemTxs = append(systemTxs, tx)
				continue
			}
		}
		commonTxs = append(commonTxs, tx)
	}
	
	return commonTxs, systemTxs, nil
}

// processBatchedTransactions processes transactions in parallel batches
func (psp *ParallelStateProcessor) processBatchedTransactions(txs []*types.Transaction, statedb *state.StateDB, vmenv *vm.EVM, gp *GasPool, usedGas *uint64, blockNumber *big.Int, blockHash common.Hash) ([]*types.Receipt, []*types.Log, error) {
	receipts := make([]*types.Receipt, len(txs))
	allLogs := make([]*types.Log, 0)
	
	batchSize := psp.config.TxBatchSize
	numBatches := (len(txs) + batchSize - 1) / batchSize
	
	// Process batches sequentially to maintain state consistency
	for batchIdx := 0; batchIdx < numBatches; batchIdx++ {
		start := batchIdx * batchSize
		end := start + batchSize
		if end > len(txs) {
			end = len(txs)
		}
		
		batchTxs := txs[start:end]
		batchReceipts, batchLogs, err := psp.processBatch(batchTxs, start, statedb, vmenv, gp, usedGas, blockNumber, blockHash)
		if err != nil {
			return nil, nil, err
		}
		
		// Copy results
		copy(receipts[start:end], batchReceipts)
		allLogs = append(allLogs, batchLogs...)
	}
	
	return receipts, allLogs, nil
}

// processBatch processes a batch of transactions with parallel bloom filter creation
func (psp *ParallelStateProcessor) processBatch(txs []*types.Transaction, startIdx int, statedb *state.StateDB, vmenv *vm.EVM, gp *GasPool, usedGas *uint64, blockNumber *big.Int, blockHash common.Hash) ([]*types.Receipt, []*types.Log, error) {
	receipts := make([]*types.Receipt, len(txs))
	allLogs := make([]*types.Log, 0)
	
	var bloomWg sync.WaitGroup
	
	// Process transactions sequentially within batch (for state consistency)
	for i, tx := range txs {
		msg, err := tx.AsMessage(types.MakeSigner(psp.StateProcessor.config, blockNumber), nil)
		if err != nil {
			return nil, nil, fmt.Errorf("could not apply tx %d [%v]: %w", startIdx+i, tx.Hash().Hex(), err)
		}
		
		statedb.Prepare(tx.Hash(), startIdx+i)
		
		// Apply transaction
		receipt, err := psp.applyTransactionParallel(msg, psp.StateProcessor.config, psp.bc, nil, gp, statedb, blockNumber, blockHash, tx, usedGas, vmenv, &bloomWg)
		if err != nil {
			return nil, nil, fmt.Errorf("could not apply tx %d [%v]: %w", startIdx+i, tx.Hash().Hex(), err)
		}
		
		receipts[i] = receipt
		allLogs = append(allLogs, receipt.Logs...)
	}
	
	// Wait for all bloom filters to be created
	bloomWg.Wait()
	
	return receipts, allLogs, nil
}

// processPipelinedTransactions processes transactions using pipelining
func (psp *ParallelStateProcessor) processPipelinedTransactions(txs []*types.Transaction, statedb *state.StateDB, vmenv *vm.EVM, gp *GasPool, usedGas *uint64, blockNumber *big.Int, blockHash common.Hash) ([]*types.Receipt, []*types.Log, error) {
	receipts := make([]*types.Receipt, len(txs))
	allLogs := make([]*types.Log, 0)
	
	// Pipeline stages: validation -> execution -> bloom creation
	validationCh := make(chan *txValidationResult, psp.config.MaxTxConcurrency)
	executionCh := make(chan *txExecutionResult, psp.config.MaxTxConcurrency)
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Stage 1: Transaction validation
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(validationCh)
		psp.validateTransactionsPipeline(ctx, txs, blockNumber, validationCh)
	}()
	
	// Stage 2: Transaction execution
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(executionCh)
		psp.executeTransactionsPipeline(ctx, validationCh, statedb, vmenv, gp, usedGas, blockNumber, blockHash, executionCh)
	}()
	
	// Stage 3: Result collection and bloom creation
	wg.Add(1)
	go func() {
		defer wg.Done()
		psp.collectResultsPipeline(ctx, executionCh, receipts, &allLogs)
	}()
	
	wg.Wait()
	
	return receipts, allLogs, nil
}

// processSequentialTransactions processes transactions sequentially (fallback)
func (psp *ParallelStateProcessor) processSequentialTransactions(txs []*types.Transaction, statedb *state.StateDB, vmenv *vm.EVM, gp *GasPool, usedGas *uint64, blockNumber *big.Int, blockHash common.Hash) ([]*types.Receipt, []*types.Log, error) {
	receipts := make([]*types.Receipt, len(txs))
	allLogs := make([]*types.Log, 0)
	
	var bloomWg sync.WaitGroup
	
	for i, tx := range txs {
		msg, err := tx.AsMessage(types.MakeSigner(psp.StateProcessor.config, blockNumber), nil)
		if err != nil {
			return nil, nil, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		
		statedb.Prepare(tx.Hash(), i)
		
		receipt, err := psp.applyTransactionParallel(msg, psp.StateProcessor.config, psp.bc, nil, gp, statedb, blockNumber, blockHash, tx, usedGas, vmenv, &bloomWg)
		if err != nil {
			return nil, nil, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		
		receipts[i] = receipt
		allLogs = append(allLogs, receipt.Logs...)
	}
	
	bloomWg.Wait()
	return receipts, allLogs, nil
}

// applyTransactionParallel applies a transaction with parallel bloom filter creation
func (psp *ParallelStateProcessor) applyTransactionParallel(msg types.Message, config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM, bloomWg *sync.WaitGroup) (*types.Receipt, error) {
	// Create a new context to be used in the EVM environment
	txContext := NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)
	
	// Apply the transaction to the current state
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}
	
	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(blockNumber) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
	}
	*usedGas += result.UsedGas
	
	// Create receipt
	receipt := &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	
	// Set contract address if contract creation
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}
	
	// Set logs and create bloom filter
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockHash)
	receipt.BlockHash = blockHash
	receipt.BlockNumber = blockNumber
	receipt.TransactionIndex = uint(statedb.TxIndex())
	
	// Create bloom filter in parallel if enabled
	if psp.config.EnableBloomParallel && bloomWg != nil {
		bloomWg.Add(1)
		psp.processor.SubmitTask(&gopool.Task{
			Type: gopool.TaskTypeTx,
			Fn: func() error {
				receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
				return nil
			},
			OnComplete: func(err error) {
				bloomWg.Done()
				if err != nil {
					log.Error("Failed to create bloom filter", "err", err)
				}
			},
		})
	} else {
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	}
	
	return receipt, nil
}

// Pipeline helper types and functions
type txValidationResult struct {
	index int
	tx    *types.Transaction
	msg   types.Message
	err   error
}

type txExecutionResult struct {
	index   int
	receipt *types.Receipt
	logs    []*types.Log
	err     error
}

func (psp *ParallelStateProcessor) validateTransactionsPipeline(ctx context.Context, txs []*types.Transaction, blockNumber *big.Int, output chan<- *txValidationResult) {
	signer := types.MakeSigner(psp.StateProcessor.config, blockNumber)
	
	for i, tx := range txs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		msg, err := tx.AsMessage(signer, nil)
		result := &txValidationResult{
			index: i,
			tx:    tx,
			msg:   msg,
			err:   err,
		}
		
		select {
		case output <- result:
		case <-ctx.Done():
			return
		}
	}
}

func (psp *ParallelStateProcessor) executeTransactionsPipeline(ctx context.Context, input <-chan *txValidationResult, statedb *state.StateDB, vmenv *vm.EVM, gp *GasPool, usedGas *uint64, blockNumber *big.Int, blockHash common.Hash, output chan<- *txExecutionResult) {
	for {
		select {
		case <-ctx.Done():
			return
		case validation, ok := <-input:
			if !ok {
				return
			}
			
			if validation.err != nil {
				result := &txExecutionResult{
					index: validation.index,
					err:   validation.err,
				}
				select {
				case output <- result:
				case <-ctx.Done():
					return
				}
				continue
			}
			
			statedb.Prepare(validation.tx.Hash(), validation.index)
			
			receipt, err := psp.applyTransactionParallel(validation.msg, psp.StateProcessor.config, psp.bc, nil, gp, statedb, blockNumber, blockHash, validation.tx, usedGas, vmenv, nil)
			
			result := &txExecutionResult{
				index:   validation.index,
				receipt: receipt,
				err:     err,
			}
			
			if receipt != nil {
				result.logs = receipt.Logs
			}
			
			select {
			case output <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (psp *ParallelStateProcessor) collectResultsPipeline(ctx context.Context, input <-chan *txExecutionResult, receipts []*types.Receipt, allLogs *[]*types.Log) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-input:
			if !ok {
				return
			}
			
			if result.err != nil {
				log.Error("Transaction execution failed", "index", result.index, "err", result.err)
				continue
			}
			
			receipts[result.index] = result.receipt
			*allLogs = append(*allLogs, result.logs...)
		}
	}
}

// Performance monitoring and adaptive scaling
func (psp *ParallelStateProcessor) updateMetrics(duration time.Duration) {
	psp.mu.Lock()
	defer psp.mu.Unlock()
	
	psp.processedBlocks++
	if psp.avgBlockTime == 0 {
		psp.avgBlockTime = duration
	} else {
		psp.avgBlockTime = (psp.avgBlockTime + duration) / 2
	}
	
	// Adaptive scaling based on performance
	if psp.config.AdaptiveScaling {
		psp.adjustConcurrency(duration)
	}
}

func (psp *ParallelStateProcessor) adjustConcurrency(duration time.Duration) {
	currentConcurrency := atomic.LoadInt32(&psp.maxConcurrency)
	
	// If processing is too slow, reduce concurrency to avoid overhead
	if duration > 5*time.Second && currentConcurrency > 1 {
		newConcurrency := currentConcurrency * 8 / 10 // Reduce by 20%
		atomic.StoreInt32(&psp.maxConcurrency, newConcurrency)
		log.Debug("Reduced concurrency due to slow processing", "old", currentConcurrency, "new", newConcurrency)
	}
	
	// If processing is fast and we have capacity, increase concurrency
	if duration < 1*time.Second && currentConcurrency < int32(psp.config.MaxTxConcurrency) {
		newConcurrency := currentConcurrency * 11 / 10 // Increase by 10%
		if newConcurrency > int32(psp.config.MaxTxConcurrency) {
			newConcurrency = int32(psp.config.MaxTxConcurrency)
		}
		atomic.StoreInt32(&psp.maxConcurrency, newConcurrency)
		log.Debug("Increased concurrency due to fast processing", "old", currentConcurrency, "new", newConcurrency)
	}
}

// GetStats returns performance statistics
func (psp *ParallelStateProcessor) GetStats() ParallelProcessorStats {
	psp.mu.RLock()
	defer psp.mu.RUnlock()
	
	processorStats := psp.processor.GetStats()
	
	return ParallelProcessorStats{
		ProcessedBlocks:   psp.processedBlocks,
		AvgBlockTime:      psp.avgBlockTime,
		CurrentConcurrency: atomic.LoadInt32(&psp.maxConcurrency),
		ProcessorStats:    processorStats,
	}
}

type ParallelProcessorStats struct {
	ProcessedBlocks     uint64                  `json:"processedBlocks"`
	AvgBlockTime        time.Duration           `json:"avgBlockTime"`
	CurrentConcurrency  int32                   `json:"currentConcurrency"`
	ProcessorStats      gopool.ProcessorStats   `json:"processorStats"`
}

// Close shuts down the parallel processor
func (psp *ParallelStateProcessor) Close() error {
	return psp.processor.Close()
}
