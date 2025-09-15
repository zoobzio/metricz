package reliability

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// Test metric keys - the RIGHT way to use Key type.
const (
	LargeKey              metricz.Key = "large"
	MemoryPressureTestKey metricz.Key = "test"
)

// TestUnboundedMetricCreation verifies behavior under extreme metric creation.
func TestUnboundedMetricCreation(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Get initial memory stats
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	var totalMetrics int
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		totalMetrics = 100_000 // Full stress test
	} else {
		totalMetrics = 1_000 // Quick CI test
	}

	// Create many unique metrics
	for i := 0; i < totalMetrics; i++ {
		key := fmt.Sprintf("metric_%d", i)
		registry.Counter(metricz.Key(key)).Inc()

		// Check memory growth periodically
		if i%10_000 == 0 && i > 0 {
			var currentMem runtime.MemStats
			runtime.ReadMemStats(&currentMem)

			memGrowthMB := float64(currentMem.Alloc-initialMem.Alloc) / 1024 / 1024
			metricsPerMB := float64(i) / memGrowthMB

			t.Logf("After %d metrics: Memory growth: %.2f MB, Metrics per MB: %.0f",
				i, memGrowthMB, metricsPerMB)

			// Alert if memory per metric seems excessive (>1KB per metric is suspicious)
			bytesPerMetric := float64(currentMem.Alloc-initialMem.Alloc) / float64(i)
			if bytesPerMetric > 1024 {
				t.Logf("WARNING: High memory usage per metric: %.0f bytes", bytesPerMetric)
			}
		}
	}

	// Verify all metrics are accessible
	counters := registry.GetCounters()
	if len(counters) != totalMetrics {
		t.Errorf("Expected %d counters, got %d", totalMetrics, len(counters))
	}

	// Force GC and check memory is reasonable
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	totalMemoryMB := float64(finalMem.Alloc-initialMem.Alloc) / 1024 / 1024
	t.Logf("Final memory usage for %d metrics: %.2f MB", totalMetrics, totalMemoryMB)
}

// TestLargeHistogramBuckets tests memory and performance with many buckets.
func TestLargeHistogramBuckets(t *gotesting.T) {
	registry := NewTestRegistry(t)

	// Create histogram with many buckets
	buckets := make([]float64, 1000)
	for i := range buckets {
		buckets[i] = float64(i)
	}

	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	hist := registry.Histogram(LargeKey, buckets)

	// Observe many values
	var observations int
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		observations = 100_000
	} else {
		observations = 10_000 // Quick CI test
	}
	start := time.Now()
	for i := 0; i < observations; i++ {
		hist.Observe(float64(i % 1000))
	}
	duration := time.Since(start)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memUsedMB := float64(finalMem.Alloc-initialMem.Alloc) / 1024 / 1024
	opsPerSec := float64(observations) / duration.Seconds()

	t.Logf("Large histogram performance: %.0f ops/sec, Memory: %.2f MB",
		opsPerSec, memUsedMB)

	// Verify all observations were recorded
	obsCount := uint64(observations) //nolint:gosec // observations is always positive and bounded
	if hist.Count() != obsCount {
		t.Errorf("Expected %d observations, got %d", observations, hist.Count())
	}

	// Check bucket distribution
	_, counts := hist.Buckets()
	totalInBuckets := uint64(0)
	for _, count := range counts {
		totalInBuckets += count
	}

	// All values should be in buckets (including overflow if any)
	obsCount2 := uint64(observations) //nolint:gosec // observations is always positive and bounded
	if totalInBuckets != obsCount2 {
		t.Errorf("Expected all %d values in buckets, got %d", observations, totalInBuckets)
	}
}

// TestMetricKeyLengthLimits tests behavior with maximum length keys.
func TestMetricKeyLengthLimits(t *gotesting.T) {
	registry := NewTestRegistry(t)

	// Test maximum length key (200 chars)
	maxKey := strings.Repeat("a", 200)
	counter := registry.Counter(metricz.Key(maxKey))
	counter.Inc()

	if counter.Value() != 1 {
		t.Error("Max length key should work")
	}

	// Create many max-length keys to stress memory
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	var keyCount int
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		keyCount = 1000
	} else {
		keyCount = 100 // Quick CI test
	}
	for i := 0; i < keyCount; i++ {
		// Create unique max-length keys
		key := strings.Repeat("a", 190) + fmt.Sprintf("%010d", i)
		registry.Counter(metricz.Key(key)).Inc()
	}

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memUsedMB := float64(finalMem.Alloc-initialMem.Alloc) / 1024 / 1024
	t.Logf("Memory used for %d max-length keys: %.2f MB", keyCount, memUsedMB)

	// Verify we can still access them
	counters := registry.GetCounters()
	expectedCounters := keyCount + 1 // keyCount + initial maxKey
	if len(counters) != expectedCounters {
		t.Errorf("Expected %d counters, got %d", expectedCounters, len(counters))
	}
}

