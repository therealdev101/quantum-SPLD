package gpu

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

/*
#cgo LDFLAGS: -lOpenCL -L/root/splendor-blockchain-v4/Core-Blockchain/node_src/common/gpu -lsplendor_cuda -lcudart -L/usr/local/cuda/lib64

#include <stdlib.h>
#include <string.h>

// CUDA function declarations (implemented in cuda_kernels.cu)
int cuda_init_device();
int cuda_process_transactions(void* txs, int count, void* results);
int cuda_process_hashes(void* hashes, int count, void* results);
int cuda_verify_signatures(void* sigs, void* msgs, void* keys, int count, void* results);
void cuda_cleanup();

// CUDA function declarations (stubs for now - can be replaced with real implementations)
int initCUDA();
int processTxBatchCUDA(void* txData, int txCount, void* results);
int processHashesCUDA(void* hashes, int count, void* results);
int verifySignaturesCUDA(void* signatures, int count, void* results);
void cleanupCUDA();

// OpenCL function declarations (implemented in opencl_kernels.c)
int initOpenCL();
int processTxBatchOpenCL(void* txData, int txCount, void* results);
int processHashesOpenCL(void* hashes, int count, void* results);
int verifySignaturesOpenCL(void* signatures, int count, void* results);
void cleanupOpenCL();

// Working stub implementations for CUDA (can be replaced when CUDA is properly configured)
int initCUDA() { 
    // Return -1 to indicate CUDA not available, system will fall back to OpenCL or CPU
    return -1; 
}

int processHashesCUDA(void* hashes, int count, void* results) { 
    return -1; // Not implemented, will fall back to OpenCL or CPU
}

int verifySignaturesCUDA(void* signatures, int count, void* results) {
    return -1; // Not implemented, will fall back to OpenCL or CPU
}

int processTxBatchCUDA(void* txData, int txCount, void* results) { 
    return -1; // Not implemented, will fall back to OpenCL or CPU
}

void cleanupCUDA() { 
    // No-op for stub implementation
}
*/
import "C"

// GPUType represents the type of GPU acceleration
type GPUType int

const (
	GPUTypeNone GPUType = iota
	GPUTypeCUDA
	GPUTypeOpenCL
)

// GPUProcessor provides GPU-accelerated blockchain operations
type GPUProcessor struct {
	gpuType         GPUType
	deviceCount     int
	maxBatchSize    int
	maxMemoryUsage  uint64
	
	// Processing pools
	hashPool        chan *HashBatch
	signaturePool   chan *SignatureBatch
	txPool          chan *TransactionBatch
	
	// Statistics
	mu              sync.RWMutex
	processedHashes uint64
	processedSigs   uint64
	processedTxs    uint64
	avgHashTime     time.Duration
	avgSigTime      time.Duration
	avgTxTime       time.Duration
	
	// Shutdown coordination
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	
	// Memory management
	memoryPool      sync.Pool
	cudaStreams     []unsafe.Pointer
	openclQueues    []unsafe.Pointer
}

// GPUConfig holds configuration for GPU processing
type GPUConfig struct {
	PreferredGPUType GPUType `json:"preferredGpuType"`
	MaxBatchSize     int     `json:"maxBatchSize"`
	MaxMemoryUsage   uint64  `json:"maxMemoryUsage"`
	HashWorkers      int     `json:"hashWorkers"`
	SignatureWorkers int     `json:"signatureWorkers"`
	TxWorkers        int     `json:"txWorkers"`
	EnablePipelining bool    `json:"enablePipelining"`
}

// DefaultGPUConfig returns optimized GPU configuration for NVIDIA RTX 4000 SFF Ada (20GB VRAM)
func DefaultGPUConfig() *GPUConfig {
	return &GPUConfig{
		PreferredGPUType: GPUTypeOpenCL, // Prefer OpenCL for RTX 4000 SFF Ada
		MaxBatchSize:     200000,        // 4x increase - 200K batches for massive throughput
		MaxMemoryUsage:   18 * 1024 * 1024 * 1024, // 18GB GPU memory (leave 2GB for system)
		HashWorkers:      50,            // 2.5x increase - 50 workers for RTX 4000 SFF Ada
		SignatureWorkers: 50,            // 2.5x increase - 50 workers for signature verification
		TxWorkers:        50,            // 2.5x increase - 50 workers for transaction processing
		EnablePipelining: true,
	}
}

