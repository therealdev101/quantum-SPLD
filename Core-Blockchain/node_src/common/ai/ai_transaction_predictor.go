package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// AITransactionPredictor uses AI to predict transaction patterns and optimize processing
type AITransactionPredictor struct {
	// AI Configuration
	aiLoadBalancer *AILoadBalancer
	
	// Transaction Pattern Analysis
	mu                sync.RWMutex
	transactionHistory []TransactionPattern
	predictions       []TPSPrediction
	
	// Performance Optimization
	optimalBatchSizes map[string]int // Strategy -> optimal batch size
	gasPatterns      []GasPattern
	
	// Real-time Metrics
	currentTPS        uint64
	peakTPS           uint64
	avgLatency        time.Duration
	
	// Control
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
}

// TransactionPattern holds transaction pattern analysis
type TransactionPattern struct {
	Timestamp       time.Time `json:"timestamp"`
	TxCount         int       `json:"txCount"`
	AvgGasPrice     uint64    `json:"avgGasPrice"`
	AvgGasLimit     uint64    `json:"avgGasLimit"`
	ComplexityScore float64   `json:"complexityScore"`
	TxTypes         map[int]int `json:"txTypes"` // Transaction type distribution
	TimeOfDay       int       `json:"timeOfDay"` // Hour of day (0-23)
	DayOfWeek       int       `json:"dayOfWeek"` // Day of week (0-6)
}

// TPSPrediction holds AI predictions for TPS optimization
type TPSPrediction struct {
	Timestamp         time.Time `json:"timestamp"`
	PredictedTPS      uint64    `json:"predictedTps"`
	PredictedLatency  float64   `json:"predictedLatency"`
	OptimalBatchSize  int       `json:"optimalBatchSize"`
	RecommendedStrategy string  `json:"recommendedStrategy"`
	Confidence        float64   `json:"confidence"`
	Reasoning         string    `json:"reasoning"`
	TimeHorizon       time.Duration `json:"timeHorizon"` // How far ahead this prediction is for
}

// GasPattern holds gas usage pattern analysis
type GasPattern struct {
	Timestamp       time.Time `json:"timestamp"`
	AvgGasUsed      uint64    `json:"avgGasUsed"`
	GasEfficiency   float64   `json:"gasEfficiency"`
	ComplexTxRatio  float64   `json:"complexTxRatio"`
	OptimalGasLimit uint64    `json:"optimalGasLimit"`
}

// PredictorStats holds AI transaction predictor statistics
type PredictorStats struct {
	TotalPatterns     int                `json:"totalPatterns"`
	TotalPredictions  int                `json:"totalPredictions"`
	CurrentTPS        uint64             `json:"currentTps"`
	PeakTPS           uint64             `json:"peakTps"`
	AvgLatency        time.Duration      `json:"avgLatency"`
	OptimalBatchSizes map[string]int     `json:"optimalBatchSizes"`
	LastPrediction    *TPSPrediction     `json:"lastPrediction"`
}

// NewAITransactionPredictor creates a new AI transaction predictor
func NewAITransactionPredictor(aiLoadBalancer *AILoadBalancer) (*AITransactionPredictor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	predictor := &AITransactionPredictor{
		aiLoadBalancer:    aiLoadBalancer,
		transactionHistory: make([]TransactionPattern, 0, 1000),
		predictions:       make([]TPSPrediction, 0, 100),
		optimalBatchSizes: make(map[string]int),
		gasPatterns:      make([]GasPattern, 0, 500),
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// Initialize optimal batch sizes with defaults
	predictor.optimalBatchSizes["CPU_ONLY"] = 1000
	predictor.optimalBatchSizes["GPU_ONLY"] = 100000
	predictor.optimalBatchSizes["HYBRID"] = 50000
	
	// Start prediction loops
	predictor.wg.Add(2)
	go predictor.patternAnalysisLoop()
	go predictor.tpsPredictionLoop()
	
	log.Info("AI transaction predictor initialized")
	return predictor, nil
}

// patternAnalysisLoop continuously analyzes transaction patterns
func (p *AITransactionPredictor) patternAnalysisLoop() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second) // Analyze patterns every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.analyzeTransactionPatterns()
		}
	}
}

// tpsPredictionLoop makes TPS predictions using AI
func (p *AITransactionPredictor) tpsPredictionLoop() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(2 * time.Second) // Make predictions every 2 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.makeTpsPrediction()
		}
	}
}

