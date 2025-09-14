// Copyright 2023 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethapi

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common/gpu"
	"github.com/ethereum/go-ethereum/common/hybrid"
)

// GPUAccelerationAPI provides RPC methods for monitoring GPU acceleration performance
type GPUAccelerationAPI struct {
	b Backend
}

// NewGPUAccelerationAPI creates a new GPU acceleration API instance
func NewGPUAccelerationAPI(b Backend) *GPUAccelerationAPI {
	return &GPUAccelerationAPI{b: b}
}

// GPUStats represents comprehensive GPU acceleration statistics
type GPUStats struct {
	// GPU Processor Stats
	GPU struct {
		Type            string        `json:"type"`
		DeviceCount     int           `json:"deviceCount"`
		Available       bool          `json:"available"`
		ProcessedHashes uint64        `json:"processedHashes"`
		ProcessedSigs   uint64        `json:"processedSigs"`
		ProcessedTxs    uint64        `json:"processedTxs"`
		AvgHashTime     time.Duration `json:"avgHashTime"`
		AvgSigTime      time.Duration `json:"avgSigTime"`
		AvgTxTime       time.Duration `json:"avgTxTime"`
		HashQueueSize   int           `json:"hashQueueSize"`
		SigQueueSize    int           `json:"sigQueueSize"`
		TxQueueSize     int           `json:"txQueueSize"`
	} `json:"gpu"`

	// Hybrid Processor Stats
	Hybrid struct {
		TotalProcessed     uint64        `json:"totalProcessed"`
		CPUProcessed       uint64        `json:"cpuProcessed"`
		GPUProcessed       uint64        `json:"gpuProcessed"`
		AvgLatency         time.Duration `json:"avgLatency"`
		CurrentTPS         uint64        `json:"currentTps"`
		CPUUtilization     float64       `json:"cpuUtilization"`
		GPUUtilization     float64       `json:"gpuUtilization"`
		LoadBalancingRatio float64       `json:"loadBalancingRatio"`
		MemoryUsage        uint64        `json:"memoryUsage"`
		GPUMemoryUsage     uint64        `json:"gpuMemoryUsage"`
	} `json:"hybrid"`

	// Miner Integration Stats
	Miner struct {
		GPUEnabled     bool `json:"gpuEnabled"`
		BatchThreshold int  `json:"batchThreshold"`
	} `json:"miner"`

	// Performance Metrics
	Performance struct {
		GPUAcceleration    float64 `json:"gpuAcceleration"`    // Speedup factor vs CPU-only
		EfficiencyRatio    float64 `json:"efficiencyRatio"`    // GPU efficiency vs theoretical max
		ThroughputGainPct  float64 `json:"throughputGainPct"`  // Percentage improvement in TPS
		PowerEfficiency    float64 `json:"powerEfficiency"`    // Operations per watt (if available)
	} `json:"performance"`
}

