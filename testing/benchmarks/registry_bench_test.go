package benchmarks

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/zoobzio/metricz"
)

// Registry benchmark keys.
const (
	RegBenchCounterKey metricz.Key = "reg_bench_counter"
	RegBenchGaugeKey   metricz.Key = "reg_bench_gauge"
	RegBenchHistKey    metricz.Key = "reg_bench_hist"
	RegBenchTimerKey   metricz.Key = "reg_bench_timer"
)

// BenchmarkRegistry_Counter_Creation tests registry counter creation performance.
func BenchmarkRegistry_Counter_Creation(b *testing.B) {
	registry := metricz.New()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		_ = registry.Counter(key)
	}
}

// BenchmarkRegistry_Counter_Retrieval tests registry counter lookup performance.
func BenchmarkRegistry_Counter_Retrieval(b *testing.B) {
	registry := metricz.New()

	// Pre-create counter.
	counter := registry.Counter(RegBenchCounterKey)
	_ = counter

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = registry.Counter(RegBenchCounterKey)
	}
}

// BenchmarkRegistry_Mixed_Operations tests mixed registry operations.
func BenchmarkRegistry_Mixed_Operations(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		switch i % 4 {
		case 0:
			key := metricz.Key(fmt.Sprintf("counter_%d", i%100))
			registry.Counter(key).Inc()
		case 1:
			key := metricz.Key(fmt.Sprintf("gauge_%d", i%100))
			registry.Gauge(key).Set(float64(i))
		case 2:
			key := metricz.Key(fmt.Sprintf("hist_%d", i%100))
			registry.Histogram(key, buckets).Observe(rand.Float64() * 10)
		case 3:
			key := metricz.Key(fmt.Sprintf("timer_%d", i%100))
			stopwatch := registry.Timer(key).Start()
			stopwatch.Stop()
		}
	}
}

// BenchmarkRegistry_GetCounters tests export operations.
func BenchmarkRegistry_GetCounters(b *testing.B) {
	registry := metricz.New()

	// Pre-populate with counters.
	for i := 0; i < 100; i++ {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		counter := registry.Counter(key)
		counter.Add(float64(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = registry.GetCounters()
	}
}

// BenchmarkRegistry_GetGauges tests gauge export operations.
func BenchmarkRegistry_GetGauges(b *testing.B) {
	registry := metricz.New()

	// Pre-populate with gauges.
	for i := 0; i < 100; i++ {
		key := metricz.Key(fmt.Sprintf("gauge_%d", i))
		gauge := registry.Gauge(key)
		gauge.Set(float64(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = registry.GetGauges()
	}
}

// BenchmarkRegistry_GetHistograms tests histogram export operations.
func BenchmarkRegistry_GetHistograms(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}

	// Pre-populate with histograms.
	for i := 0; i < 50; i++ { // Fewer histograms due to complexity.
		key := metricz.Key(fmt.Sprintf("hist_%d", i))
		histogram := registry.Histogram(key, buckets)
		for j := 0; j < 100; j++ {
			histogram.Observe(rand.Float64() * 10)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = registry.GetHistograms()
	}
}

// BenchmarkRegistry_GetTimers tests timer export operations.
func BenchmarkRegistry_GetTimers(b *testing.B) {
	registry := metricz.New()

	// Pre-populate with timers.
	for i := 0; i < 50; i++ {
		key := metricz.Key(fmt.Sprintf("timer_%d", i))
		timer := registry.Timer(key)
		for j := 0; j < 100; j++ {
			stopwatch := timer.Start()
			stopwatch.Stop()
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = registry.GetTimers()
	}
}

// BenchmarkRegistry_Export_Under_Load tests registry mutex contention.
func BenchmarkRegistry_Export_Under_Load(b *testing.B) {
	registry := metricz.New()

	// Pre-populate with metrics.
	counters := make([]metricz.Counter, 100)
	for i := range counters {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		counters[i] = registry.Counter(key)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if rand.Float64() < 0.1 {
				// 10% export operations (requires read lock).
				_ = registry.GetCounters()
			} else {
				// 90% metric updates (lock-free after creation).
				counter := counters[rand.Intn(len(counters))]
				counter.Inc()
			}
		}
	})
}

// BenchmarkRegistry_Creation_Under_Load tests registry write lock contention.
func BenchmarkRegistry_Creation_Under_Load(b *testing.B) {
	registry := metricz.New()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Create new metrics concurrently (requires write lock).
			key := metricz.Key(fmt.Sprintf("counter_%d", rand.Intn(1000)))
			registry.Counter(key).Inc()
		}
	})
}

// BenchmarkRegistry_Mixed_Load tests realistic mixed workload.
func BenchmarkRegistry_Mixed_Load(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}

	// Pre-create some metrics.
	counters := make([]metricz.Counter, 50)
	gauges := make([]metricz.Gauge, 25)
	histograms := make([]metricz.Histogram, 10)
	timers := make([]metricz.Timer, 10)

	for i := range counters {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		counters[i] = registry.Counter(key)
	}
	for i := range gauges {
		key := metricz.Key(fmt.Sprintf("gauge_%d", i))
		gauges[i] = registry.Gauge(key)
	}
	for i := range histograms {
		key := metricz.Key(fmt.Sprintf("hist_%d", i))
		histograms[i] = registry.Histogram(key, buckets)
	}
	for i := range timers {
		key := metricz.Key(fmt.Sprintf("timer_%d", i))
		timers[i] = registry.Timer(key)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			operation := rand.Float64()

			if operation < 0.5 { //nolint:gocritic // ifElseChain: probability-based selection clearer as if-else.
				// 50% counter operations.
				counter := counters[rand.Intn(len(counters))]
				counter.Inc()
			} else if operation < 0.7 {
				// 20% gauge operations.
				gauge := gauges[rand.Intn(len(gauges))]
				gauge.Set(rand.Float64() * 100)
			} else if operation < 0.85 {
				// 15% histogram operations.
				histogram := histograms[rand.Intn(len(histograms))]
				histogram.Observe(rand.Float64() * 10)
			} else if operation < 0.95 {
				// 10% timer operations.
				timer := timers[rand.Intn(len(timers))]
				stopwatch := timer.Start()
				stopwatch.Stop()
			} else {
				// 5% export operations.
				switch rand.Intn(4) {
				case 0:
					_ = registry.GetCounters()
				case 1:
					_ = registry.GetGauges()
				case 2:
					_ = registry.GetHistograms()
				case 3:
					_ = registry.GetTimers()
				}
			}
		}
	})
}

// BenchmarkRegistry_Reset tests registry reset performance.
func BenchmarkRegistry_Reset(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Setup registry with metrics.
		registry := metricz.New()
		buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}

		for j := 0; j < 100; j++ {
			registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", j))).Inc()
			registry.Gauge(metricz.Key(fmt.Sprintf("gauge_%d", j))).Set(float64(j))
			registry.Histogram(metricz.Key(fmt.Sprintf("hist_%d", j)), buckets).Observe(float64(j))
			registry.Timer(metricz.Key(fmt.Sprintf("timer_%d", j))).Start().Stop()
		}

		b.StartTimer()

		// Time the reset operation.
		registry.Reset()
	}
}
