package gopool

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/ethereum/go-ethereum/log"
)

// ParallelProcessor provides advanced parallel processing capabilities for blockchain operations
type ParallelProcessor struct {
	// Core pools for different types of operations
	txPool          *ants.Pool // Transaction processing pool
	validationPool  *ants.Pool // Block validation pool
	consensusPool   *ants.Pool // Consensus operations pool
	statePool       *ants.Pool // State processing pool
	networkPool     *ants.Pool // Network operations pool
	
	// Configuration
	maxWorkers      int
	queueSize       int
	timeout         time.Duration
	
	// Metrics and monitoring
	mu              sync.RWMutex
	processedTasks  uint64
	failedTasks     uint64
	avgProcessTime  time.Duration
	
	// Shutdown coordination
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// ProcessorConfig holds configuration for the parallel processor
type ProcessorConfig struct {
	MaxWorkers      int           `json:"maxWorkers"`
	QueueSize       int           `json:"queueSize"`
	Timeout         time.Duration `json:"timeout"`
	TxWorkers       int           `json:"txWorkers"`
	ValidationWorkers int         `json:"validationWorkers"`
	ConsensusWorkers  int         `json:"consensusWorkers"`
	StateWorkers      int         `json:"stateWorkers"`
	NetworkWorkers    int         `json:"networkWorkers"`
}

// DefaultProcessorConfig returns a default configuration optimized for 16+ core systems
func DefaultProcessorConfig() *ProcessorConfig {
	numCPU := runtime.NumCPU()
	return &ProcessorConfig{
		MaxWorkers:        numCPU * 8,  // 8x CPU cores for maximum concurrency on 16+ core systems
		QueueSize:         50000,       // Massive queue for high-throughput transaction bursts
		Timeout:           30 * time.Second,
		TxWorkers:         numCPU * 4,  // 4x CPU cores for transaction processing (64 workers on 16-core)
		ValidationWorkers: numCPU * 2,  // 2x CPU cores for validation (32 workers on 16-core)
		ConsensusWorkers:  numCPU,      // 1x CPU cores for consensus operations (16 workers on 16-core)
		StateWorkers:      numCPU * 2,  // 2x CPU cores for state operations (32 workers on 16-core)
		NetworkWorkers:    numCPU * 2,  // 2x CPU cores for network operations (32 workers on 16-core)
	}
}

// NewParallelProcessor creates a new parallel processor with the given configuration
func NewParallelProcessor(config *ProcessorConfig) (*ParallelProcessor, error) {
	if config == nil {
		config = DefaultProcessorConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create specialized pools for different operation types
	txPool, err := ants.NewPool(config.TxWorkers, 
		ants.WithExpiryDuration(5*time.Second),
		ants.WithNonblocking(false),
		ants.WithPreAlloc(true),
	)
	if err != nil {
		cancel()
		return nil, err
	}
	
	validationPool, err := ants.NewPool(config.ValidationWorkers,
		ants.WithExpiryDuration(10*time.Second),
		ants.WithNonblocking(false),
		ants.WithPreAlloc(true),
	)
	if err != nil {
		cancel()
		txPool.Release()
		return nil, err
	}
	
	consensusPool, err := ants.NewPool(config.ConsensusWorkers,
		ants.WithExpiryDuration(15*time.Second),
		ants.WithNonblocking(false),
		ants.WithPreAlloc(true),
	)
	if err != nil {
		cancel()
		txPool.Release()
		validationPool.Release()
		return nil, err
	}
	
	statePool, err := ants.NewPool(config.StateWorkers,
		ants.WithExpiryDuration(10*time.Second),
		ants.WithNonblocking(false),
		ants.WithPreAlloc(true),
	)
	if err != nil {
		cancel()
		txPool.Release()
		validationPool.Release()
		consensusPool.Release()
		return nil, err
	}
	
	networkPool, err := ants.NewPool(config.NetworkWorkers,
		ants.WithExpiryDuration(5*time.Second),
		ants.WithNonblocking(false),
		ants.WithPreAlloc(true),
	)
	if err != nil {
		cancel()
		txPool.Release()
		validationPool.Release()
		consensusPool.Release()
		statePool.Release()
		return nil, err
	}
	
	processor := &ParallelProcessor{
		txPool:         txPool,
		validationPool: validationPool,
		consensusPool:  consensusPool,
		statePool:      statePool,
		networkPool:    networkPool,
		maxWorkers:     config.MaxWorkers,
		queueSize:      config.QueueSize,
		timeout:        config.Timeout,
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// Start monitoring goroutine
	processor.wg.Add(1)
	go processor.monitor()
	
	log.Info("Parallel processor initialized", 
		"txWorkers", config.TxWorkers,
		"validationWorkers", config.ValidationWorkers,
		"consensusWorkers", config.ConsensusWorkers,
		"stateWorkers", config.StateWorkers,
		"networkWorkers", config.NetworkWorkers,
	)
	
	return processor, nil
}

// TaskType represents different types of blockchain operations
type TaskType int

const (
	TaskTypeTx TaskType = iota
	TaskTypeValidation
	TaskTypeConsensus
	TaskTypeState
	TaskTypeNetwork
)

// Task represents a unit of work to be processed in parallel
type Task struct {
	Type        TaskType
	Fn          func() error
	Timeout     time.Duration
	OnComplete  func(error)
	Priority    int // Higher values = higher priority
}

// SubmitTask submits a task to the appropriate worker pool based on its type
func (p *ParallelProcessor) SubmitTask(task *Task) error {
	if task == nil || task.Fn == nil {
		return nil
	}
	
	start := time.Now()
	
	wrappedTask := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Task panicked", "type", task.Type, "panic", r)
				p.incrementFailedTasks()
				if task.OnComplete != nil {
					task.OnComplete(ErrTaskPanicked)
				}
			}
		}()
		
		// Set timeout context if specified
		var ctx context.Context
		var cancel context.CancelFunc
		
		if task.Timeout > 0 {
			ctx, cancel = context.WithTimeout(p.ctx, task.Timeout)
			defer cancel()
		} else {
			ctx = p.ctx
		}
		
		// Execute task with timeout
		done := make(chan error, 1)
		go func() {
			done <- task.Fn()
		}()
		
		var err error
		select {
		case err = <-done:
		case <-ctx.Done():
			err = ctx.Err()
		}
		
		// Update metrics
		duration := time.Since(start)
		if err != nil {
			p.incrementFailedTasks()
		} else {
			p.incrementProcessedTasks()
		}
		p.updateAvgProcessTime(duration)
		
		// Call completion callback
		if task.OnComplete != nil {
			task.OnComplete(err)
		}
	}
	
	// Submit to appropriate pool
	var pool *ants.Pool
	switch task.Type {
	case TaskTypeTx:
		pool = p.txPool
	case TaskTypeValidation:
		pool = p.validationPool
	case TaskTypeConsensus:
		pool = p.consensusPool
	case TaskTypeState:
		pool = p.statePool
	case TaskTypeNetwork:
		pool = p.networkPool
	default:
		pool = p.txPool // Default to transaction pool
	}
	
	return pool.Submit(wrappedTask)
}