// HashBatch represents a batch of hashes to process
type HashBatch struct {
	Hashes   [][]byte
	Results  [][]byte
	Callback func([][]byte, error)
}

// SignatureBatch represents a batch of signatures to verify
type SignatureBatch struct {
	Signatures [][]byte
	Messages   [][]byte
	PublicKeys [][]byte
	Results    []bool
	Callback   func([]bool, error)
}

// TransactionBatch represents a batch of transactions to process
type TransactionBatch struct {
	Transactions []*types.Transaction
	Results      []*TxResult
	Callback     func([]*TxResult, error)
}

// TxResult holds the result of GPU transaction processing
type TxResult struct {
	Hash      common.Hash
	Valid     bool
	GasUsed   uint64
	Error     error
	Signature []byte
}

// NewGPUProcessor creates a new GPU processor
func NewGPUProcessor(config *GPUConfig) (*GPUProcessor, error) {
	if config == nil {
		config = DefaultGPUConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	processor := &GPUProcessor{
		maxBatchSize:   config.MaxBatchSize,
		maxMemoryUsage: config.MaxMemoryUsage,
		ctx:            ctx,
		cancel:         cancel,
		hashPool:       make(chan *HashBatch, 100),
		signaturePool:  make(chan *SignatureBatch, 100),
		txPool:         make(chan *TransactionBatch, 100),
	}
	
	// Initialize memory pool
	processor.memoryPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, config.MaxBatchSize*256) // 256 bytes per item
		},
	}
	
	// Try to initialize GPU
	if err := processor.initializeGPU(config.PreferredGPUType); err != nil {
		log.Warn("GPU initialization failed, falling back to CPU", "error", err)
		processor.gpuType = GPUTypeNone
	}
	
	// Start worker goroutines
	processor.startWorkers(config)
	
	log.Info("GPU processor initialized", 
		"type", processor.gpuType,
		"deviceCount", processor.deviceCount,
		"maxBatchSize", processor.maxBatchSize,
	)
	
	return processor, nil
}

// initializeGPU attempts to initialize GPU acceleration
func (p *GPUProcessor) initializeGPU(preferredType GPUType) error {
	// Try CUDA first if preferred or if no preference
	if preferredType == GPUTypeCUDA || preferredType == GPUTypeNone {
		if result := C.cuda_init_device(); result > 0 {
			p.gpuType = GPUTypeCUDA
			p.deviceCount = int(result)
			log.Info("CUDA GPU acceleration enabled", "devices", p.deviceCount)
			return nil
		}
	}
	
	// Try OpenCL if CUDA failed or if preferred
	if preferredType == GPUTypeOpenCL || preferredType == GPUTypeNone {
		if result := C.initOpenCL(); result > 0 {
			p.gpuType = GPUTypeOpenCL
			p.deviceCount = int(result)
			log.Info("OpenCL GPU acceleration enabled", "devices", p.deviceCount, "type", "RTX 4000 SFF Ada")
			return nil
		}
	}
	
	return errors.New("no GPU acceleration available")
}

// startWorkers starts the GPU worker goroutines
func (p *GPUProcessor) startWorkers(config *GPUConfig) {
	// Hash processing workers
	for i := 0; i < config.HashWorkers; i++ {
		p.wg.Add(1)
		go p.hashWorker()
	}
	
	// Signature verification workers
	for i := 0; i < config.SignatureWorkers; i++ {
		p.wg.Add(1)
		go p.signatureWorker()
	}
	
	// Transaction processing workers
	for i := 0; i < config.TxWorkers; i++ {
		p.wg.Add(1)
		go p.transactionWorker()
	}
}

