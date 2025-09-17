//go:build cgo && gpu
// +build cgo,gpu

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
#cgo CFLAGS: -I${SRCDIR}/native

#include <stdlib.h>
#include <string.h>

// CUDA helper entry points implemented in native/cuda_kernels.cu
int cuda_init_device();
int cuda_process_transactions(void* txs, void* lengths, int count, void* results);
int cuda_process_hashes(void* hashes, void* lengths, int count, void* results);
int cuda_verify_signatures(void* sigs, void* msgs, void* keys, int count, void* results);
void cuda_cleanup();

// OpenCL helper entry points implemented in native/opencl_kernels.c
int initOpenCL();
int processTxBatchOpenCL(void* txData, void* lengths, int txCount, void* results);
int processHashesOpenCL(void* hashes, void* lengths, int count, void* results);
int verifySignaturesOpenCL(void* signatures, void* messages, void* keys, int count, void* results);
void cleanupOpenCL();
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
	gpuType        GPUType
	deviceCount    int
	maxBatchSize   int
	maxMemoryUsage uint64

	// Processing pools
	hashPool      chan *HashBatch
	signaturePool chan *SignatureBatch
	txPool        chan *TransactionBatch

	// Statistics
	mu              sync.RWMutex
	processedHashes uint64
	processedSigs   uint64
	processedTxs    uint64
	avgHashTime     time.Duration
	avgSigTime      time.Duration
	avgTxTime       time.Duration

	// Shutdown coordination
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Memory management
	memoryPool   sync.Pool
	cudaStreams  []unsafe.Pointer
	openclQueues []unsafe.Pointer
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
// Balanced for blockchain processing + MobileLLM-R1 AI model
func DefaultGPUConfig() *GPUConfig {
	return &GPUConfig{
		PreferredGPUType: GPUTypeOpenCL,           // Prefer OpenCL for RTX 4000 SFF Ada
		MaxBatchSize:     800000,                  // 4x increase - 800K batches (reserve GPU for AI)
		MaxMemoryUsage:   14 * 1024 * 1024 * 1024, // 14GB GPU memory (6GB for MobileLLM-R1 + system)
		HashWorkers:      80,                      // 80 workers - balance with AI workload
		SignatureWorkers: 80,                      // 80 workers - balance with AI workload
		TxWorkers:        80,                      // 80 workers - balance with AI workload
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

	// Pack input into fixed 256-byte slots per hash and capture exact lengths
	in, lengths := p.prepareHashData(batch.Hashes)
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
			unsafe.Pointer(&lengths[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	case GPUTypeOpenCL:
		result = int(C.processHashesOpenCL(
			unsafe.Pointer(&in[0]),
			unsafe.Pointer(&lengths[0]),
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

	// Output buffer: 1 byte (0/1) per signature
	count := len(batch.Signatures)
	out := make([]byte, count)

	sigs := make([]byte, count*65)
	msgs := make([]byte, count*32)
	keys := make([]byte, count*65)
	for i := 0; i < count; i++ {
		sigSlot := sigs[i*65 : (i+1)*65]
		msgSlot := msgs[i*32 : (i+1)*32]
		keySlot := keys[i*65 : (i+1)*65]

		copy(sigSlot, batch.Signatures[i])
		copy(msgSlot, batch.Messages[i])
		copy(keySlot, batch.PublicKeys[i])
	}

	// Process on GPU
	var result int
	switch p.gpuType {
	case GPUTypeCUDA:
		result = int(C.cuda_verify_signatures(
			unsafe.Pointer(&sigs[0]),
			unsafe.Pointer(&msgs[0]),
			unsafe.Pointer(&keys[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	case GPUTypeOpenCL:
		result = int(C.verifySignaturesOpenCL(
			unsafe.Pointer(&sigs[0]),
			unsafe.Pointer(&msgs[0]),
			unsafe.Pointer(&keys[0]),
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
		if len(batch.Signatures[i]) < 64 || len(batch.Messages[i]) != 32 {
			batch.Results[i] = false
			continue
		}
		key := batch.PublicKeys[i]
		switch len(key) {
		case 65:
			key = append([]byte(nil), key...)
		case 64:
			prefixed := make([]byte, 65)
			prefixed[0] = 0x04
			copy(prefixed[1:], key)
			key = prefixed
		default:
			batch.Results[i] = false
			continue
		}
		sig := batch.Signatures[i][:64] // R||S only
		hash := batch.Messages[i]
		batch.Results[i] = crypto.VerifySignature(key, hash, sig)
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

	// Pack input into fixed 1024-byte slots per tx and capture lengths
	in, lengths := p.prepareTransactionData(batch.Transactions)
	defer p.memoryPool.Put(in)

	count := len(batch.Transactions)
	const txResultStride = 64
	out := make([]byte, count*txResultStride)

	// Process on GPU
	var result int
	switch p.gpuType {
	case GPUTypeCUDA:
		result = int(C.cuda_process_transactions(
			unsafe.Pointer(&in[0]),
			unsafe.Pointer(&lengths[0]),
			C.int(count),
			unsafe.Pointer(&out[0]),
		))
	case GPUTypeOpenCL:
		result = int(C.processTxBatchOpenCL(
			unsafe.Pointer(&in[0]),
			unsafe.Pointer(&lengths[0]),
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
		offset := i * txResultStride
		entry := out[offset : offset+txResultStride]

		hash := common.Hash{}
		copy(hash[:], entry[:32])
		if (hash == common.Hash{}) {
			hash = batch.Transactions[i].Hash()
		}

		valid := entry[32] != 0
		errCode := entry[33]
		gas := binary.LittleEndian.Uint64(entry[40:48])

		if batch.Results[i] == nil {
			batch.Results[i] = &TxResult{}
		}
		batch.Results[i].Hash = hash
		batch.Results[i].Valid = valid && errCode == 0
		if gas > 0 {
			batch.Results[i].GasUsed = gas
		} else {
			batch.Results[i].GasUsed = batch.Transactions[i].Gas()
		}
		if errCode != 0 {
			batch.Results[i].Error = errors.New("gpu transaction decode failed")
		} else {
			batch.Results[i].Error = nil
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
func (p *GPUProcessor) prepareHashData(hashes [][]byte) ([]byte, []uint32) {
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
	for i := range data {
		data[i] = 0
	}

	lengths := make([]uint32, count)
	for i, h := range hashes {
		base := i * slot
		n := len(h)
		if n > slot {
			n = slot
		}
		copy(data[base:base+n], h[:n])
		lengths[i] = uint32(n)
	}
	return data, lengths
}

func (p *GPUProcessor) prepareTransactionData(txs []*types.Transaction) ([]byte, []uint32) {
	// Kernels expect fixed 1024 bytes per transaction
	const slot = 1024
	count := len(txs)
	total := count * slot

	buf := p.memoryPool.Get().([]byte)
	if cap(buf) < total {
		buf = make([]byte, total)
	}
	data := buf[:total]
	for i := range data {
		data[i] = 0
	}

	lengths := make([]uint32, count)
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
		lengths[i] = uint32(n)
	}
	return data, lengths
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
