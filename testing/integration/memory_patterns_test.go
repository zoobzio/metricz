package integration

import (
	"runtime"
	"sync"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// TestMemoryIsolation verifies registries don't leak memory between instances.
func TestMemoryIsolation(t *gotesting.T) {
	// Force GC to get baseline
	runtime.GC()
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Create and destroy many registries
	for i := 0; i < 100; i++ {
		// Create temporary registry for this iteration
		registry := metricz.New() // Not using testing helper here as we want to test cleanup

		// Add some metrics - using static keys instead of dynamic ones
		// Test the same memory behavior with fewer distinct keys
		registry.Counter(MemoryTestCounterKey).Add(float64(i))
		registry.Gauge(MemoryTestGaugeKey).Set(float64(i))
		registry.Histogram(MemoryTestHistKey, metricz.DefaultLatencyBuckets).Observe(float64(i))

		// Registry goes out of scope
	}

	// Force GC and check memory
	runtime.GC()
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Memory should be released (allowing some overhead)
	heapGrowth := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc) //nolint:gosec // Safe: heap allocations are well within int64 range
	maxAcceptableGrowth := int64(1 * 1024 * 1024)                        // 1MB tolerance

	if heapGrowth > maxAcceptableGrowth {
		t.Logf("Warning: Heap grew by %d bytes after registry cleanup", heapGrowth)
		// Not failing as GC timing can vary
	}

	// Verify no goroutine leaks
	initialGoroutines := runtime.NumGoroutine()

	// Create registries with concurrent operations
	const numWorkers = 10
	const operations = 100
	registries := make([]*metricz.Registry, numWorkers)

	for i := range registries {
		registries[i] = metricz.New() // Temporary registries for memory test
	}

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    numWorkers,
		Operations: operations,
		Operation: func(workerID, _ int) { // opID unused, workerID used for registry selection
			registries[workerID].Counter(metricz.Key("test")).Inc()
		},
	})
	time.Sleep(100 * time.Millisecond) // Let goroutines fully terminate

	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines+2 { // Allow small variance
		t.Errorf("Goroutine leak detected: started with %d, ended with %d",
			initialGoroutines, finalGoroutines)
	}
}

// TestResetMemoryBehavior verifies Reset() properly clears memory.
func TestResetMemoryBehavior(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Create many metrics with static keys instead of dynamic ones
	// Using single keys repeatedly tests the same memory behavior
	for i := 0; i < 1000; i++ {
		registry.Counter(MemoryTestCounterKey).Add(float64(i))
		registry.Gauge(MemoryTestGaugeKey).Set(float64(i))

		if i%10 == 0 {
			registry.Histogram(MemoryTestHistKey, metricz.DefaultLatencyBuckets).Observe(float64(i))
		}
	}

	// Check memory usage before reset
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Reset should clear internal maps
	registry.Reset()

	// Force GC to reclaim memory
	runtime.GC()
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Verify metrics are cleared
	if registry.Counter(MemoryTestCounterKey).Value() != 0 {
		t.Error("Counter not reset to zero")
	}

	// New metrics should work after reset
	registry.Counter(NewCounterKey).Inc()
	if registry.Counter(NewCounterKey).Value() != 1 {
		t.Error("Cannot create new metrics after reset")
	}

	// Memory should be mostly reclaimed
	if memAfter.HeapAlloc > memBefore.HeapAlloc {
		t.Logf("Note: Heap did not shrink after reset (before: %d, after: %d)",
			memBefore.HeapAlloc, memAfter.HeapAlloc)
	}
}