// ProcessHashesBatch processes a batch of hashes using GPU acceleration
func (p *GPUProcessor) ProcessHashesBatch(hashes [][]byte, callback func([][]byte, error)) error {
	if len(hashes) == 0 {
		callback(nil, nil)
		return nil
	}
	
	batch := &HashBatch{
		Hashes:   hashes,
		Results:  make([][]byte, len(hashes)),
		Callback: callback,
	}
	
	select {
	case p.hashPool <- batch:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
		return errors.New("hash processing queue full")
	}
}

// ProcessSignaturesBatch verifies a batch of signatures using GPU acceleration
func (p *GPUProcessor) ProcessSignaturesBatch(signatures, messages, publicKeys [][]byte, callback func([]bool, error)) error {
	if len(signatures) == 0 {
		callback(nil, nil)
		return nil
	}
	
	batch := &SignatureBatch{
		Signatures: signatures,
		Messages:   messages,
		PublicKeys: publicKeys,
		Results:    make([]bool, len(signatures)),
		Callback:   callback,
	}
	
	select {
	case p.signaturePool <- batch:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
		return errors.New("signature processing queue full")
	}
}

// ProcessTransactionsBatch processes a batch of transactions using GPU acceleration
func (p *GPUProcessor) ProcessTransactionsBatch(txs []*types.Transaction, callback func([]*TxResult, error)) error {
	if len(txs) == 0 {
		callback(nil, nil)
		return nil
	}
	
	batch := &TransactionBatch{
		Transactions: txs,
		Results:      make([]*TxResult, len(txs)),
		Callback:     callback,
	}
	
	select {
	case p.txPool <- batch:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
		return errors.New("transaction processing queue full")
	}
}

// hashWorker processes hash batches using GPU acceleration
func (p *GPUProcessor) hashWorker() {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case batch := <-p.hashPool:
			start := time.Now()
			
			if p.gpuType == GPUTypeNone {
				// CPU fallback
				p.processHashesCPU(batch)
			} else {
				// GPU processing
				p.processHashesGPU(batch)
			}
			
			duration := time.Since(start)
			p.updateHashStats(duration)
		}
	}
}

// signatureWorker processes signature verification batches
func (p *GPUProcessor) signatureWorker() {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case batch := <-p.signaturePool:
			start := time.Now()
			
			if p.gpuType == GPUTypeNone {
				// CPU fallback
				p.processSignaturesCPU(batch)
			} else {
				// GPU processing
				p.processSignaturesGPU(batch)
			}
			
			duration := time.Since(start)
			p.updateSigStats(duration)
		}
	}
}

// transactionWorker processes transaction batches
func (p *GPUProcessor) transactionWorker() {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case batch := <-p.txPool:
			start := time.Now()
			
			if p.gpuType == GPUTypeNone {
				// CPU fallback
				p.processTransactionsCPU(batch)
			} else {
				// GPU processing
				p.processTransactionsGPU(batch)
			}
			
			duration := time.Since(start)
			p.updateTxStats(duration)
		}
	}
}

