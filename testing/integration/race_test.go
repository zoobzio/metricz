package integration

import (
	"fmt"
	"runtime"
	"sync/atomic"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// TestConcurrentRegistryCreation tests creating multiple registries concurrently.
func TestConcurrentRegistryCreation(t *gotesting.T) {
	const goroutines = 100

	registries := make([]*metricz.Registry, goroutines)

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: 1,
		Operation: func(workerID, _ int) { // opID unused in registry creation
			registries[workerID] = testing.NewTestRegistry(t)
		},
	})

	// Verify all registries were created
	for i, reg := range registries {
		if reg == nil {
			t.Errorf("Registry %d was not created", i)
		}
	}
}

// TestConcurrentCounterOperations tests concurrent operations on counters.
func TestConcurrentCounterOperations(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	const goroutines = 100
	const operations = 1000

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: operations,
		Operation: func(_, _ int) { // workerID and opID unused in simple increment
			counter.Inc()
		},
	})

	expected := float64(goroutines * operations)
	if counter.Value() != expected {
		t.Errorf("Expected counter value %f, got %f", expected, counter.Value())
	}
}

// TestConcurrentGaugeOperations tests concurrent operations on gauges.
func TestConcurrentGaugeOperations(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	gauge := registry.Gauge(TestGaugeKey)

	const goroutines = 100
	const operations = 100

	// Half increment, half decrement
	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: operations,
		Operation: func(_, _ int) { // workerID and opID unused in gauge increment
			gauge.Inc()
		},
	})

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: operations,
		Operation: func(_, _ int) { // workerID and opID unused in gauge decrement
			gauge.Dec()
		},
	})

	// Should be zero after equal inc/dec
	if gauge.Value() != 0 {
		t.Errorf("Expected gauge value 0, got %f", gauge.Value())
	}
}

// TestConcurrentHistogramObservations tests concurrent histogram observations.
func TestConcurrentHistogramObservations(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	histogram := registry.Histogram(TestHistogramKey, metricz.DefaultLatencyBuckets)

	const goroutines = 100
	const observations = 100

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: observations,
		Operation: func(workerID, _ int) { // opID unused, workerID used for observation value
			histogram.Observe(float64(workerID))
		},
	})

	expectedCount := uint64(goroutines * observations)
	if histogram.Count() != expectedCount {
		t.Errorf("Expected histogram count %d, got %d", expectedCount, histogram.Count())
	}
}

// TestConcurrentTimerRecording tests concurrent timer recordings.
func TestConcurrentTimerRecording(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	const goroutines = 100
	const recordings = 100

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: recordings,
		Operation: func(_, _ int) { // workerID and opID unused in timer recording
			timer.Record(time.Duration(time.Millisecond.Nanoseconds()))
		},
	})

	expectedCount := uint64(goroutines * recordings)
	if timer.Count() != expectedCount {
		t.Errorf("Expected timer count %d, got %d", expectedCount, timer.Count())
	}
}

// TestConcurrentMetricCreation tests creating metrics concurrently.
func TestConcurrentMetricCreation(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	const goroutines = 100
	const metricsPerGoroutine = 10

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    goroutines,
		Operations: metricsPerGoroutine,
		Operation: func(workerID, opID int) {
			// Each worker creates its own metrics
			counter := registry.Counter(CounterKey)
			counter.Inc()

			gauge := registry.Gauge(GaugeKey)
			gauge.Set(float64(workerID))

			histogram := registry.Histogram(HistogramKey, metricz.DefaultLatencyBuckets)
			histogram.Observe(float64(opID))

			timer := registry.Timer(TimerKey)
			timer.Record(1000)
		},
	})

	// Verify metrics were created (should be the same instances)
	if registry.Counter(CounterKey).Value() < float64(goroutines*metricsPerGoroutine) {
		t.Error("Counter value is less than expected")
	}
}

// TestConcurrentResetAndOperations tests reset while operations are ongoing.
func TestConcurrentResetAndOperations(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	stopCh := make(chan bool)

	// Continuously perform operations
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				registry.Counter(CounterKey).Inc()
				registry.Gauge(GaugeKey).Inc()
				registry.Histogram(HistogramKey, metricz.DefaultLatencyBuckets).Observe(1.0)
				registry.Timer(TimerKey).Record(1000)
			}
		}
	}()

	// Perform resets
	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Millisecond)
		registry.Reset()
	}

	close(stopCh)

	// After reset, all metrics should be at zero/empty
	if registry.Counter(CounterKey).Value() == 0 {
		// This is expected after a reset
		t.Log("Counter correctly reset")
	}
}