// AnalyzeTransactionBatch analyzes a batch of transactions for patterns
func (p *AITransactionPredictor) AnalyzeTransactionBatch(txs []*types.Transaction) TransactionPattern {
	if len(txs) == 0 {
		return TransactionPattern{Timestamp: time.Now()}
	}
	
	now := time.Now()
	pattern := TransactionPattern{
		Timestamp:   now,
		TxCount:     len(txs),
		TxTypes:     make(map[int]int),
		TimeOfDay:   now.Hour(),
		DayOfWeek:   int(now.Weekday()),
	}
	
	var totalGasPrice, totalGasLimit uint64
	var complexityScore float64
	
	for _, tx := range txs {
		// Analyze gas patterns
		totalGasPrice += tx.GasPrice().Uint64()
		totalGasLimit += tx.Gas()
		
		// Count transaction types
		txType := int(tx.Type())
		pattern.TxTypes[txType]++
		
		// Calculate complexity score based on data size and gas usage
		dataSize := len(tx.Data())
		gasRatio := float64(tx.Gas()) / float64(21000) // Ratio to basic transfer
		complexity := math.Log(float64(dataSize+1)) * gasRatio
		complexityScore += complexity
	}
	
	// Calculate averages
	if len(txs) > 0 {
		pattern.AvgGasPrice = totalGasPrice / uint64(len(txs))
		pattern.AvgGasLimit = totalGasLimit / uint64(len(txs))
		pattern.ComplexityScore = complexityScore / float64(len(txs))
	}
	
	// Store pattern
	p.mu.Lock()
	p.transactionHistory = append(p.transactionHistory, pattern)
	if len(p.transactionHistory) > 1000 {
		p.transactionHistory = p.transactionHistory[1:]
	}
	p.mu.Unlock()
	
	return pattern
}

// analyzeTransactionPatterns performs deep analysis of transaction patterns
func (p *AITransactionPredictor) analyzeTransactionPatterns() {
	p.mu.RLock()
	if len(p.transactionHistory) < 10 {
		p.mu.RUnlock()
		return // Need more data
	}
	
	// Get recent patterns
	recentPatterns := make([]TransactionPattern, 10)
	copy(recentPatterns, p.transactionHistory[len(p.transactionHistory)-10:])
	p.mu.RUnlock()
	
	// Analyze gas efficiency trends
	gasPattern := p.analyzeGasPatterns(recentPatterns)
	
	p.mu.Lock()
	p.gasPatterns = append(p.gasPatterns, gasPattern)
	if len(p.gasPatterns) > 500 {
		p.gasPatterns = p.gasPatterns[1:]
	}
	p.mu.Unlock()
	
	// Update optimal batch sizes based on patterns
	p.updateOptimalBatchSizes(recentPatterns)
}

// analyzeGasPatterns analyzes gas usage patterns
func (p *AITransactionPredictor) analyzeGasPatterns(patterns []TransactionPattern) GasPattern {
	var totalGasUsed uint64
	var totalComplexity float64
	var complexTxCount int
	
	for _, pattern := range patterns {
		totalGasUsed += pattern.AvgGasLimit * uint64(pattern.TxCount)
		totalComplexity += pattern.ComplexityScore
		
		if pattern.ComplexityScore > 2.0 { // Threshold for complex transactions
			complexTxCount++
		}
	}
	
	avgGasUsed := totalGasUsed / uint64(len(patterns))
	gasEfficiency := 1.0 / (totalComplexity/float64(len(patterns)) + 1.0) // Higher complexity = lower efficiency
	complexTxRatio := float64(complexTxCount) / float64(len(patterns))
	
	// Calculate optimal gas limit based on patterns
	optimalGasLimit := uint64(float64(avgGasUsed) * 1.2) // 20% buffer
	
	return GasPattern{
		Timestamp:       time.Now(),
		AvgGasUsed:      avgGasUsed,
		GasEfficiency:   gasEfficiency,
		ComplexTxRatio:  complexTxRatio,
		OptimalGasLimit: optimalGasLimit,
	}
}