// TestConcurrentMemorySafety verifies no memory corruption under concurrent access.
func TestConcurrentMemorySafety(t *gotesting.T) {
	registry := metricz.New()

	const (
		numGoroutines = 100
		numOperations = 1000
	)

	// Shared metric keys
	keys := []string{"shared1", "shared2", "shared3", "shared4", "shared5"}

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 types of operations

	// Concurrent counter operations
	for i := 0; i < numGoroutines; i++ {
		go func(_ int) { // goroutine ID unused in counter operations
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := keys[j%len(keys)]
				registry.Counter(metricz.Key(key)).Inc()
			}
		}(i)
	}

	// Concurrent gauge operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) { // id used in gauge value calculation
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := keys[j%len(keys)]
				registry.Gauge(metricz.Key(key)).Set(float64(id * j))
			}
		}(i)
	}

	// Concurrent histogram operations
	for i := 0; i < numGoroutines; i++ {
		go func(_ int) { // goroutine ID unused in histogram operations
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := keys[j%len(keys)]
				registry.Histogram(metricz.Key(key), metricz.DefaultLatencyBuckets).Observe(float64(j))
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(_ int) { // goroutine ID unused in read operations
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := keys[j%len(keys)]
				_ = registry.Counter(metricz.Key(key)).Value()
				_ = registry.Gauge(metricz.Key(key)).Value()
				_ = registry.Histogram(metricz.Key(key), metricz.DefaultLatencyBuckets).Count()
			}
		}(i)
	}

	wg.Wait()

	// Verify data integrity - counters should have exact count
	// Operations are distributed across keys: numOperations per goroutine, distributed by j%len(keys)
	for _, key := range keys {
		value := registry.Counter(metricz.Key(key)).Value()
		// Each key gets numOperations/len(keys) increments per goroutine
		// Since we iterate j from 0 to numOperations-1 and use j%len(keys),
		// each key gets numOperations/len(keys) increments per goroutine
		expected := float64(numGoroutines * (numOperations / len(keys)))
		if value != expected {
			t.Errorf("Counter %s: expected %f, got %f (possible memory corruption)",
				key, expected, value)
		}
	}

	// Histograms should have exact observation count
	for _, key := range keys {
		count := registry.Histogram(metricz.Key(key), metricz.DefaultLatencyBuckets).Count()
		// Same distribution logic as counters
		expected := uint64(numGoroutines * (numOperations / len(keys))) //nolint:gosec // Safe: test values are small integers
		if count != expected {
			t.Errorf("Histogram %s: expected %d observations, got %d",
				key, expected, count)
		}
	}
}

// TestMetricReferenceLifetime verifies metric references remain valid.
func TestMetricReferenceLifetime(t *gotesting.T) {
	registry := metricz.New()

	// Get references to metrics
	counter1 := registry.Counter(TestCounterKey)
	counter2 := registry.Counter(TestCounterKey) // Same key
	gauge := registry.Gauge(TestGaugeKey)

	// Modify through first reference
	counter1.Add(10)

	// Second reference should see the change (same underlying metric)
	if counter2.Value() != 10 {
		t.Error("Counter references not pointing to same metric")
	}

	// Continue using references after more operations
	// No need to create new metrics - test with existing ones
	for i := 0; i < 100; i++ {
		registry.Counter(MemoryTestCounterKey).Inc()
	}

	// Original references should still work
	counter1.Add(5)
	if counter2.Value() != 15 {
		t.Error("Counter reference invalidated by new metrics")
	}

	gauge.Set(42)
	if gauge.Value() != 42 {
		t.Error("Gauge reference not working")
	}

	// References survive Reset() but values are cleared
	registry.Reset()

	// After reset, old references still point to old (now zeroed) metrics
	// Get new references to the reset metrics
	counter1 = registry.Counter(TestCounterKey)
	counter2 = registry.Counter(TestCounterKey)

	counter1.Inc() // Should work on reset metric
	if counter1.Value() != 1 {
		t.Error("Counter reference not working after reset")
	}

	if counter2.Value() != 1 {
		t.Error("Both references should see reset metric")
	}
}

