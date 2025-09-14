package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/common/hybrid"
)

// AILoadBalancer uses a local LLM to make intelligent load balancing decisions
type AILoadBalancer struct {
	// LLM Configuration
	llmEndpoint     string
	llmModel        string
	llmTimeout      time.Duration
	
	// Performance History
	mu              sync.RWMutex
	performanceData []PerformanceMetrics
	predictions     []LoadPrediction
	
	// Decision Making
	decisionHistory []AIDecision
	confidence      float64
	learningRate    float64
	
	// Hybrid Processor Reference
	hybridProcessor *hybrid.HybridProcessor
	
	// Control
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// AIConfig holds configuration for AI load balancer
type AIConfig struct {
	LLMEndpoint      string        `json:"llmEndpoint"`
	LLMModel         string        `json:"llmModel"`
	LLMTimeout       time.Duration `json:"llmTimeout"`
	UpdateInterval   time.Duration `json:"updateInterval"`
	HistorySize      int           `json:"historySize"`
	LearningRate     float64       `json:"learningRate"`
	ConfidenceThreshold float64    `json:"confidenceThreshold"`
}

// DefaultAIConfig returns default AI configuration for Phi-3 Mini with vLLM
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		LLMEndpoint:         "http://localhost:8000/v1/completions", // vLLM OpenAI-compatible API
		LLMModel:           "microsoft/Phi-3-mini-4k-instruct", // Phi-3 Mini 3.8B parameter model
		LLMTimeout:         2 * time.Second, // Ultra-fast response with vLLM
		UpdateInterval:     500 * time.Millisecond, // Very frequent updates due to vLLM speed
		HistorySize:        100,
		LearningRate:       0.15,
		ConfidenceThreshold: 0.75,
	}
}

// PerformanceMetrics holds system performance data
type PerformanceMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalTPS        uint64    `json:"totalTps"`
	CPUUtilization  float64   `json:"cpuUtilization"`
	GPUUtilization  float64   `json:"gpuUtilization"`
	MemoryUsage     uint64    `json:"memoryUsage"`
	GPUMemoryUsage  uint64    `json:"gpuMemoryUsage"`
	AvgLatency      float64   `json:"avgLatency"`
	BatchSize       int       `json:"batchSize"`
	CurrentStrategy string    `json:"currentStrategy"`
	QueueDepth      int       `json:"queueDepth"`
}

// LoadPrediction holds AI predictions for load balancing
type LoadPrediction struct {
	Timestamp         time.Time `json:"timestamp"`
	PredictedTPS      uint64    `json:"predictedTps"`
	RecommendedRatio  float64   `json:"recommendedRatio"`
	RecommendedStrategy string  `json:"recommendedStrategy"`
	Confidence        float64   `json:"confidence"`
	Reasoning         string    `json:"reasoning"`
}

// AIDecision holds AI decision history
type AIDecision struct {
	Timestamp       time.Time `json:"timestamp"`
	Input           PerformanceMetrics `json:"input"`
	Decision        LoadPrediction `json:"decision"`
	ActualOutcome   PerformanceMetrics `json:"actualOutcome"`
	Success         bool       `json:"success"`
	PerformanceGain float64    `json:"performanceGain"`
}

// VLLMRequest represents a request to vLLM (OpenAI-compatible)
type VLLMRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	Stream      bool    `json:"stream"`
}

// VLLMResponse represents a response from vLLM
type VLLMResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
}