// updateOptimalBatchSizes updates optimal batch sizes based on transaction patterns
func (p *AITransactionPredictor) updateOptimalBatchSizes(patterns []TransactionPattern) {
	// Calculate average complexity
	var avgComplexity float64
	var totalTxs int
	
	for _, pattern := range patterns {
		avgComplexity += pattern.ComplexityScore * float64(pattern.TxCount)
		totalTxs += pattern.TxCount
	}
	
	if totalTxs > 0 {
		avgComplexity /= float64(totalTxs)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Adjust batch sizes based on complexity
	if avgComplexity < 1.0 {
		// Simple transactions - can use larger batches
		p.optimalBatchSizes["CPU_ONLY"] = 2000
		p.optimalBatchSizes["GPU_ONLY"] = 200000
		p.optimalBatchSizes["HYBRID"] = 100000
	} else if avgComplexity > 3.0 {
		// Complex transactions - use smaller batches
		p.optimalBatchSizes["CPU_ONLY"] = 500
		p.optimalBatchSizes["GPU_ONLY"] = 50000
		p.optimalBatchSizes["HYBRID"] = 25000
	} else {
		// Medium complexity - balanced batch sizes
		p.optimalBatchSizes["CPU_ONLY"] = 1000
		p.optimalBatchSizes["GPU_ONLY"] = 100000
		p.optimalBatchSizes["HYBRID"] = 50000
	}
}

// makeTpsPrediction uses AI to predict TPS and optimize processing
func (p *AITransactionPredictor) makeTpsPrediction() {
	if p.aiLoadBalancer == nil {
		return
	}
	
	// Generate AI prompt for TPS prediction
	prompt := p.generateTPSPredictionPrompt()
	
	// Query AI for prediction
	response, err := p.aiLoadBalancer.queryLLM(prompt)
	if err != nil {
		log.Debug("TPS prediction failed", "error", err)
		return
	}
	
	// Parse AI response
	prediction, err := p.parseTPSPrediction(response)
	if err != nil {
		log.Debug("TPS prediction parsing failed", "error", err)
		return
	}
	
	// Store prediction
	p.mu.Lock()
	p.predictions = append(p.predictions, prediction)
	if len(p.predictions) > 100 {
		p.predictions = p.predictions[1:]
	}
	p.mu.Unlock()
	
	log.Debug("TPS prediction made",
		"predictedTPS", prediction.PredictedTPS,
		"optimalBatchSize", prediction.OptimalBatchSize,
		"confidence", prediction.Confidence,
	)
}

// generateTPSPredictionPrompt creates a prompt for TPS prediction
func (p *AITransactionPredictor) generateTPSPredictionPrompt() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Get recent patterns and gas data
	recentPatterns := ""
	if len(p.transactionHistory) > 0 {
		recent := p.transactionHistory[len(p.transactionHistory)-min(5, len(p.transactionHistory)):]
		for i, pattern := range recent {
			recentPatterns += fmt.Sprintf("  %d. TxCount: %d, AvgGas: %d, Complexity: %.2f, Hour: %d\n",
				i+1, pattern.TxCount, pattern.AvgGasPrice, pattern.ComplexityScore, pattern.TimeOfDay)
		}
	}
	
	gasEfficiency := 1.0
	if len(p.gasPatterns) > 0 {
		gasEfficiency = p.gasPatterns[len(p.gasPatterns)-1].GasEfficiency
	}
	
	now := time.Now()
	
	prompt := fmt.Sprintf(`You are an AI transaction predictor for a MASSIVE TPS blockchain system optimized for 500K-2M+ TPS.

CURRENT SYSTEM STATUS:
- Current TPS: %d
- Peak TPS: %d  
- Average Latency: %.1fms
- Gas Efficiency: %.2f
- Time: %02d:00 (Hour %d, Day %d)

RECENT TRANSACTION PATTERNS:
%s

OPTIMIZATION CONTEXT:
- System now supports 2.5M transaction pool capacity
- GPU batch sizes up to 200K transactions
- RTX 4000 SFF Ada with 18GB allocated memory
- Block generation every 50ms minimum

PREDICTION REQUIRED:
Based on transaction patterns and time-based trends, predict:
1. Expected TPS for next 30 seconds
2. Optimal batch size for current transaction complexity
3. Best processing strategy (CPU_ONLY, GPU_ONLY, HYBRID)
4. Expected latency
5. Confidence level (0.0-1.0)

OPTIMIZATION GOALS:
- Maximize TPS while keeping latency under 25ms
- Utilize RTX 4000 SFF Ada at 95-98%% capacity
- Adapt batch sizes based on transaction complexity
- Predict and prevent bottlenecks before they occur

Respond in JSON format:
{
  "predictedTps": 750000,
  "predictedLatency": 18.5,
  "optimalBatchSize": 150000,
  "recommendedStrategy": "GPU_ONLY",
  "confidence": 0.92,
  "reasoning": "High simple transaction volume detected, GPU can handle large batches efficiently"
}`,
		p.currentTPS,
		p.peakTPS,
		float64(p.avgLatency.Milliseconds()),
		gasEfficiency,
		now.Hour(),
		now.Hour(),
		int(now.Weekday()),
		recentPatterns,
	)
	
	return prompt
}

