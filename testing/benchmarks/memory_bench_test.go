package benchmarks

import (
	"fmt"
	"math/rand"
	"runtime"
	"testing"

	"github.com/zoobzio/metricz"
)

// Memory benchmark keys.
const (
	MemBenchCounterKey metricz.Key = "mem_bench_counter"
	MemBenchGaugeKey   metricz.Key = "mem_bench_gauge"
	MemBenchHistKey    metricz.Key = "mem_bench_hist"
	MemBenchTimerKey   metricz.Key = "mem_bench_timer"
)

// BenchmarkMemoryPressure_Counters tests counter performance under memory pressure.
func BenchmarkMemoryPressure_Counters(b *testing.B) {
	// Force GC and get baseline.
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	registry := metricz.New()

	// Create many counters to pressure memory.
	counters := make([]metricz.Counter, 1000)
	for i := range counters {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		counters[i] = registry.Counter(key)
	}

	// Allocate memory ballast to create pressure (50% of available memory).
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	ballastSize := int(memStats.Sys / 2) //nolint:gosec // Safe: memory stats conversion for benchmark.
	ballast := make([]byte, ballastSize)
	defer func() { ballast = nil; runtime.KeepAlive(ballast) }()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter := counters[rand.Intn(len(counters))]
			counter.Inc()
		}
	})

	b.StopTimer()
	runtime.ReadMemStats(&m2)
	b.Logf("Memory allocated: %d bytes", m2.TotalAlloc-m1.TotalAlloc)
}

// BenchmarkMemoryPressure_Histograms tests histogram performance under memory pressure.
func BenchmarkMemoryPressure_Histograms(b *testing.B) {
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	registry := metricz.New()
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0, 50.0, 100.0}

	// Create many histograms (more memory intensive).
	histograms := make([]metricz.Histogram, 500)
	for i := range histograms {
		key := metricz.Key(fmt.Sprintf("histogram_%d", i))
		histograms[i] = registry.Histogram(key, buckets)
	}

	// Memory pressure.
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	ballastSize := int(memStats.Sys / 3) //nolint:gosec // Safe: memory stats conversion for benchmark.
	ballast := make([]byte, ballastSize)
	defer func() { ballast = nil; runtime.KeepAlive(ballast) }()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			histogram := histograms[rand.Intn(len(histograms))]
			histogram.Observe(rand.Float64() * 100)
		}
	})

	b.StopTimer()
	runtime.ReadMemStats(&m2)
	b.Logf("Memory allocated: %d bytes", m2.TotalAlloc-m1.TotalAlloc)
}

// BenchmarkMemoryPressure_MixedLoad tests mixed operations under memory pressure.
func BenchmarkMemoryPressure_MixedLoad(b *testing.B) {
	runtime.GC()

	registry := metricz.New()
	buckets := []float64{0.1, 1.0, 10.0}

	// Create mixed metrics.
	counters := make([]metricz.Counter, 200)
	gauges := make([]metricz.Gauge, 200)
	histograms := make([]metricz.Histogram, 100)
	timers := make([]metricz.Timer, 100)

	for i := range counters {
		counters[i] = registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i)))
	}
	for i := range gauges {
		gauges[i] = registry.Gauge(metricz.Key(fmt.Sprintf("gauge_%d", i)))
	}
	for i := range histograms {
		histograms[i] = registry.Histogram(metricz.Key(fmt.Sprintf("hist_%d", i)), buckets)
	}
	for i := range timers {
		timers[i] = registry.Timer(metricz.Key(fmt.Sprintf("timer_%d", i)))
	}

	// Create memory pressure.
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	ballastSize := int(memStats.Sys / 4) //nolint:gosec // Safe: memory stats conversion for benchmark.
	ballast := make([]byte, ballastSize)
	defer func() { ballast = nil; runtime.KeepAlive(ballast) }()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			switch rand.Intn(4) {
			case 0:
				counters[rand.Intn(len(counters))].Inc()
			case 1:
				gauges[rand.Intn(len(gauges))].Set(rand.Float64() * 100)
			case 2:
				histograms[rand.Intn(len(histograms))].Observe(rand.Float64() * 10)
			case 3:
				timer := timers[rand.Intn(len(timers))]
				stopwatch := timer.Start()
				stopwatch.Stop()
			}
		}
	})
}

// BenchmarkAllocation_Counter_Updates verifies zero-allocation claims for counters.
func BenchmarkAllocation_Counter_Updates(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(MemBenchCounterKey) // Pre-create.

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		counter.Inc() // Should be 0 allocs/op.
	}
}