// SubmitTxTask submits a transaction processing task
func (p *ParallelProcessor) SubmitTxTask(fn func() error, onComplete func(error)) error {
	return p.SubmitTask(&Task{
		Type:       TaskTypeTx,
		Fn:         fn,
		OnComplete: onComplete,
		Timeout:    p.timeout,
	})
}

// SubmitValidationTask submits a validation task
func (p *ParallelProcessor) SubmitValidationTask(fn func() error, onComplete func(error)) error {
	return p.SubmitTask(&Task{
		Type:       TaskTypeValidation,
		Fn:         fn,
		OnComplete: onComplete,
		Timeout:    p.timeout,
	})
}

// SubmitConsensusTask submits a consensus task
func (p *ParallelProcessor) SubmitConsensusTask(fn func() error, onComplete func(error)) error {
	return p.SubmitTask(&Task{
		Type:       TaskTypeConsensus,
		Fn:         fn,
		OnComplete: onComplete,
		Timeout:    p.timeout * 2, // Longer timeout for consensus operations
	})
}

// SubmitStateTask submits a state processing task
func (p *ParallelProcessor) SubmitStateTask(fn func() error, onComplete func(error)) error {
	return p.SubmitTask(&Task{
		Type:       TaskTypeState,
		Fn:         fn,
		OnComplete: onComplete,
		Timeout:    p.timeout,
	})
}

// SubmitNetworkTask submits a network operation task
func (p *ParallelProcessor) SubmitNetworkTask(fn func() error, onComplete func(error)) error {
	return p.SubmitTask(&Task{
		Type:       TaskTypeNetwork,
		Fn:         fn,
		OnComplete: onComplete,
		Timeout:    p.timeout / 2, // Shorter timeout for network operations
	})
}