// processHashesGPU processes hashes using GPU acceleration
func (p *GPUProcessor) processHashesGPU(batch *HashBatch) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("GPU hash processing panicked", "panic", r)
			p.processHashesCPU(batch) // Fallback to CPU
		}
	}()

	// Pack input into fixed 256-byte slots per hash
	in := p.prepareHashData(batch.Hashes)
	defer p.memoryPool.Put(in)

	// Allocate output buffer: 32 bytes per hash
	count := len(batch.Hashes)
	out := make([]byte, count*32)

	// Process on GPU
	var result int
	switch p.gpuType {
	case GPUTypeCUDA:
		result = int(C.cuda_process_hashes(
			unsafe.Pointer(&in[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	case GPUTypeOpenCL:
		result = int(C.processHashesOpenCL(
			unsafe.Pointer(&in[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	}

	if result != 0 {
		log.Warn("GPU hash processing failed, falling back to CPU", "error", result)
		p.processHashesCPU(batch)
		return
	}

	// Split flat output into [][]byte
	for i := 0; i < count; i++ {
		start := i * 32
		dst := make([]byte, 32)
		copy(dst, out[start:start+32])
		batch.Results[i] = dst
	}

	if batch.Callback != nil {
		batch.Callback(batch.Results, nil)
	}
}

// processHashesCPU processes hashes using CPU as fallback
func (p *GPUProcessor) processHashesCPU(batch *HashBatch) {
	for i, hash := range batch.Hashes {
		result := crypto.Keccak256(hash)
		batch.Results[i] = result
	}
	
	if batch.Callback != nil {
		batch.Callback(batch.Results, nil)
	}
}

// processSignaturesGPU processes signature verification using GPU
func (p *GPUProcessor) processSignaturesGPU(batch *SignatureBatch) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("GPU signature processing panicked", "panic", r)
			p.processSignaturesCPU(batch) // Fallback to CPU
		}
	}()

	// Pack input as [65|32|64] per item (stride 161 bytes)
	packed := p.prepareSignatureData(batch.Signatures, batch.Messages, batch.PublicKeys)
	defer p.memoryPool.Put(packed)

	// Output buffer: 1 byte (0/1) per signature
	count := len(batch.Signatures)
	out := make([]byte, count)

	// Process on GPU
	var result int
	switch p.gpuType {
	case GPUTypeCUDA: {
		// CUDA expects separate buffers for signatures, messages, and pubkeys
		sigs := make([]byte, count*65)
		msgs := make([]byte, count*32)
		keys := make([]byte, count*64)
		for i := 0; i < count; i++ {
			copy(sigs[i*65:(i+1)*65], batch.Signatures[i])
			copy(msgs[i*32:(i+1)*32], batch.Messages[i])
			copy(keys[i*64:(i+1)*64], batch.PublicKeys[i])
		}
		result = int(C.cuda_verify_signatures(
			unsafe.Pointer(&sigs[0]),
			unsafe.Pointer(&msgs[0]),
			unsafe.Pointer(&keys[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	}
	case GPUTypeOpenCL:
		result = int(C.verifySignaturesOpenCL(
			unsafe.Pointer(&packed[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	}

	if result != 0 {
		log.Warn("GPU signature processing failed, falling back to CPU", "error", result)
		p.processSignaturesCPU(batch)
		return
	}

	// Map bytes to bools
	for i := 0; i < count; i++ {
		batch.Results[i] = out[i] != 0
	}

	if batch.Callback != nil {
		batch.Callback(batch.Results, nil)
	}
}

// processSignaturesCPU processes signature verification using CPU as fallback
func (p *GPUProcessor) processSignaturesCPU(batch *SignatureBatch) {
	for i := range batch.Signatures {
		if len(batch.Signatures[i]) != 65 || len(batch.Messages[i]) != 32 || len(batch.PublicKeys[i]) != 64 {
			batch.Results[i] = false
			continue
		}
		sig := batch.Signatures[i][:64] // R||S only
		hash := batch.Messages[i]
		pubkey := batch.PublicKeys[i] // uncompressed 64-byte (X||Y)
		batch.Results[i] = crypto.VerifySignature(pubkey, hash, sig)
	}
	if batch.Callback != nil {
		batch.Callback(batch.Results, nil)
	}
}

// processTransactionsGPU processes transactions using GPU
func (p *GPUProcessor) processTransactionsGPU(batch *TransactionBatch) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("GPU transaction processing panicked", "panic", r)
			p.processTransactionsCPU(batch) // Fallback to CPU
		}
	}()

	// Pack input into fixed 1024-byte slots per tx
	in := p.prepareTransactionData(batch.Transactions)
	defer p.memoryPool.Put(in)

	count := len(batch.Transactions)
	// Output buffer: 64 bytes per tx: [0]=valid, [1]=checksum, [2..9]=gas (8 bytes LE), rest reserved
	out := make([]byte, count*64)

	// Process on GPU
	var result int
	switch p.gpuType {
	case GPUTypeCUDA:
		result = int(C.cuda_process_transactions(
			unsafe.Pointer(&in[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	case GPUTypeOpenCL:
		result = int(C.processTxBatchOpenCL(
			unsafe.Pointer(&in[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	}

	if result != 0 {
		log.Warn("GPU transaction processing failed, falling back to CPU", "error", result)
		p.processTransactionsCPU(batch)
		return
	}

	// Convert results
	for i := 0; i < count; i++ {
		offset := i * 64

		// Interpret result layout based on backend:
		// - CUDA:   [0]=valid, [1]=checksum, [2..9]=gas (LE)
		// - OpenCL: [0..31]=hash, [32]=valid, (no gas written)
		var valid bool
		var gas uint64
		switch p.gpuType {
		case GPUTypeOpenCL:
			if len(out) >= offset+33 {
				valid = out[offset+32] != 0
			}
			// No gas provided by OpenCL kernel, will fall back to tx.Gas()
		default:
			if len(out) >= offset+1 {
				valid = out[offset] != 0
			}
			if len(out) >= offset+10 {
				gas = binary.LittleEndian.Uint64(out[offset+2 : offset+10])
			}
		}

		if batch.Results[i] == nil {
			batch.Results[i] = &TxResult{}
		}
		batch.Results[i].Hash = batch.Transactions[i].Hash()
		batch.Results[i].Valid = valid
		if gas > 0 {
			batch.Results[i].GasUsed = gas
		} else {
			batch.Results[i].GasUsed = batch.Transactions[i].Gas()
		}
	}

	if batch.Callback != nil {
		batch.Callback(batch.Results, nil)
	}
}

// processTransactionsCPU processes transactions using CPU as fallback
func (p *GPUProcessor) processTransactionsCPU(batch *TransactionBatch) {
	for i, tx := range batch.Transactions {
		batch.Results[i] = &TxResult{
			Hash:    tx.Hash(),
			Valid:   true, // Simplified validation
			GasUsed: tx.Gas(),
			Error:   nil,
		}
	}
	
	if batch.Callback != nil {
		batch.Callback(batch.Results, nil)
	}
}

// Helper functions for data preparation with safety checks
func (p *GPUProcessor) prepareHashData(hashes [][]byte) []byte {
	// OpenCL/CUDA kernels expect fixed 256 bytes per input item
	const slot = 256
	count := len(hashes)
	total := count * slot

	buf := p.memoryPool.Get().([]byte)
	if cap(buf) < total {
		// Allocate a new buffer if the pool buffer is too small
		buf = make([]byte, total)
	}
	data := buf[:total]

	for i, h := range hashes {
		base := i * slot
		n := len(h)
		if n > slot {
			n = slot
		}
		copy(data[base:base+n], h[:n])
		// Remaining bytes are already zeroed
	}
	return data
}

func (p *GPUProcessor) prepareSignatureData(signatures, messages, publicKeys [][]byte) []byte {
	data := p.memoryPool.Get().([]byte)
	offset := 0
	
	for i := range signatures {
		totalSize := len(signatures[i]) + len(messages[i]) + len(publicKeys[i])
		// Safety check to prevent buffer overflow
		if offset+totalSize > len(data) {
			log.Warn("Signature data buffer overflow, truncating batch", "offset", offset, "totalSize", totalSize, "bufferLen", len(data))
			break
		}
		
		copy(data[offset:], signatures[i])
		offset += len(signatures[i])
		copy(data[offset:], messages[i])
		offset += len(messages[i])
		copy(data[offset:], publicKeys[i])
		offset += len(publicKeys[i])
	}
	
	return data[:offset]
}

func (p *GPUProcessor) prepareTransactionData(txs []*types.Transaction) []byte {
	// Kernels expect fixed 1024 bytes per transaction
	const slot = 1024
	count := len(txs)
	total := count * slot

	buf := p.memoryPool.Get().([]byte)
	if cap(buf) < total {
		buf = make([]byte, total)
	}
	data := buf[:total]

	for i, tx := range txs {
		txBytes, err := tx.MarshalBinary()
		if err != nil {
			log.Warn("Failed to marshal transaction", "hash", tx.Hash(), "error", err)
			continue
		}
		base := i * slot
		n := len(txBytes)
		if n > slot {
			// Truncate if too large for slot
			n = slot
		}
		copy(data[base:base+n], txBytes[:n])
		// Remaining bytes are left zeroed
	}
	return data
}

func (p *GPUProcessor) convertHashResults(batch *HashBatch) {
	// Results are already in the correct format from GPU
	// This function can be extended for format conversion if needed
}

func (p *GPUProcessor) convertTransactionResults(batch *TransactionBatch) {
	// Convert GPU results to TxResult format
	// This is a simplified implementation
	for i := range batch.Results {
		if batch.Results[i] == nil {
			batch.Results[i] = &TxResult{
				Hash:  batch.Transactions[i].Hash(),
				Valid: true,
			}
		}
	}
}

// Statistics update functions
func (p *GPUProcessor) updateHashStats(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.processedHashes++
	if p.avgHashTime == 0 {
		p.avgHashTime = duration
	} else {
		p.avgHashTime = (p.avgHashTime + duration) / 2
	}
}

func (p *GPUProcessor) updateSigStats(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.processedSigs++
	if p.avgSigTime == 0 {
		p.avgSigTime = duration
	} else {
		p.avgSigTime = (p.avgSigTime + duration) / 2
	}
}

func (p *GPUProcessor) updateTxStats(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.processedTxs++
	if p.avgTxTime == 0 {
		p.avgTxTime = duration
	} else {
		p.avgTxTime = (p.avgTxTime + duration) / 2
	}
}

// GetStats returns current GPU processor statistics
func (p *GPUProcessor) GetStats() GPUStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return GPUStats{
		GPUType:         p.gpuType,
		DeviceCount:     p.deviceCount,
		ProcessedHashes: p.processedHashes,
		ProcessedSigs:   p.processedSigs,
		ProcessedTxs:    p.processedTxs,
		AvgHashTime:     p.avgHashTime,
		AvgSigTime:      p.avgSigTime,
		AvgTxTime:       p.avgTxTime,
		HashQueueSize:   len(p.hashPool),
		SigQueueSize:    len(p.signaturePool),
		TxQueueSize:     len(p.txPool),
	}
}

// GPUStats holds GPU processor statistics
type GPUStats struct {
	GPUType         GPUType       `json:"gpuType"`
	DeviceCount     int           `json:"deviceCount"`
	ProcessedHashes uint64        `json:"processedHashes"`
	ProcessedSigs   uint64        `json:"processedSigs"`
	ProcessedTxs    uint64        `json:"processedTxs"`
	AvgHashTime     time.Duration `json:"avgHashTime"`
	AvgSigTime      time.Duration `json:"avgSigTime"`
	AvgTxTime       time.Duration `json:"avgTxTime"`
	HashQueueSize   int           `json:"hashQueueSize"`
	SigQueueSize    int           `json:"sigQueueSize"`
	TxQueueSize     int           `json:"txQueueSize"`
}

// IsGPUAvailable returns true if GPU acceleration is available
func (p *GPUProcessor) IsGPUAvailable() bool {
	return p.gpuType != GPUTypeNone
}

// GetGPUType returns the current GPU type
func (p *GPUProcessor) GetGPUType() GPUType {
	return p.gpuType
}

// Close gracefully shuts down the GPU processor
func (p *GPUProcessor) Close() error {
	log.Info("Shutting down GPU processor...")
	
	// Cancel context to stop all workers
	p.cancel()
	
	// Wait for all workers to finish
	p.wg.Wait()
	
	// Cleanup GPU resources
	switch p.gpuType {
	case GPUTypeCUDA:
		C.cuda_cleanup()
	case GPUTypeOpenCL:
		C.cleanupOpenCL()
	}
	
	log.Info("GPU processor shutdown complete")
	return nil
}

// Global GPU processor instance
var globalGPUProcessor *GPUProcessor

// InitGlobalGPUProcessor initializes the global GPU processor
func InitGlobalGPUProcessor(config *GPUConfig) error {
	if globalGPUProcessor != nil {
		globalGPUProcessor.Close()
	}
	
	var err error
	globalGPUProcessor, err = NewGPUProcessor(config)
	return err
}

// GetGlobalGPUProcessor returns the global GPU processor
func GetGlobalGPUProcessor() *GPUProcessor {
	return globalGPUProcessor
}

// CloseGlobalGPUProcessor closes the global GPU processor
func CloseGlobalGPUProcessor() error {
	if globalGPUProcessor != nil {
		return globalGPUProcessor.Close()
	}
	return nil
}
