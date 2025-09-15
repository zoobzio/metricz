package benchmarks

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

// Stress test keys.
const (
	StressCounterKey metricz.Key = "stress_counter"
	StressGaugeKey   metricz.Key = "stress_gauge"
	StressHistKey    metricz.Key = "stress_hist"
	StressTimerKey   metricz.Key = "stress_timer"
)

// BenchmarkStress_ExtremeConcurrency tests performance under extreme concurrency.
func BenchmarkStress_ExtremeConcurrency(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(StressCounterKey)

	// Test with varying levels of concurrency.
	concurrencyLevels := []int{10, 50, 100, 500}

	for _, goroutines := range concurrencyLevels {
		b.Run(fmt.Sprintf("goroutines_%d", goroutines), func(b *testing.B) {
			var wg sync.WaitGroup
			iterations := b.N / goroutines

			b.ResetTimer()

			for g := 0; g < goroutines; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < iterations; i++ {
						counter.Inc()
					}
				}()
			}

			wg.Wait()
		})
	}
}

// BenchmarkStress_HighCardinalityKeys tests performance with many unique metrics.
func BenchmarkStress_HighCardinalityKeys(b *testing.B) {
	registry := metricz.New()

	// Pre-create high cardinality metrics (simulate production scenarios).
	const numKeys = 10000
	keys := make([]metricz.Key, numKeys)
	counters := make([]metricz.Counter, numKeys)

	for i := range keys {
		keys[i] = metricz.Key(fmt.Sprintf("high_cardinality_counter_%d", i))
		counters[i] = registry.Counter(keys[i])
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Random access to high cardinality metrics.
			counter := counters[rand.Intn(len(counters))]
			counter.Inc()
		}
	})
}

// BenchmarkStress_LargeBucketHistograms tests histograms with many buckets.
func BenchmarkStress_LargeBucketHistograms(b *testing.B) {
	registry := metricz.New()

	// Create histogram with many buckets (edge case).
	largeBuckets := make([]float64, 50) // Maximum allowed by library.
	for i := range largeBuckets {
		largeBuckets[i] = float64(i) * 0.1
	}

	histogram := registry.Histogram(StressHistKey, largeBuckets)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Observations that hit different buckets.
			value := rand.Float64() * 5.0
			histogram.Observe(value)
		}
	})
}

// BenchmarkStress_MixedOperationsHighLoad tests realistic high-load scenarios.
func BenchmarkStress_MixedOperationsHighLoad(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.01, 0.1, 1.0, 10.0}

	// Create multiple metrics for load distribution.
	const numMetrics = 100
	counters := make([]metricz.Counter, numMetrics)
	gauges := make([]metricz.Gauge, numMetrics)
	histograms := make([]metricz.Histogram, numMetrics/2)
	timers := make([]metricz.Timer, numMetrics/2)

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

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			operation := rand.Float64()

			if operation < 0.4 { //nolint:gocritic // ifElseChain: probability-based selection clearer as if-else.
				// 40% counter operations.
				counter := counters[rand.Intn(len(counters))]
				counter.Inc()
			} else if operation < 0.7 {
				// 30% gauge operations.
				gauge := gauges[rand.Intn(len(gauges))]
				gauge.Set(rand.Float64() * 100)
			} else if operation < 0.9 {
				// 20% histogram operations.
				histogram := histograms[rand.Intn(len(histograms))]
				histogram.Observe(rand.Float64() * 10)
			} else {
				// 10% timer operations.
				timer := timers[rand.Intn(len(timers))]
				stopwatch := timer.Start()
				stopwatch.Stop()
			}
		}
	})
}

// BenchmarkStress_ContinuousExportLoad tests export operations under continuous load.
func BenchmarkStress_ContinuousExportLoad(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 1.0, 10.0}

	// Create metrics for export testing.
	const numCounters = 500
	const numHistograms = 100

	counters := make([]metricz.Counter, numCounters)
	histograms := make([]metricz.Histogram, numHistograms)

	for i := range counters {
		counters[i] = registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i)))
	}
	for i := range histograms {
		histograms[i] = registry.Histogram(metricz.Key(fmt.Sprintf("hist_%d", i)), buckets)
	}

	b.ResetTimer()

	var exportOps int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			operation := rand.Float64()

			if operation < 0.7 {
				// 70% metric updates.
				if rand.Float64() < 0.8 {
					counter := counters[rand.Intn(len(counters))]
					counter.Inc()
				} else {
					histogram := histograms[rand.Intn(len(histograms))]
					histogram.Observe(rand.Float64() * 10)
				}
			} else {
				// 30% export operations (high frequency to stress locks).
				switch rand.Intn(3) {
				case 0:
					_ = registry.GetCounters()
				case 1:
					_ = registry.GetHistograms()
				case 2:
					// Mixed export.
					_ = registry.GetCounters()
					_ = registry.GetHistograms()
				}
				exportOps++
			}
		}
	})

	b.Logf("Performed %d export operations", exportOps)
}