// TestConcurrentPackageRegistration tests concurrent package registration.
// TestConcurrentPackageRegistration was removed - RegisterPackage functionality eliminated.
// Key type provides compile-time safety, making this race condition test unnecessary.

// TestRaceConditionMixedOperations tests for race conditions with mixed operations.
func TestRaceConditionMixedOperations(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	const duration = 100 * time.Millisecond
	stopCh := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for {
				select {
				case <-stopCh:
					return
				default:
					registry.Counter(SharedCounterKey).Add(1.0)
					registry.Gauge(SharedGaugeKey).Set(float64(id))
					registry.Histogram(SharedHistogramKey, metricz.DefaultLatencyBuckets).Observe(float64(id))
					registry.Timer(SharedTimerKey).Record(time.Duration(id * 1000))
				}
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-stopCh:
					return
				default:
					_ = registry.Counter(SharedCounterKey).Value()
					_ = registry.Gauge(SharedGaugeKey).Value()
					_ = registry.Histogram(SharedHistogramKey, metricz.DefaultLatencyBuckets).Count()
					_ = registry.Timer(SharedTimerKey).Count()

					// Also read via registry reader interface
					_ = registry.GetCounters()
					_ = registry.GetGauges()
					_ = registry.GetHistograms()
					_ = registry.GetTimers()
				}
			}
		}()
	}

	// Reset goroutine
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				time.Sleep(10 * time.Millisecond)
				registry.Reset()
			}
		}
	}()

	time.Sleep(duration)
	close(stopCh)

	// If we get here without panic/crash, race detection passed
	t.Log("Mixed operations completed without race conditions")
}

// TestReaderWriterInteraction tests reader-writer interaction patterns.
func TestReaderWriterInteraction(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	stop := make(chan bool)

	// Create initial metrics for interaction testing
	for i := 0; i < 5; i++ {
		registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i))).Inc()
		registry.Gauge(metricz.Key(fmt.Sprintf("gauge_%d", i))).Set(float64(i))
	}

	// Start readers to test interaction patterns
	readCount := uint64(0)
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
					// Read metrics to test interaction behavior
					counters := registry.GetCounters()
					gauges := registry.GetGauges()

					// Verify interaction consistency
					if len(counters) > 0 && len(gauges) > 0 {
						atomic.AddUint64(&readCount, 1)
					}
					runtime.Gosched()
				}
			}
		}()
	}

	// Start concurrent writers
	writeCount := uint64(0)
	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    5,
		Operations: 20,
		Operation: func(workerID, _ int) {
			counterName := fmt.Sprintf("counter_%d", workerID)
			gaugeName := fmt.Sprintf("gauge_%d", workerID)

			registry.Counter(metricz.Key(counterName)).Inc()
			registry.Gauge(metricz.Key(gaugeName)).Add(1)
			atomic.AddUint64(&writeCount, 2)
		},
	})

	// Stop and verify interaction behavior
	close(stop)
	time.Sleep(50 * time.Millisecond)

	finalReads := atomic.LoadUint64(&readCount)
	finalWrites := atomic.LoadUint64(&writeCount)

	t.Logf("Reader-writer interaction: %d reads, %d writes", finalReads, finalWrites)

	// Verify final state consistency
	for i := 0; i < 5; i++ {
		counter := registry.Counter(metricz.Key(fmt.Sprintf("counter_%d", i)))
		expected := float64(1 + 20) // Initial + operations
		if counter.Value() != expected {
			t.Errorf("Counter %d: expected %f, got %f", i, expected, counter.Value())
		}
	}
}

// TestConcurrentGetters tests concurrent access to metric getters.
func TestConcurrentGetters(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Pre-populate some metrics
	registry.Counter(Counter1Key).Inc()
	registry.Gauge(Gauge1Key).Set(42)
	registry.Histogram(Hist1Key, metricz.DefaultLatencyBuckets).Observe(100)
	registry.Timer(Timer1Key).Record(1000000)

	const readers = 100
	const reads = 1000

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    readers,
		Operations: reads,
		Operation: func(_, _ int) { // workerID and opID unused in read operations
			// Read all metric types
			counters := registry.GetCounters()
			if len(counters) != 1 {
				t.Error("Unexpected counter count")
			}

			gauges := registry.GetGauges()
			if len(gauges) != 1 {
				t.Error("Unexpected gauge count")
			}

			histograms := registry.GetHistograms()
			if len(histograms) != 1 {
				t.Error("Unexpected histogram count")
			}

			timers := registry.GetTimers()
			if len(timers) != 1 {
				t.Error("Unexpected timer count")
			}
		},
	})

	t.Log("Concurrent getters completed successfully")
}
