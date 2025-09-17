package miner

import (
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hybrid"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/params"
)

func TestCalculateOptimalBatchSizeIncreasesTowardsTarget(t *testing.T) {
	const targetTPS = 100000
	t.Setenv("THROUGHPUT_TARGET", strconv.FormatUint(targetTPS, 10))
	t.Setenv("GPU_MAX_BATCH_SIZE", strconv.FormatUint(1000000, 10))

	w := &worker{
		batchThreshold:   1000,
		adaptiveBatching: true,
	}
	w.hybridThroughputTarget = targetTPS

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

func TestCommitTransactionsProcessesUnderThresholdBatch(t *testing.T) {
	engine := ethash.NewFaker()
	defer engine.Close()

	chainConfig := new(params.ChainConfig)
	*chainConfig = *params.AllEthashProtocolChanges

	w, backend := newTestWorker(t, chainConfig, engine, rawdb.NewMemoryDatabase(), 0)
	defer backend.chain.Stop()
	defer w.close()

	w.gpuEnabled = true
	w.hybridProcessor = &hybrid.HybridProcessor{}
	w.adaptiveBatching = false
	w.batchThreshold = 8

	pending := backend.txPool.Pending(true)
	expected := make(map[common.Hash]struct{})
	for _, txs := range pending {
		for _, tx := range txs {
			expected[tx.Hash()] = struct{}{}
		}
	}
	if len(expected) == 0 {
		tx := backend.newRandomTx(false)
		if err := backend.txPool.AddLocal(tx); err != nil {
			t.Fatalf("failed to add local transaction: %v", err)
		}
		expected[tx.Hash()] = struct{}{}
	}

	if len(expected) == 0 {
		t.Fatal("expected pending transactions for GPU staging test")
	}
	if len(expected) >= w.batchThreshold/2 {
		t.Fatalf("need fewer pending txs than half threshold: have %d, limit %d", len(expected), w.batchThreshold/2)
	}

	w.commitNewWork(nil, false, time.Now().Unix())

	if len(w.current.txs) != len(expected) {
		t.Fatalf("expected %d transactions in block, got %d", len(expected), len(w.current.txs))
	}
	for _, tx := range w.current.txs {
		if _, ok := expected[tx.Hash()]; !ok {
			t.Fatalf("unexpected transaction %s in block", tx.Hash())
		}
		delete(expected, tx.Hash())
	}
	if len(expected) != 0 {
		t.Fatalf("missing %d transactions from block", len(expected))
	}
}