// BenchmarkStress_MemoryExhaustion tests behavior under low memory conditions.
func BenchmarkStress_MemoryExhaustion(b *testing.B) {
	// Force GC to start clean.
	runtime.GC()
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	registry := metricz.New()
	buckets := []float64{0.1, 1.0, 10.0}

	// Allocate significant memory to simulate pressure.
	availableMemory := memStatsBefore.Sys
	ballastSize := int(availableMemory / 2) //nolint:gosec // Safe: memory stats conversion for benchmark.
	ballast := make([]byte, ballastSize)
	defer func() { ballast = nil; runtime.KeepAlive(ballast) }()

	// Create metrics under memory pressure.
	counters := make([]metricz.Counter, 1000)
	histograms := make([]metricz.Histogram, 200)

	for i := range counters {
		counters[i] = registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i)))
	}
	for i := range histograms {
		histograms[i] = registry.Histogram(metricz.Key(fmt.Sprintf("hist_%d", i)), buckets)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if rand.Float64() < 0.8 {
				counter := counters[rand.Intn(len(counters))]
				counter.Inc()
			} else {
				histogram := histograms[rand.Intn(len(histograms))]
				histogram.Observe(rand.Float64() * 10)
			}
		}
	})

	runtime.GC()
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	b.Logf("Memory growth during stress test: %d bytes",
		memStatsAfter.TotalAlloc-memStatsBefore.TotalAlloc)
}

// BenchmarkStress_GoroutineLeakDetection tests for goroutine leaks during operations.
func BenchmarkStress_GoroutineLeakDetection(b *testing.B) {
	initialGoroutines := runtime.NumGoroutine()

	registry := metricz.New()
	timer := registry.Timer(StressTimerKey)

	b.ResetTimer()

	// Perform many timer operations that could potentially leak goroutines.
	for i := 0; i < b.N; i++ {
		stopwatch := timer.Start()
		stopwatch.Stop()
	}

	b.StopTimer()

	// Allow time for any goroutines to clean up.
	time.Sleep(10 * time.Millisecond)
	runtime.GC()

	finalGoroutines := runtime.NumGoroutine()
	goroutineLeak := finalGoroutines - initialGoroutines

	b.Logf("Goroutines at start: %d, at end: %d, leak: %d",
		initialGoroutines, finalGoroutines, goroutineLeak)

	if goroutineLeak > 5 { // Allow some tolerance.
		b.Errorf("Potential goroutine leak detected: %d extra goroutines", goroutineLeak)
	}
}

// BenchmarkStress_RapidRegistryCreation tests rapid registry creation and destruction.
func BenchmarkStress_RapidRegistryCreation(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		registry := metricz.New()

		// Create a few metrics.
		counter := registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i%100)))
		gauge := registry.Gauge(metricz.Key(fmt.Sprintf("gauge_%d", i%100)))

		// Use them briefly.
		counter.Inc()
		gauge.Set(float64(i))

		// Registry goes out of scope and should be GC'd.
		_ = registry
	}
}

// BenchmarkStress_LongRunningStability simulates long-running service stability.
func BenchmarkStress_LongRunningStability(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping long-running stability test in short mode")
	}

	registry := metricz.New()
	buckets := []float64{0.01, 0.1, 1.0, 10.0}

	// Simulate production metrics.
	requests := registry.Counter(metricz.Key("requests_total"))
	latency := registry.Histogram(metricz.Key("request_duration"), buckets)
	errors := registry.Counter(metricz.Key("errors_total"))
	active := registry.Gauge(metricz.Key("active_requests"))

	b.ResetTimer()

	// Track metrics over time.
	var totalRequests, totalErrors int64
	startTime := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate request processing.
			requests.Inc()
			active.Inc()
			totalRequests++

			// Simulate request duration.
			duration := rand.Float64() * 2.0
			latency.Observe(duration)

			// Simulate errors (2% error rate).
			if rand.Float64() < 0.02 {
				errors.Inc()
				totalErrors++
			}

			active.Dec()

			// Occasional exports (simulate monitoring).
			if rand.Float64() < 0.001 { // 0.1% of operations.
				_ = registry.GetCounters()
				_ = registry.GetHistograms()
			}
		}
	})

	duration := time.Since(startTime)
	requestRate := float64(totalRequests) / duration.Seconds()
	errorRate := float64(totalErrors) / float64(totalRequests) * 100

	b.Logf("Long-running test: %.0f req/sec, %.2f%% errors over %v",
		requestRate, errorRate, duration)
}

// BenchmarkStress_AtomicContentionBreakingPoint finds the breaking point for atomic operations.
func BenchmarkStress_AtomicContentionBreakingPoint(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(StressCounterKey)

	// Test different contention levels to find breaking point.
	contentionLevels := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512}

	for _, goroutines := range contentionLevels {
		b.Run(fmt.Sprintf("contention_%d", goroutines), func(b *testing.B) {
			var wg sync.WaitGroup
			iterations := b.N / goroutines

			if iterations == 0 {
				iterations = 1
			}

			start := time.Now()

			for g := 0; g < goroutines; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < iterations; i++ {
						counter.Inc()
					}
				}()
			}

			wg.Wait()

			elapsed := time.Since(start)
			opsPerSec := float64(b.N) / elapsed.Seconds()

			b.Logf("Goroutines: %d, Ops/sec: %.0f, Efficiency: %.2f%%",
				goroutines, opsPerSec, opsPerSec/float64(goroutines)/1000000*100)
		})
	}
}