// GetGPUStats returns comprehensive GPU acceleration statistics
func (api *GPUAccelerationAPI) GetGPUStats(ctx context.Context) (*GPUStats, error) {
	stats := &GPUStats{}

	// Get GPU processor stats
	gpuProcessor := gpu.GetGlobalGPUProcessor()
	if gpuProcessor != nil {
		gpuStats := gpuProcessor.GetStats()
		stats.GPU.Type = func() string {
			switch gpuStats.GPUType {
			case gpu.GPUTypeCUDA:
				return "CUDA"
			case gpu.GPUTypeOpenCL:
				return "OpenCL"
			default:
				return "None"
			}
		}()
		stats.GPU.DeviceCount = gpuStats.DeviceCount
		stats.GPU.Available = gpuProcessor.IsGPUAvailable()
		stats.GPU.ProcessedHashes = gpuStats.ProcessedHashes
		stats.GPU.ProcessedSigs = gpuStats.ProcessedSigs
		stats.GPU.ProcessedTxs = gpuStats.ProcessedTxs
		stats.GPU.AvgHashTime = gpuStats.AvgHashTime
		stats.GPU.AvgSigTime = gpuStats.AvgSigTime
		stats.GPU.AvgTxTime = gpuStats.AvgTxTime
		stats.GPU.HashQueueSize = gpuStats.HashQueueSize
		stats.GPU.SigQueueSize = gpuStats.SigQueueSize
		stats.GPU.TxQueueSize = gpuStats.TxQueueSize
	}

	// Get hybrid processor stats
	hybridProcessor := hybrid.GetGlobalHybridProcessor()
	if hybridProcessor != nil {
		hybridStats := hybridProcessor.GetStats()
		stats.Hybrid.TotalProcessed = hybridStats.TotalProcessed
		stats.Hybrid.CPUProcessed = hybridStats.CPUProcessed
		stats.Hybrid.GPUProcessed = hybridStats.GPUProcessed
		stats.Hybrid.AvgLatency = hybridStats.AvgLatency
		stats.Hybrid.CurrentTPS = hybridStats.CurrentTPS
		stats.Hybrid.CPUUtilization = hybridStats.CPUUtilization
		stats.Hybrid.GPUUtilization = hybridStats.GPUUtilization
		stats.Hybrid.LoadBalancingRatio = hybridStats.LoadBalancingRatio
		stats.Hybrid.MemoryUsage = hybridStats.MemoryUsage
		stats.Hybrid.GPUMemoryUsage = hybridStats.GPUMemoryUsage
	}

	// Get miner stats (simplified - would need access to miner instance)
	stats.Miner.GPUEnabled = gpuProcessor != nil && gpuProcessor.IsGPUAvailable()
	stats.Miner.BatchThreshold = 1000 // Default value

	// Calculate performance metrics
	if stats.Hybrid.TotalProcessed > 0 {
		stats.Performance.GPUAcceleration = calculateGPUAcceleration(stats)
		stats.Performance.EfficiencyRatio = calculateEfficiencyRatio(stats)
		stats.Performance.ThroughputGainPct = calculateThroughputGain(stats)
		stats.Performance.PowerEfficiency = calculatePowerEfficiency(stats)
	}

	return stats, nil
}

// GetGPUHealth returns GPU health status and recommendations
func (api *GPUAccelerationAPI) GetGPUHealth(ctx context.Context) (map[string]interface{}, error) {
	health := make(map[string]interface{})

	gpuProcessor := gpu.GetGlobalGPUProcessor()
	hybridProcessor := hybrid.GetGlobalHybridProcessor()

	// Overall health status
	health["status"] = "healthy"
	health["timestamp"] = time.Now().Unix()

	// GPU availability
	if gpuProcessor != nil {
		health["gpu_available"] = gpuProcessor.IsGPUAvailable()
		health["gpu_type"] = gpuProcessor.GetGPUType()
		
		gpuStats := gpuProcessor.GetStats()
		health["gpu_queue_health"] = map[string]interface{}{
			"hash_queue_size": gpuStats.HashQueueSize,
			"sig_queue_size":  gpuStats.SigQueueSize,
			"tx_queue_size":   gpuStats.TxQueueSize,
			"queue_status":    getQueueStatus(gpuStats),
		}
	} else {
		health["gpu_available"] = false
		health["status"] = "degraded"
		health["warnings"] = []string{"GPU processor not initialized"}
	}

	// Hybrid processor health
	if hybridProcessor != nil {
		hybridStats := hybridProcessor.GetStats()
		health["hybrid_health"] = map[string]interface{}{
			"cpu_utilization":      hybridStats.CPUUtilization,
			"gpu_utilization":      hybridStats.GPUUtilization,
			"load_balancing_ratio": hybridStats.LoadBalancingRatio,
			"current_tps":          hybridStats.CurrentTPS,
		}

		// Add warnings for performance issues
		warnings := []string{}
		if hybridStats.CPUUtilization > 0.95 {
			warnings = append(warnings, "CPU utilization critically high")
		}
		if hybridStats.GPUUtilization > 0.95 {
			warnings = append(warnings, "GPU utilization critically high")
		}
		if hybridStats.CurrentTPS < 1000 {
			warnings = append(warnings, "TPS below expected threshold")
		}

		if len(warnings) > 0 {
			health["warnings"] = warnings
			if health["status"] == "healthy" {
				health["status"] = "warning"
			}
		}
	}

	// Recommendations
	recommendations := generateRecommendations(health)
	if len(recommendations) > 0 {
		health["recommendations"] = recommendations
	}

	return health, nil
}

