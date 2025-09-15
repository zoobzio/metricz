package benchmarks

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/zoobzio/metricz"
)

// Benchmark metric keys - pre-defined, compile-time safe.
const (
	BenchCounterKey metricz.Key = "bench_counter"
	BenchGaugeKey   metricz.Key = "bench_gauge"
	BenchHistKey    metricz.Key = "bench_histogram"
	BenchTimerKey   metricz.Key = "bench_timer"
)

// Standard histogram buckets for realistic testing.
var standardBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

// BenchmarkCounter_Inc tests sequential counter increments.
func BenchmarkCounter_Inc(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		counter.Inc()
	}
}

// BenchmarkCounter_Inc_Parallel tests counter increments under contention.
func BenchmarkCounter_Inc_Parallel(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Inc()
		}
	})
}

// BenchmarkCounter_Add tests sequential counter additions.
func BenchmarkCounter_Add(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		counter.Add(1.5)
	}
}

// BenchmarkCounter_Add_Parallel tests counter additions under contention.
func BenchmarkCounter_Add_Parallel(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1.5)
		}
	})
}

// BenchmarkCounter_Value tests sequential counter reads.
func BenchmarkCounter_Value(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)
	counter.Add(1000) // Pre-populate.

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = counter.Value()
	}
}

// BenchmarkCounter_Value_Parallel tests counter reads under contention.
func BenchmarkCounter_Value_Parallel(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)
	counter.Add(1000) // Pre-populate.

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = counter.Value()
		}
	})
}

// BenchmarkGauge_Set tests sequential gauge sets.
func BenchmarkGauge_Set(b *testing.B) {
	registry := metricz.New()
	gauge := registry.Gauge(BenchGaugeKey)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gauge.Set(float64(i))
	}
}

// BenchmarkGauge_Set_Parallel tests gauge sets under contention.
func BenchmarkGauge_Set_Parallel(b *testing.B) {
	registry := metricz.New()
	gauge := registry.Gauge(BenchGaugeKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gauge.Set(rand.Float64() * 100)
		}
	})
}

// BenchmarkGauge_Add tests sequential gauge additions.
func BenchmarkGauge_Add(b *testing.B) {
	registry := metricz.New()
	gauge := registry.Gauge(BenchGaugeKey)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gauge.Add(1.0)
	}
}

// BenchmarkGauge_Add_Parallel tests gauge additions under contention.
func BenchmarkGauge_Add_Parallel(b *testing.B) {
	registry := metricz.New()
	gauge := registry.Gauge(BenchGaugeKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gauge.Add(1.0)
		}
	})
}

// BenchmarkGauge_Value tests sequential gauge reads.
func BenchmarkGauge_Value(b *testing.B) {
	registry := metricz.New()
	gauge := registry.Gauge(BenchGaugeKey)
	gauge.Set(42.0) // Pre-populate.

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = gauge.Value()
	}
}

// BenchmarkHistogram_Observe tests sequential histogram observations.
func BenchmarkHistogram_Observe(b *testing.B) {
	registry := metricz.New()
	histogram := registry.Histogram(BenchHistKey, standardBuckets)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		histogram.Observe(rand.Float64() * 10)
	}
}

// BenchmarkHistogram_Observe_Parallel tests histogram observations under contention.
func BenchmarkHistogram_Observe_Parallel(b *testing.B) {
	registry := metricz.New()
	histogram := registry.Histogram(BenchHistKey, standardBuckets)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			histogram.Observe(rand.Float64() * 10)
		}
	})
}

// BenchmarkHistogram_Sum tests sequential histogram sum reads.
func BenchmarkHistogram_Sum(b *testing.B) {
	registry := metricz.New()
	histogram := registry.Histogram(BenchHistKey, standardBuckets)

	// Pre-populate with observations.
	for i := 0; i < 1000; i++ {
		histogram.Observe(rand.Float64() * 10)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = histogram.Sum()
	}
}

// BenchmarkHistogram_Count tests sequential histogram count reads.
func BenchmarkHistogram_Count(b *testing.B) {
	registry := metricz.New()
	histogram := registry.Histogram(BenchHistKey, standardBuckets)

	// Pre-populate with observations.
	for i := 0; i < 1000; i++ {
		histogram.Observe(rand.Float64() * 10)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = histogram.Count()
	}
}

// BenchmarkTimer_Record tests sequential timer recordings.
func BenchmarkTimer_Record(b *testing.B) {
	registry := metricz.New()
	timer := registry.Timer(BenchTimerKey)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stopwatch := timer.Start()
		stopwatch.Stop()
	}
}

// BenchmarkTimer_Record_Parallel tests timer recordings under contention.
func BenchmarkTimer_Record_Parallel(b *testing.B) {
	registry := metricz.New()
	timer := registry.Timer(BenchTimerKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stopwatch := timer.Start()
			stopwatch.Stop()
		}
	})
}

// BenchmarkAtomicFloat64_CAS_Contention tests the atomic CAS loop under extreme contention.
func BenchmarkAtomicFloat64_CAS_Contention(b *testing.B) {
	registry := metricz.New()
	counter := registry.Counter(BenchCounterKey)

	b.ResetTimer()

	// Run with high contention (multiple goroutines hammering same counter).
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(rand.Float64())
		}
	})
}

// BenchmarkMultipleCounters_Parallel tests contention across multiple counters.
func BenchmarkMultipleCounters_Parallel(b *testing.B) {
	registry := metricz.New()

	// Create multiple counters to spread contention.
	counters := make([]metricz.Counter, 10)
	for i := range counters {
		key := metricz.Key(fmt.Sprintf("counter_%d", i))
		counters[i] = registry.Counter(key)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Random counter selection to distribute load.
			counter := counters[rand.Intn(len(counters))]
			counter.Inc()
		}
	})
}