// parseTPSPrediction parses AI response into TPS prediction
func (p *AITransactionPredictor) parseTPSPrediction(response string) (TPSPrediction, error) {
	// Extract JSON from response
	start := bytes.Index([]byte(response), []byte("{"))
	end := bytes.LastIndex([]byte(response), []byte("}"))
	
	if start == -1 || end == -1 || start >= end {
		return TPSPrediction{}, fmt.Errorf("no valid JSON found in response")
	}
	
	jsonStr := response[start : end+1]
	
	var aiResponse struct {
		PredictedTPS      uint64  `json:"predictedTps"`
		PredictedLatency  float64 `json:"predictedLatency"`
		OptimalBatchSize  int     `json:"optimalBatchSize"`
		RecommendedStrategy string `json:"recommendedStrategy"`
		Confidence        float64 `json:"confidence"`
		Reasoning         string  `json:"reasoning"`
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &aiResponse); err != nil {
		return TPSPrediction{}, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Validate and sanitize response
	if aiResponse.PredictedTPS > 10000000 {
		aiResponse.PredictedTPS = 10000000 // Cap at 10M TPS
	}
	if aiResponse.OptimalBatchSize < 100 {
		aiResponse.OptimalBatchSize = 100
	}
	if aiResponse.OptimalBatchSize > 200000 {
		aiResponse.OptimalBatchSize = 200000
	}
	if aiResponse.Confidence < 0 || aiResponse.Confidence > 1 {
		aiResponse.Confidence = 0.5
	}
	
	return TPSPrediction{
		Timestamp:         time.Now(),
		PredictedTPS:      aiResponse.PredictedTPS,
		PredictedLatency:  aiResponse.PredictedLatency,
		OptimalBatchSize:  aiResponse.OptimalBatchSize,
		RecommendedStrategy: aiResponse.RecommendedStrategy,
		Confidence:        aiResponse.Confidence,
		Reasoning:         aiResponse.Reasoning,
		TimeHorizon:       30 * time.Second,
	}, nil
}

// GetOptimalBatchSize returns the AI-recommended optimal batch size for a strategy
func (p *AITransactionPredictor) GetOptimalBatchSize(strategy string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if size, exists := p.optimalBatchSizes[strategy]; exists {
		return size
	}
	
	// Default fallbacks
	switch strategy {
	case "CPU_ONLY":
		return 1000
	case "GPU_ONLY":
		return 100000
	case "HYBRID":
		return 50000
	default:
		return 10000
	}
}

// UpdatePerformanceMetrics updates real-time performance metrics
func (p *AITransactionPredictor) UpdatePerformanceMetrics(tps uint64, latency time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.currentTPS = tps
	if tps > p.peakTPS {
		p.peakTPS = tps
	}
	
	// Update average latency with exponential moving average
	if p.avgLatency == 0 {
		p.avgLatency = latency
	} else {
		p.avgLatency = time.Duration(float64(p.avgLatency)*0.9 + float64(latency)*0.1)
	}
}

// GetLatestPrediction returns the most recent TPS prediction
func (p *AITransactionPredictor) GetLatestPrediction() *TPSPrediction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if len(p.predictions) == 0 {
		return nil
	}
	
	latest := p.predictions[len(p.predictions)-1]
	return &latest
}

// GetPredictorStats returns current predictor statistics
func (p *AITransactionPredictor) GetPredictorStats() PredictorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return PredictorStats{
		TotalPatterns:     len(p.transactionHistory),
		TotalPredictions:  len(p.predictions),
		CurrentTPS:       p.currentTPS,
		PeakTPS:          p.peakTPS,
		AvgLatency:       p.avgLatency,
		OptimalBatchSizes: p.copyBatchSizes(),
		LastPrediction:   p.getLastPredictionCopy(),
	}
}

func (p *AITransactionPredictor) copyBatchSizes() map[string]int {
	copy := make(map[string]int)
	for k, v := range p.optimalBatchSizes {
		copy[k] = v
	}
	return copy
}

func (p *AITransactionPredictor) getLastPredictionCopy() *TPSPrediction {
	if len(p.predictions) == 0 {
		return nil
	}
	latest := p.predictions[len(p.predictions)-1]
	return &latest
}

// Close gracefully shuts down the AI transaction predictor
func (p *AITransactionPredictor) Close() error {
	log.Info("Shutting down AI transaction predictor...")
	
	p.cancel()
	p.wg.Wait()
	
	log.Info("AI transaction predictor shutdown complete")
	return nil
}


// Global AI transaction predictor instance
var globalAITransactionPredictor *AITransactionPredictor

// InitGlobalAITransactionPredictor initializes the global AI transaction predictor
func InitGlobalAITransactionPredictor(aiLoadBalancer *AILoadBalancer) error {
	if globalAITransactionPredictor != nil {
		globalAITransactionPredictor.Close()
	}
	
	var err error
	globalAITransactionPredictor, err = NewAITransactionPredictor(aiLoadBalancer)
	return err
}

// GetGlobalAITransactionPredictor returns the global AI transaction predictor
func GetGlobalAITransactionPredictor() *AITransactionPredictor {
	return globalAITransactionPredictor
}

// CloseGlobalAITransactionPredictor closes the global AI transaction predictor
func CloseGlobalAITransactionPredictor() error {
	if globalAITransactionPredictor != nil {
		return globalAITransactionPredictor.Close()
	}
	return nil
}