// BenchmarkAllocation_Gauge_Updates verifies zero-allocation claims for gauges.
func BenchmarkAllocation_Gauge_Updates(b *testing.B) {
	registry := metricz.New()
	gauge := registry.Gauge(MemBenchGaugeKey) // Pre-create.

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gauge.Set(float64(i)) // Should be 0 allocs/op.
	}
}

// BenchmarkAllocation_Histogram_Observations verifies histogram allocation behavior.
func BenchmarkAllocation_Histogram_Observations(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 1.0, 10.0}
	histogram := registry.Histogram(MemBenchHistKey, buckets) // Pre-create.

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		histogram.Observe(float64(i % 10)) // Should be 0 allocs/op after creation.
	}
}

// BenchmarkAllocation_Timer_Operations verifies timer allocation behavior.
func BenchmarkAllocation_Timer_Operations(b *testing.B) {
	registry := metricz.New()
	timer := registry.Timer(MemBenchTimerKey) // Pre-create.

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stopwatch := timer.Start()
		stopwatch.Stop()
		// Note: Start() may allocate stopwatch struct, but should be minimal.
	}
}

// BenchmarkGC_Impact_During_Operations tests performance during GC pressure.
func BenchmarkGC_Impact_During_Operations(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 1.0, 10.0}

	// Create metrics.
	counter := registry.Counter(MemBenchCounterKey)
	histogram := registry.Histogram(MemBenchHistKey, buckets)

	// Create garbage to trigger frequent GC.
	garbage := make([][]byte, 0, 1000)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		localGarbage := make([][]byte, 0, 100)

		for pb.Next() {
			// Metric operations.
			counter.Inc()
			histogram.Observe(rand.Float64() * 10)

			// Create garbage to pressure GC.
			if rand.Float64() < 0.1 { // 10% of operations create garbage.
				garbage := make([]byte, 1024)
				localGarbage = append(localGarbage, garbage)

				// Occasionally clean up to maintain steady state.
				if len(localGarbage) > 50 {
					localGarbage = localGarbage[:0]
				}
			}
		}

		// Keep local garbage alive during benchmark.
		_ = localGarbage
	})

	// Keep garbage alive.
	_ = garbage
}

// BenchmarkLargeRegistryMemoryUsage tests memory efficiency with many metrics.
func BenchmarkLargeRegistryMemoryUsage(b *testing.B) {
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	registry := metricz.New()
	buckets := []float64{0.1, 1.0, 10.0}

	// Create large number of metrics (simulate production cardinality).
	const numMetrics = 10000

	counters := make([]metricz.Counter, numMetrics)
	for i := range counters {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		counters[i] = registry.Counter(key)
	}

	// Add some histograms (more expensive).
	histograms := make([]metricz.Histogram, numMetrics/10)
	for i := range histograms {
		key := metricz.Key(fmt.Sprintf("hist_%d", i))
		histograms[i] = registry.Histogram(key, buckets)
	}

	runtime.ReadMemStats(&m2)
	b.Logf("Registry with %d counters + %d histograms: %d bytes",
		numMetrics, len(histograms), m2.TotalAlloc-m1.TotalAlloc)

	b.ResetTimer()
	b.ReportAllocs()

	// Test operations on large registry.
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if rand.Float64() < 0.9 {
				// 90% counter operations.
				counter := counters[rand.Intn(len(counters))]
				counter.Inc()
			} else {
				// 10% histogram operations.
				histogram := histograms[rand.Intn(len(histograms))]
				histogram.Observe(rand.Float64() * 10)
			}
		}
	})
}

// BenchmarkMemoryLeakDetection tests for potential memory leaks in metric operations.
func BenchmarkMemoryLeakDetection(b *testing.B) {
	const iterations = 1000

	// Baseline memory.
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < iterations; i++ {
		registry := metricz.New()
		buckets := []float64{0.1, 1.0, 10.0}

		// Create and use metrics.
		counter := registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i)))
		histogram := registry.Histogram(metricz.Key(fmt.Sprintf("hist_%d", i)), buckets)

		// Perform operations.
		for j := 0; j < 100; j++ {
			counter.Inc()
			histogram.Observe(float64(j))
		}

		// Force cleanup.
		registry.Reset()
		registry = nil //nolint:wastedassign // Intentional nil assignment for GC.

		// Periodic GC.
		if i%100 == 0 {
			runtime.GC()
		}
	}

	// Final GC and measurement.
	runtime.GC()
	var final runtime.MemStats
	runtime.ReadMemStats(&final)

	memoryGrowth := final.TotalAlloc - baseline.TotalAlloc
	b.Logf("Memory growth after %d registry cycles: %d bytes (%.2f KB per cycle)",
		iterations, memoryGrowth, float64(memoryGrowth)/float64(iterations)/1024.0)

	// The actual benchmark - just to satisfy framework.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = i
	}
}