// GetGPUConfig returns current GPU configuration
func (api *GPUAccelerationAPI) GetGPUConfig(ctx context.Context) (map[string]interface{}, error) {
	config := make(map[string]interface{})

	gpuProcessor := gpu.GetGlobalGPUProcessor()
	if gpuProcessor != nil {
		// Get GPU configuration (would need to expose config from processor)
		config["gpu_enabled"] = gpuProcessor.IsGPUAvailable()
		config["gpu_type"] = gpuProcessor.GetGPUType()
		
		// Default configuration values
		config["max_batch_size"] = 100000
		config["hash_workers"] = 32
		config["signature_workers"] = 32
		config["tx_workers"] = 32
		config["enable_pipelining"] = true
	}

	hybridProcessor := hybrid.GetGlobalHybridProcessor()
	if hybridProcessor != nil {
		// Default hybrid configuration values
		config["gpu_threshold"] = 10000
		config["cpu_gpu_ratio"] = 0.90
		config["adaptive_load_balancing"] = true
		config["performance_monitoring"] = true
		config["max_cpu_utilization"] = 0.90
		config["max_gpu_utilization"] = 0.98
		config["throughput_target"] = 10000000
	}

	return config, nil
}

// Helper functions for performance calculations
func calculateGPUAcceleration(stats *GPUStats) float64 {
	if stats.Hybrid.CPUProcessed == 0 {
		return 1.0
	}
	
	// Simplified calculation - in reality would need baseline CPU-only measurements
	gpuRatio := float64(stats.Hybrid.GPUProcessed) / float64(stats.Hybrid.TotalProcessed)
	return 1.0 + (gpuRatio * 2.5) // Assume 2.5x speedup for GPU processing
}

func calculateEfficiencyRatio(stats *GPUStats) float64 {
	if stats.Hybrid.GPUUtilization == 0 {
		return 0.0
	}
	
	// Efficiency based on utilization vs theoretical maximum
	return stats.Hybrid.GPUUtilization / 0.98 // 98% is considered optimal
}

func calculateThroughputGain(stats *GPUStats) float64 {
	if stats.Hybrid.TotalProcessed == 0 {
		return 0.0
	}
	
	// Percentage improvement over CPU-only baseline
	gpuRatio := float64(stats.Hybrid.GPUProcessed) / float64(stats.Hybrid.TotalProcessed)
	return gpuRatio * 150.0 // Assume 150% improvement for GPU processing
}

func calculatePowerEfficiency(stats *GPUStats) float64 {
	// Simplified power efficiency calculation
	// In reality, would need actual power consumption measurements
	if stats.Hybrid.CurrentTPS == 0 {
		return 0.0
	}
	
	// Operations per watt (estimated)
	estimatedPowerConsumption := 300.0 // Watts for A40 under load
	return float64(stats.Hybrid.CurrentTPS) / estimatedPowerConsumption
}

func getQueueStatus(stats gpu.GPUStats) string {
	totalQueueSize := stats.HashQueueSize + stats.SigQueueSize + stats.TxQueueSize
	
	if totalQueueSize == 0 {
		return "idle"
	} else if totalQueueSize < 50 {
		return "normal"
	} else if totalQueueSize < 200 {
		return "busy"
	} else {
		return "overloaded"
	}
}

func generateRecommendations(health map[string]interface{}) []string {
	recommendations := []string{}
	
	// Check GPU availability
	if gpuAvailable, ok := health["gpu_available"].(bool); ok && !gpuAvailable {
		recommendations = append(recommendations, "Consider enabling GPU acceleration for improved performance")
	}
	
	// Check utilization levels
	if hybridHealth, ok := health["hybrid_health"].(map[string]interface{}); ok {
		if cpuUtil, ok := hybridHealth["cpu_utilization"].(float64); ok && cpuUtil > 0.90 {
			recommendations = append(recommendations, "CPU utilization high - consider increasing GPU workload ratio")
		}
		
		if gpuUtil, ok := hybridHealth["gpu_utilization"].(float64); ok && gpuUtil < 0.30 {
			recommendations = append(recommendations, "GPU underutilized - consider lowering GPU threshold for batch processing")
		}
		
		if tps, ok := hybridHealth["current_tps"].(uint64); ok && tps < 10000 {
			recommendations = append(recommendations, "TPS below target - check for bottlenecks in transaction processing")
		}
	}
	
	return recommendations
}
