//go:build cgo && !gpu
// +build cgo,!gpu

package gpu

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// CPU-only stub for the gpu package when built with cgo but without the 'gpu' tag.
// Provides the same public API so the project can compile without CUDA/OpenCL.

type GPUType int

const (
	GPUTypeNone GPUType = iota
	GPUTypeCUDA
	GPUTypeOpenCL
)

type GPUProcessor struct{}

type GPUConfig struct {
	PreferredGPUType GPUType `json:"preferredGpuType"`
	MaxBatchSize     int     `json:"maxBatchSize"`
	MaxMemoryUsage   uint64  `json:"maxMemoryUsage"`
	HashWorkers      int     `json:"hashWorkers"`
	SignatureWorkers int     `json:"signatureWorkers"`
	TxWorkers        int     `json:"txWorkers"`
	EnablePipelining bool    `json:"enablePipelining"`
}

func DefaultGPUConfig() *GPUConfig {
	return &GPUConfig{}
}

// Batches and results (mirror cgo implementation signatures)
type HashBatch struct {
	Hashes   [][]byte
	Results  [][]byte
	Callback func([][]byte, error)
}

type SignatureBatch struct {
	Signatures [][]byte
	Messages   [][]byte
	PublicKeys [][]byte
	Results    []bool
	Callback   func([]bool, error)
}

type TransactionBatch struct {
	Transactions []*types.Transaction
	Results      []*TxResult
	Callback     func([]*TxResult, error)
}

type TxResult struct {
	Hash      common.Hash
	Valid     bool
	GasUsed   uint64
	Error     error
	Signature []byte
}

func NewGPUProcessor(config *GPUConfig) (*GPUProcessor, error) {
	// No GPU active when 'gpu' tag is not set; return a stub processor.
	return &GPUProcessor{}, nil
}

func (p *GPUProcessor) ProcessHashesBatch(hashes [][]byte, callback func([][]byte, error)) error {
	if callback != nil {
		callback(nil, errors.New("GPU unavailable: built without 'gpu' tag"))
	}
	return nil
}

func (p *GPUProcessor) ProcessSignaturesBatch(signatures, messages, publicKeys [][]byte, callback func([]bool, error)) error {
	if callback != nil {
		callback(nil, errors.New("GPU unavailable: built without 'gpu' tag"))
	}
	return nil
}

func (p *GPUProcessor) ProcessTransactionsBatch(txs []*types.Transaction, callback func([]*TxResult, error)) error {
	if callback != nil {
		callback(nil, errors.New("GPU unavailable: built without 'gpu' tag"))
	}
	return nil
}

func (p *GPUProcessor) Close() error            { return nil }
func (p *GPUProcessor) IsGPUAvailable() bool    { return false }
func (p *GPUProcessor) GetGPUType() GPUType     { return GPUTypeNone }
func (p *GPUProcessor) GetStats() GPUStats      { return GPUStats{GPUType: GPUTypeNone} }

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

// Global instance helpers (no-ops in CPU-only)
var globalGPUProcessor *GPUProcessor

func InitGlobalGPUProcessor(config *GPUConfig) error {
	var err error
	globalGPUProcessor, err = NewGPUProcessor(config)
	return err
}

func GetGlobalGPUProcessor() *GPUProcessor {
	return globalGPUProcessor
}

func CloseGlobalGPUProcessor() error {
	return nil
}