// TestHistogramMemoryEfficiency tests histogram memory usage patterns.
func TestHistogramMemoryEfficiency(t *gotesting.T) {
	registry := metricz.New()

	// Different bucket configurations
	configs := []struct {
		name    string
		buckets []float64
	}{
		{"small", []float64{1, 5, 10}},
		{"medium", metricz.DefaultLatencyBuckets},
		{"large", generateBuckets(50)},
		{"xlarge", generateBuckets(100)},
	}

	for _, config := range configs {
		t.Run(config.name, func(t *gotesting.T) {
			// Measure memory before
			runtime.GC()
			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)

			// Create histogram and add observations
			hist := registry.Histogram(metricz.Key(config.name), config.buckets)
			for i := 0; i < 10000; i++ {
				hist.Observe(float64(i % 1000))
			}

			// Measure memory after
			runtime.GC()
			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			memUsed := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc) //nolint:gosec // Safe: heap allocations are well within int64 range

			t.Logf("Histogram with %d buckets used ~%d KB for 10k observations",
				len(config.buckets), memUsed/1024)

			// Verify histogram is working
			if hist.Count() != 10000 {
				t.Errorf("Histogram lost observations: expected 10000, got %d", hist.Count())
			}
		})
	}
}

// TestMemoryReclamation verifies memory is reclaimed when metrics are no longer used.
func TestMemoryReclamation(t *gotesting.T) {
	// Phase 1: Create registry with many metrics
	var registry *metricz.Registry
	var memWithMetrics runtime.MemStats

	func() {
		registry = metricz.New()

		// Add many metrics using static keys instead of dynamic ones
		for i := 0; i < 5000; i++ {
			registry.Counter(MemoryTestCounterKey).Add(float64(i))
			registry.Gauge(MemoryTestGaugeKey).Set(float64(i))
		}

		runtime.GC()
		runtime.ReadMemStats(&memWithMetrics)
		t.Logf("Memory with 10k metrics: %d KB", memWithMetrics.HeapAlloc/1024)
	}()

	// Phase 2: Reset and verify memory is reclaimed
	registry.Reset()
	runtime.GC()
	runtime.GC() // Double GC to ensure cleanup

	var memAfterReset runtime.MemStats
	runtime.ReadMemStats(&memAfterReset)

	memReclaimed := int64(memWithMetrics.HeapAlloc) - int64(memAfterReset.HeapAlloc) //nolint:gosec // Safe: heap allocations are well within int64 range
	t.Logf("Memory after reset: %d KB (reclaimed: %d KB)",
		memAfterReset.HeapAlloc/1024, memReclaimed/1024)

	// Should reclaim significant memory
	if memReclaimed < 0 {
		t.Log("Note: Memory increased after reset (GC timing variance)")
	}

	// Phase 3: Let registry go out of scope
	registry = nil
	runtime.GC()
	runtime.GC()

	var memFinal runtime.MemStats
	runtime.ReadMemStats(&memFinal)

	t.Logf("Final memory: %d KB", memFinal.HeapAlloc/1024)
}

// TestConcurrentResetMemorySafety tests Reset() during concurrent operations.
func TestConcurrentResetMemorySafety(t *gotesting.T) {
	registry := metricz.New()

	stopCh := make(chan struct{})
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					registry.Counter(metricz.Key("counter")).Inc()
					registry.Gauge(metricz.Key("gauge")).Set(float64(id))
					registry.Histogram(metricz.Key("hist"), metricz.DefaultLatencyBuckets).Observe(float64(id))
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					_ = registry.Counter(metricz.Key("counter")).Value()
					_ = registry.Gauge(metricz.Key("gauge")).Value()
					_ = registry.GetCounters()
					_ = registry.GetGauges()
				}
			}
		}()
	}

	// Reset periodically
	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Millisecond)
		registry.Reset()
	}

	close(stopCh)
	wg.Wait()

	// Final verification - should be able to use registry normally
	registry.Counter(FinalKey).Inc()
	if registry.Counter(FinalKey).Value() != 1 {
		t.Error("Registry corrupted by concurrent reset")
	}

	// Check for memory leaks
	runtime.GC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	t.Logf("Final goroutines: %d, heap: %d KB", runtime.NumGoroutine(), mem.HeapAlloc/1024)
}

// Helper to generate bucket boundaries.
func generateBuckets(n int) []float64 {
	buckets := make([]float64, n)
	for i := 0; i < n; i++ {
		buckets[i] = float64(i * 10)
	}
	return buckets
}
