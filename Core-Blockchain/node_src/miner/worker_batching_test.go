package miner

import (
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hybrid"
)

func TestCalculateOptimalBatchSizeIncreasesTowardsTarget(t *testing.T) {
	const targetTPS = 100000
	t.Setenv("THROUGHPUT_TARGET", strconv.FormatUint(targetTPS, 10))
	t.Setenv("GPU_MAX_BATCH_SIZE", strconv.FormatUint(1000000, 10))

	w := &worker{
		batchThreshold:   1000,
		adaptiveBatching: true,
	}

	baseStats := hybrid.HybridStats{
		GPUUtilization: 0.75,
		CPUUtilization: 0.75,
	}

	ratios := []float64{0.60, 0.75, 0.90}
	var batches []int

	for _, ratio := range ratios {
		stats := baseStats
		stats.CurrentTPS = uint64(float64(targetTPS) * ratio)
		w.hybridStatsOverride = &stats

		batch := w.calculateOptimalBatchSize()
		batches = append(batches, batch)

		w.updateBatchPerformance(batch, 60*time.Millisecond)
	}

	for i := 1; i < len(batches); i++ {
		if batches[i] <= batches[i-1] {
			t.Fatalf("expected batch size at step %d (ratio %.2f) to exceed previous value: %d <= %d", i, ratios[i], batches[i], batches[i-1])
		}
	}

	if len(batches) >= 3 {
		firstGrowth := batches[1] - batches[0]
		secondGrowth := batches[2] - batches[1]
		if secondGrowth >= firstGrowth {
			t.Fatalf("expected growth to slow as TPS approaches target: %d >= %d", secondGrowth, firstGrowth)
		}
	}
}