// NewAILoadBalancer creates a new AI-powered load balancer
func NewAILoadBalancer(config *AIConfig, hybridProcessor *hybrid.HybridProcessor) (*AILoadBalancer, error) {
	if config == nil {
		config = DefaultAIConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	ai := &AILoadBalancer{
		llmEndpoint:     config.LLMEndpoint,
		llmModel:        config.LLMModel,
		llmTimeout:      config.LLMTimeout,
		performanceData: make([]PerformanceMetrics, 0, config.HistorySize),
		predictions:     make([]LoadPrediction, 0, config.HistorySize),
		decisionHistory: make([]AIDecision, 0, config.HistorySize),
		confidence:      0.5, // Start with neutral confidence
		learningRate:    config.LearningRate,
		hybridProcessor: hybridProcessor,
		ctx:             ctx,
		cancel:          cancel,
	}
	
	// Start AI decision making loop regardless of LLM connection
	ai.wg.Add(1)
	go ai.aiDecisionLoop(config.UpdateInterval)
	
	log.Info("AI load balancer initialized (will attempt LLM connection in background)",
		"llmEndpoint", config.LLMEndpoint,
		"llmModel", config.LLMModel,
		"updateInterval", config.UpdateInterval,
	)
	
	return ai, nil
}

// testLLMConnection tests connection to local LLM
func (ai *AILoadBalancer) testLLMConnection() error {
	testPrompt := "Hello, respond with 'OK' if you can process blockchain performance data."
	
	response, err := ai.queryLLM(testPrompt)
	if err != nil {
		return fmt.Errorf("LLM connection test failed: %w", err)
	}
	
	log.Info("LLM connection successful", "response", response[:min(50, len(response))])
	return nil
}

// queryLLM sends a query to vLLM using OpenAI-compatible API
func (ai *AILoadBalancer) queryLLM(prompt string) (string, error) {
	request := VLLMRequest{
		Model:       ai.llmModel,
		Prompt:      prompt,
		MaxTokens:   200,
		Temperature: 0.1,
		Stream:      false,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	
	ctx, cancel := context.WithTimeout(ai.ctx, ai.llmTimeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "POST", ai.llmEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: ai.llmTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	var vllmResp VLLMResponse
	if err := json.Unmarshal(body, &vllmResp); err != nil {
		return "", err
	}
	
	if len(vllmResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices from vLLM")
	}
	
	return vllmResp.Choices[0].Text, nil
}

// aiDecisionLoop runs the AI decision making process
func (ai *AILoadBalancer) aiDecisionLoop(interval time.Duration) {
	defer ai.wg.Done()
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ai.ctx.Done():
			return
		case <-ticker.C:
			ai.makeAIDecision()
		}
	}
}

// makeAIDecision uses AI to make load balancing decisions
func (ai *AILoadBalancer) makeAIDecision() {
	// Collect current performance metrics
	metrics := ai.collectCurrentMetrics()
	
	// Store performance data
	ai.mu.Lock()
	ai.performanceData = append(ai.performanceData, metrics)
	if len(ai.performanceData) > 100 {
		ai.performanceData = ai.performanceData[1:]
	}
	ai.mu.Unlock()
	
	// Generate AI prompt with performance data
	prompt := ai.generateAIPrompt(metrics)
	
	// Query LLM for decision
	response, err := ai.queryLLM(prompt)
	if err != nil {
		log.Warn("AI decision failed, using fallback", "error", err)
		ai.fallbackDecision(metrics)
		return
	}
	
	// Parse AI response
	prediction, err := ai.parseAIResponse(response)
	if err != nil {
		log.Warn("AI response parsing failed", "error", err, "response", response)
		ai.fallbackDecision(metrics)
		return
	}
	
	// Apply AI decision if confidence is high enough
	if prediction.Confidence >= 0.7 {
		ai.applyAIDecision(prediction, metrics)
	} else {
		log.Debug("AI confidence too low, skipping decision", "confidence", prediction.Confidence)
	}
}

// collectCurrentMetrics gathers current system performance metrics
func (ai *AILoadBalancer) collectCurrentMetrics() PerformanceMetrics {
	stats := ai.hybridProcessor.GetStats()
	
	return PerformanceMetrics{
		Timestamp:       time.Now(),
		TotalTPS:        stats.CurrentTPS,
		CPUUtilization:  stats.CPUUtilization,
		GPUUtilization:  stats.GPUUtilization,
		MemoryUsage:     stats.MemoryUsage,
		GPUMemoryUsage:  stats.GPUMemoryUsage,
		AvgLatency:      float64(stats.AvgLatency.Milliseconds()),
		BatchSize:       ai.estimateCurrentBatchSize(),
		CurrentStrategy: ai.getCurrentStrategy(),
		QueueDepth:      ai.getQueueDepth(),
	}
}

// generateAIPrompt creates a prompt for the LLM
func (ai *AILoadBalancer) generateAIPrompt(current PerformanceMetrics) string {
	// Get recent performance history
	ai.mu.RLock()
	historySize := min(10, len(ai.performanceData))
	recentHistory := make([]PerformanceMetrics, historySize)
	if historySize > 0 {
		copy(recentHistory, ai.performanceData[len(ai.performanceData)-historySize:])
	}
	ai.mu.RUnlock()
	
	prompt := fmt.Sprintf(`You are an AI load balancer for a high-performance blockchain system with RTX 4090 GPU and 16+ CPU cores.

CURRENT PERFORMANCE:
- TPS: %d (target: 5,000,000)
- CPU Utilization: %.2f%% (max: 90%%)
- GPU Utilization: %.2f%% (max: 95%%)
- Latency: %.1fms (target: <50ms)
- Batch Size: %d
- Current Strategy: %s
- Queue Depth: %d

RECENT TRENDS (last %d measurements):
%s

DECISION REQUIRED:
Based on the current performance and trends, recommend:
1. CPU/GPU ratio (0.0 = all CPU, 1.0 = all GPU)
2. Processing strategy (CPU_ONLY, GPU_ONLY, HYBRID)
3. Confidence level (0.0-1.0)
4. Brief reasoning

Respond in JSON format:
{
  "ratio": 0.85,
  "strategy": "HYBRID",
  "confidence": 0.9,
  "reasoning": "High GPU utilization with good performance, maintain current ratio"
}`,
		current.TotalTPS,
		current.CPUUtilization*100,
		current.GPUUtilization*100,
		current.AvgLatency,
		current.BatchSize,
		current.CurrentStrategy,
		current.QueueDepth,
		historySize,
		ai.formatHistoryForPrompt(recentHistory),
	)
	
	return prompt
}

// formatHistoryForPrompt formats performance history for the AI prompt
func (ai *AILoadBalancer) formatHistoryForPrompt(history []PerformanceMetrics) string {
	if len(history) == 0 {
		return "No historical data available"
	}
	
	var buffer bytes.Buffer
	for i, metrics := range history {
		buffer.WriteString(fmt.Sprintf("  %d. TPS: %d, CPU: %.1f%%, GPU: %.1f%%, Latency: %.1fms\n",
			i+1, metrics.TotalTPS, metrics.CPUUtilization*100, metrics.GPUUtilization*100, metrics.AvgLatency))
	}
	
	return buffer.String()
}

// parseAIResponse parses the LLM response into a LoadPrediction
func (ai *AILoadBalancer) parseAIResponse(response string) (LoadPrediction, error) {
	// Try to extract JSON from response
	start := bytes.Index([]byte(response), []byte("{"))
	end := bytes.LastIndex([]byte(response), []byte("}"))
	
	if start == -1 || end == -1 || start >= end {
		return LoadPrediction{}, fmt.Errorf("no valid JSON found in response")
	}
	
	jsonStr := response[start : end+1]
	
	var aiResponse struct {
		Ratio      float64 `json:"ratio"`
		Strategy   string  `json:"strategy"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &aiResponse); err != nil {
		return LoadPrediction{}, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Validate response
	if aiResponse.Ratio < 0 || aiResponse.Ratio > 1 {
		aiResponse.Ratio = 0.85 // Default to 85% GPU
	}
	
	if aiResponse.Confidence < 0 || aiResponse.Confidence > 1 {
		aiResponse.Confidence = 0.5 // Default confidence
	}
	
	return LoadPrediction{
		Timestamp:         time.Now(),
		RecommendedRatio:  aiResponse.Ratio,
		RecommendedStrategy: aiResponse.Strategy,
		Confidence:        aiResponse.Confidence,
		Reasoning:         aiResponse.Reasoning,
	}, nil
}

// applyAIDecision applies the AI recommendation to the hybrid processor
func (ai *AILoadBalancer) applyAIDecision(prediction LoadPrediction, currentMetrics PerformanceMetrics) {
	// Store the decision
	decision := AIDecision{
		Timestamp: time.Now(),
		Input:     currentMetrics,
		Decision:  prediction,
	}
	
	ai.mu.Lock()
	ai.decisionHistory = append(ai.decisionHistory, decision)
	if len(ai.decisionHistory) > 50 {
		ai.decisionHistory = ai.decisionHistory[1:]
	}
	
	ai.predictions = append(ai.predictions, prediction)
	if len(ai.predictions) > 100 {
		ai.predictions = ai.predictions[1:]
	}
	ai.mu.Unlock()
	
	// Apply the decision to hybrid processor
	// Note: This would require extending the hybrid processor to accept AI recommendations
	log.Info("AI load balancing decision applied",
		"ratio", prediction.RecommendedRatio,
		"strategy", prediction.RecommendedStrategy,
		"confidence", prediction.Confidence,
		"reasoning", prediction.Reasoning,
	)
}

// fallbackDecision makes a simple rule-based decision when AI fails
func (ai *AILoadBalancer) fallbackDecision(metrics PerformanceMetrics) {
	var ratio float64 = 0.85 // Default
	var strategy string = "HYBRID"
	
	// Simple rules
	if metrics.CPUUtilization > 0.9 && metrics.GPUUtilization < 0.8 {
		ratio = 0.95 // More GPU
		strategy = "GPU_ONLY"
	} else if metrics.GPUUtilization > 0.95 && metrics.CPUUtilization < 0.8 {
		ratio = 0.7 // More CPU
		strategy = "HYBRID"
	}
	
	prediction := LoadPrediction{
		Timestamp:         time.Now(),
		RecommendedRatio:  ratio,
		RecommendedStrategy: strategy,
		Confidence:        0.6, // Lower confidence for fallback
		Reasoning:         "Fallback rule-based decision",
	}
	
	ai.applyAIDecision(prediction, metrics)
}

// Helper functions
func (ai *AILoadBalancer) estimateCurrentBatchSize() int {
	// This would be implemented based on current transaction queue
	return 10000 // Placeholder
}

func (ai *AILoadBalancer) getCurrentStrategy() string {
	// This would query the hybrid processor for current strategy
	return "HYBRID" // Placeholder
}

func (ai *AILoadBalancer) getQueueDepth() int {
	// This would query the current queue depth
	return 1000 // Placeholder
}

// GetStats returns current AI load balancer statistics
func (ai *AILoadBalancer) GetStats() AIStats {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	
	return AIStats{
		TotalDecisions:    len(ai.decisionHistory),
		SuccessfulDecisions: ai.countSuccessfulDecisions(),
		AverageConfidence: ai.calculateAverageConfidence(),
		CurrentConfidence: ai.confidence,
		LearningRate:     ai.learningRate,
		LastPrediction:   ai.getLastPrediction(),
	}
}

// AIStats holds AI load balancer statistics
type AIStats struct {
	TotalDecisions      int            `json:"totalDecisions"`
	SuccessfulDecisions int            `json:"successfulDecisions"`
	AverageConfidence   float64        `json:"averageConfidence"`
	CurrentConfidence   float64        `json:"currentConfidence"`
	LearningRate        float64        `json:"learningRate"`
	LastPrediction      LoadPrediction `json:"lastPrediction"`
}

func (ai *AILoadBalancer) countSuccessfulDecisions() int {
	count := 0
	for _, decision := range ai.decisionHistory {
		if decision.Success {
			count++
		}
	}
	return count
}

func (ai *AILoadBalancer) calculateAverageConfidence() float64 {
	if len(ai.predictions) == 0 {
		return 0
	}
	
	total := 0.0
	for _, pred := range ai.predictions {
		total += pred.Confidence
	}
	
	return total / float64(len(ai.predictions))
}

func (ai *AILoadBalancer) getLastPrediction() LoadPrediction {
	if len(ai.predictions) == 0 {
		return LoadPrediction{}
	}
	return ai.predictions[len(ai.predictions)-1]
}

// Close gracefully shuts down the AI load balancer
func (ai *AILoadBalancer) Close() error {
	log.Info("Shutting down AI load balancer...")
	
	ai.cancel()
	ai.wg.Wait()
	
	log.Info("AI load balancer shutdown complete")
	return nil
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Global AI load balancer instance
var globalAILoadBalancer *AILoadBalancer

// InitGlobalAILoadBalancer initializes the global AI load balancer
func InitGlobalAILoadBalancer(config *AIConfig, hybridProcessor *hybrid.HybridProcessor) error {
	if globalAILoadBalancer != nil {
		globalAILoadBalancer.Close()
	}
	
	var err error
	globalAILoadBalancer, err = NewAILoadBalancer(config, hybridProcessor)
	return err
}

// GetGlobalAILoadBalancer returns the global AI load balancer
func GetGlobalAILoadBalancer() *AILoadBalancer {
	return globalAILoadBalancer
}

// CloseGlobalAILoadBalancer closes the global AI load balancer
func CloseGlobalAILoadBalancer() error {
	if globalAILoadBalancer != nil {
		return globalAILoadBalancer.Close()
	}
	return nil
}