// BatchSubmit submits multiple tasks in parallel
func (p *ParallelProcessor) BatchSubmit(tasks []*Task) []error {
	errors := make([]error, len(tasks))
	var wg sync.WaitGroup
	
	for i, task := range tasks {
		if task == nil {
			continue
		}
		
		wg.Add(1)
		go func(idx int, t *Task) {
			defer wg.Done()
			errors[idx] = p.SubmitTask(t)
		}(i, task)
	}
	
	wg.Wait()
	return errors
}

// WaitForCompletion waits for all submitted tasks to complete with timeout
func (p *ParallelProcessor) WaitForCompletion(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if p.IsIdle() {
				return nil
			}
		}
	}
}

// IsIdle returns true if all worker pools are idle
func (p *ParallelProcessor) IsIdle() bool {
	return p.txPool.Running() == 0 &&
		   p.validationPool.Running() == 0 &&
		   p.consensusPool.Running() == 0 &&
		   p.statePool.Running() == 0 &&
		   p.networkPool.Running() == 0
}

// GetStats returns current processor statistics
func (p *ParallelProcessor) GetStats() ProcessorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return ProcessorStats{
		ProcessedTasks:    p.processedTasks,
		FailedTasks:       p.failedTasks,
		AvgProcessTime:    p.avgProcessTime,
		TxPoolRunning:     p.txPool.Running(),
		ValidationRunning: p.validationPool.Running(),
		ConsensusRunning:  p.consensusPool.Running(),
		StateRunning:      p.statePool.Running(),
		NetworkRunning:    p.networkPool.Running(),
	}
}

// ProcessorStats holds statistics about the processor
type ProcessorStats struct {
	ProcessedTasks    uint64        `json:"processedTasks"`
	FailedTasks       uint64        `json:"failedTasks"`
	AvgProcessTime    time.Duration `json:"avgProcessTime"`
	TxPoolRunning     int           `json:"txPoolRunning"`
	ValidationRunning int           `json:"validationRunning"`
	ConsensusRunning  int           `json:"consensusRunning"`
	StateRunning      int           `json:"stateRunning"`
	NetworkRunning    int           `json:"networkRunning"`
}

// Close gracefully shuts down the parallel processor
func (p *ParallelProcessor) Close() error {
	log.Info("Shutting down parallel processor...")
	
	// Cancel context to stop all operations
	p.cancel()
	
	// Wait for monitoring goroutine to finish
	p.wg.Wait()
	
	// Release all pools
	p.txPool.Release()
	p.validationPool.Release()
	p.consensusPool.Release()
	p.statePool.Release()
	p.networkPool.Release()
	
	log.Info("Parallel processor shutdown complete")
	return nil
}

// Private helper methods

func (p *ParallelProcessor) incrementProcessedTasks() {
	p.mu.Lock()
	p.processedTasks++
	p.mu.Unlock()
}

func (p *ParallelProcessor) incrementFailedTasks() {
	p.mu.Lock()
	p.failedTasks++
	p.mu.Unlock()
}

func (p *ParallelProcessor) updateAvgProcessTime(duration time.Duration) {
	p.mu.Lock()
	if p.avgProcessTime == 0 {
		p.avgProcessTime = duration
	} else {
		// Simple moving average
		p.avgProcessTime = (p.avgProcessTime + duration) / 2
	}
	p.mu.Unlock()
}

func (p *ParallelProcessor) monitor() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			stats := p.GetStats()
			log.Debug("Parallel processor stats",
				"processed", stats.ProcessedTasks,
				"failed", stats.FailedTasks,
				"avgTime", stats.AvgProcessTime,
				"txRunning", stats.TxPoolRunning,
				"validationRunning", stats.ValidationRunning,
				"consensusRunning", stats.ConsensusRunning,
				"stateRunning", stats.StateRunning,
				"networkRunning", stats.NetworkRunning,
			)
		}
	}
}

// Global processor instance
var globalProcessor *ParallelProcessor

// InitGlobalProcessor initializes the global parallel processor
func InitGlobalProcessor(config *ProcessorConfig) error {
	if globalProcessor != nil {
		globalProcessor.Close()
	}
	
	var err error
	globalProcessor, err = NewParallelProcessor(config)
	return err
}

// GetGlobalProcessor returns the global parallel processor
func GetGlobalProcessor() *ParallelProcessor {
	return globalProcessor
}

// CloseGlobalProcessor closes the global parallel processor
func CloseGlobalProcessor() error {
	if globalProcessor != nil {
		return globalProcessor.Close()
	}
	return nil
}

// Error definitions
var (
	ErrTaskPanicked = errors.New("task panicked during execution")
)
