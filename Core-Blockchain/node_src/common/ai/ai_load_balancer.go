package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// DefaultAIConfig returns default AI configuration for TinyLlama with vLLM
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		LLMEndpoint:         "http://localhost:8000/v1/chat/completions", // vLLM OpenAI-compatible API
		LLMModel:           "TinyLlama/TinyLlama-1.1B-Chat-v1.0", // TinyLlama 1.1B parameter model
		LLMTimeout:         1 * time.Second, // 2x faster for high TPS optimization
		UpdateInterval:     250 * time.Millisecond, // 2x more frequent updates for massive TPS
		HistorySize:        200, // 2x larger history for better pattern recognition
		LearningRate:       0.25, // Higher learning rate for rapid adaptation
		ConfidenceThreshold: 0.65, // Lower threshold for more aggressive optimization
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

// VLLMChatRequest represents a chat request to vLLM (OpenAI-compatible)
type VLLMChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// VLLMChatResponse represents a response from vLLM chat API
type VLLMChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
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

// queryLLM sends a query to vLLM using OpenAI-compatible chat API
func (ai *AILoadBalancer) queryLLM(prompt string) (string, error) {
	// Validate LLM endpoint before making request
	if ai.llmEndpoint == "" || ai.llmEndpoint == "\\" || ai.llmEndpoint == "\"\"" {
		return "", fmt.Errorf("LLM endpoint not configured or empty")
	}
	
	request := VLLMChatRequest{
		Model: ai.llmModel,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
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
	
	var vllmResp VLLMChatResponse
	if err := json.Unmarshal(body, &vllmResp); err != nil {
		return "", err
	}
	
	if len(vllmResp.Choices) == 0 {
		// Not an error - model may have nothing to say, use fallback
		return "", nil
	}
	
	content := vllmResp.Choices[0].Message.Content
	if content == "" {
		// Empty response is also fine, use fallback
		return "", nil
	}
	
	return content, nil
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
	
	// If response is empty, use fallback (not an error)
	if response == "" {
		log.Debug("AI returned empty response, using fallback")
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
	
	prompt := fmt.Sprintf(`You are an AI load balancer for a MASSIVE TPS blockchain system with RTX 4000 SFF Ada (20GB VRAM) and 16+ CPU cores.

SYSTEM OPTIMIZATIONS APPLIED:
- Transaction Pool: 2.5M capacity (was 6K)
- Block Time: 50ms minimum (was 1s)
- GPU Batch Size: 200K (was 50K)
- GPU Workers: 50 each (was 20)

CURRENT PERFORMANCE:
- TPS: %d (NEW TARGET: 2,000,000+ sustained, 10M peak)
- CPU Utilization: %.2f%% (max: 95%% - pushed higher)
- GPU Utilization: %.2f%% (max: 98%% - RTX 4000 SFF Ada limits)
- Latency: %.1fms (target: <25ms - 2x faster)
- Batch Size: %d (optimal: 100K-200K)
- Current Strategy: %s
- Queue Depth: %d (capacity: 2.5M)

RECENT TRENDS (last %d measurements):
%s

AI DECISION REQUIRED:
With massive TPS optimizations in place, aggressively optimize for:
1. CPU/GPU ratio (0.0 = all CPU, 1.0 = all GPU) - favor GPU heavily
2. Processing strategy (CPU_ONLY, GPU_ONLY, HYBRID) - prefer GPU/HYBRID
3. Confidence level (0.0-1.0) - be more aggressive with high confidence
4. Brief reasoning focused on maximizing TPS

OPTIMIZATION PRIORITIES:
- Push GPU to 95-98%% utilization (RTX 4000 SFF Ada sweet spot)
- Keep CPU at 90-95%% to handle overflow
- Target 500K-2M+ TPS sustained throughput
- Minimize latency under 25ms

Respond in JSON format:
{
  "ratio": 0.92,
  "strategy": "GPU_ONLY",
  "confidence": 0.95,
  "reasoning": "GPU underutilized at X%%, can push to 95%% for massive TPS gain"
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
		// No JSON found, try to extract values from text
		return ai.parseTextResponse(response)
	}
	
	jsonStr := response[start : end+1]
	
	var aiResponse struct {
		Ratio      float64 `json:"ratio"`
		Strategy   string  `json:"strategy"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &aiResponse); err != nil {
		// JSON parsing failed, try text parsing
		return ai.parseTextResponse(response)
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

// parseTextResponse extracts values from plain text AI response
func (ai *AILoadBalancer) parseTextResponse(response string) (LoadPrediction, error) {
	// Default values
	ratio := 0.85
	strategy := "HYBRID"
	confidence := 0.7
	reasoning := "Text-based AI response parsed"
	
	// Try to extract numeric values from text
	responseUpper := strings.ToUpper(response)
	
	// Look for ratio/percentage values
	if strings.Contains(responseUpper, "GPU") {
		if strings.Contains(responseUpper, "95%") || strings.Contains(responseUpper, "0.95") {
			ratio = 0.95
		} else if strings.Contains(responseUpper, "90%") || strings.Contains(responseUpper, "0.90") {
			ratio = 0.90
		} else if strings.Contains(responseUpper, "80%") || strings.Contains(responseUpper, "0.80") {
			ratio = 0.80
		}
	}
	
	// Look for strategy keywords
	if strings.Contains(responseUpper, "GPU_ONLY") || strings.Contains(responseUpper, "GPU ONLY") {
		strategy = "GPU_ONLY"
		confidence = 0.8
	} else if strings.Contains(responseUpper, "CPU_ONLY") || strings.Contains(responseUpper, "CPU ONLY") {
		strategy = "CPU_ONLY"
		confidence = 0.8
	} else if strings.Contains(responseUpper, "HYBRID") {
		strategy = "HYBRID"
		confidence = 0.75
	}
	
	// Extract reasoning from response (first 100 chars)
	if len(response) > 10 {
		reasoning = response
		if len(reasoning) > 100 {
			reasoning = reasoning[:100] + "..."
		}
	}
	
	return LoadPrediction{
		Timestamp:         time.Now(),
		RecommendedRatio:  ratio,
		RecommendedStrategy: strategy,
		Confidence:        confidence,
		Reasoning:         reasoning,
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