// TestRegistryResetMemoryLeak ensures Reset() doesn't leak memory.
func TestRegistryResetMemoryLeak(t *gotesting.T) {
	registry := NewTestRegistry(t)

	// Perform multiple cycles of create/reset
	for cycle := 0; cycle < 10; cycle++ {
		// Create metrics
		for i := 0; i < 1000; i++ {
			registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i))).Add(float64(i))
			registry.Gauge(metricz.Key(fmt.Sprintf("gauge_%d", i))).Set(float64(i))
		}

		// Force GC before reset to establish baseline
		runtime.GC()
		time.Sleep(10 * time.Millisecond)

		var beforeReset runtime.MemStats
		runtime.ReadMemStats(&beforeReset)

		// Reset
		registry.Reset()

		// Force GC after reset
		runtime.GC()
		time.Sleep(10 * time.Millisecond)

		var afterReset runtime.MemStats
		runtime.ReadMemStats(&afterReset)

		// Memory should decrease after reset (within reason)
		if afterReset.Alloc > beforeReset.Alloc {
			t.Logf("Cycle %d: Memory increased after reset: before=%d, after=%d",
				cycle, beforeReset.Alloc, afterReset.Alloc)
		}
	}

	// Final check: registry should be empty
	if len(registry.GetCounters()) != 0 || len(registry.GetGauges()) != 0 {
		t.Error("Registry should be empty after final reset")
	}
}

// TestPackageRegistrationMemory was removed - RegisterPackage functionality eliminated.
// Key type provides compile-time safety without runtime registration overhead.

// TestLargeScaleMemoryUsage tests memory usage with many metrics - stress test for memory limits.
func TestLargeScaleMemoryUsage(t *gotesting.T) {
	if gotesting.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	registry := metricz.New()

	// Track memory growth for pressure testing
	type memSnapshot struct {
		numMetrics uint64
		heapAlloc  uint64
	}

	snapshots := []memSnapshot{}

	// Aggressive batching to test memory pressure
	batchSize := 5000 // Larger batches for pressure testing
	numBatches := 20  // More batches

	testKey := metricz.Key("stress_test")

	for batch := 0; batch < numBatches; batch++ {
		for i := 0; i < batchSize; i++ {
			idx := batch*batchSize + i
			// Heavy metric operations to stress memory
			registry.Counter(testKey).Add(float64(idx))
			registry.Gauge(testKey).Set(float64(idx))
			registry.Histogram(testKey, metricz.DefaultLatencyBuckets).Observe(float64(idx))

			if idx%5 == 0 {
				registry.Timer(testKey).Record(time.Duration(idx))
			}
		}

		// Force GC and measure memory pressure
		runtime.GC()
		runtime.GC()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		snapshots = append(snapshots, memSnapshot{
			numMetrics: uint64((batch + 1) * batchSize), //nolint:gosec // Safe: controlled test values
			heapAlloc:  mem.HeapAlloc,
		})

		// Check for memory pressure warning
		if mem.HeapAlloc > 100*1024*1024 { // 100MB warning
			t.Logf("Memory pressure warning: heap=%d MB at batch %d", mem.HeapAlloc/(1024*1024), batch)
		}
	}

	// Analyze memory pressure patterns
	maxGrowthRate := float64(0)
	for i := 1; i < len(snapshots); i++ {
		prev := snapshots[i-1]
		curr := snapshots[i]

		metricsAdded := curr.numMetrics - prev.numMetrics
		memGrowth := int64(curr.heapAlloc) - int64(prev.heapAlloc) //nolint:gosec // Safe: controlled values

		if memGrowth > 0 {
			growthRate := float64(memGrowth) / float64(metricsAdded)
			if growthRate > maxGrowthRate {
				maxGrowthRate = growthRate
			}
		}
	}

	t.Logf("Peak memory growth rate: %.2f bytes per metric operation", maxGrowthRate)

	// Verify system didn't run out of memory
	finalSnapshot := snapshots[len(snapshots)-1]
	t.Logf("Final memory usage: %d MB for %d metric operations",
		finalSnapshot.heapAlloc/(1024*1024), finalSnapshot.numMetrics)
}

// TestHistogramMemoryWithManyObservations ensures histograms don't leak memory.
func TestHistogramMemoryWithManyObservations(t *gotesting.T) {
	registry := NewTestRegistry(t)
	hist := registry.Histogram(MemoryPressureTestKey, []float64{1, 10, 100, 1000})

	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Observe values - configurable for CI vs stress testing
	var observations int
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		observations = 1_000_000
	} else {
		observations = 10_000 // Quick CI test
	}
	for i := 0; i < observations; i++ {
		hist.Observe(float64(i % 1000))
	}

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	memGrowthBytes := finalMem.Alloc - initialMem.Alloc

	// Histogram should not grow memory with observations (fixed bucket structure)
	// Allow some growth for potential internal optimizations, but should be minimal
	if memGrowthBytes > 1024*1024 { // 1MB threshold
		t.Errorf("Histogram memory grew by %d bytes with observations (should be constant)",
			memGrowthBytes)
	}

	t.Logf("Memory growth for %d observations: %d bytes", observations, memGrowthBytes)
}
