package reliability

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// getLoadConfig returns appropriate load configuration based on environment.
// Set METRICZ_STRESS_TEST=true for heavy stress testing.
// Default values are optimized for quick CI runs.
func getLoadConfig() (workers int, operations int, duration time.Duration) {
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		// Heavy stress testing mode
		return 1000, 10000, 10 * time.Second
	}
	// Default: quick CI mode
	return 10, 100, 100 * time.Millisecond
}

// Test metric keys - the RIGHT way to use Key type.
const (
	TestKey          metricz.Key = "test"
	FinalTestKey     metricz.Key = "final_test"
	SharedCounterKey metricz.Key = "shared_counter"
	SharedGaugeKey   metricz.Key = "shared_gauge"
	SurvivorKey      metricz.Key = "survivor"
	LatencyKey       metricz.Key = "latency"
)

// TestCASLoopStarvation tests if the atomic CAS loop can starve under extreme contention.
func TestCASLoopStarvation(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	counter := registry.Counter(TestKey)

	workers, operations, _ := getLoadConfig()

	start := time.Now()

	// Use GenerateLoad for extreme contention on a single counter
	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    workers,
		Operations: operations,
		Operation: func(_, _ int) { // workerID and opID unused in this stress test
			counter.Add(0.001)
		},
	})

	duration := time.Since(start)
	opsPerSec := float64(workers*operations) / duration.Seconds()
	t.Logf("Completed %d operations in %v (%.0f ops/sec)",
		workers*operations, duration, opsPerSec)

	// Verify all operations completed
	// Allow for minor floating point precision errors
	expected := float64(workers*operations) * 0.001
	actual := counter.Value()
	tolerance := 0.00001 // Allow for tiny floating point errors from many atomic operations
	if math.Abs(actual-expected) > tolerance {
		t.Errorf("Expected counter value %f (±%f), got %f", expected, tolerance, actual)
	}

	// GenerateLoad handles timeout internally, so no need for additional timeout
}

// TestResetDuringHeavyLoad tests registry reset while under heavy concurrent load.
func TestResetDuringHeavyLoad(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	stopWriters := make(chan bool)
	stopReaders := make(chan bool)
	stopResetter := make(chan bool)

	var writeOps uint64
	var readOps uint64
	var resetOps uint64

	// Start writer goroutines
	for i := 0; i < 50; i++ {
		go func(id int) {
			for {
				select {
				case <-stopWriters:
					return
				default:
					registry.Counter(metricz.Key(fmt.Sprintf("c%d", id%10))).Inc()
					registry.Gauge(metricz.Key(fmt.Sprintf("g%d", id%10))).Set(float64(id))
					registry.Histogram(metricz.Key(fmt.Sprintf("h%d", id%5)), metricz.DefaultLatencyBuckets).Observe(float64(id))
					atomic.AddUint64(&writeOps, 3)
				}
			}
		}(i)
	}

	// Start reader goroutines
	for i := 0; i < 50; i++ {
		go func() {
			for {
				select {
				case <-stopReaders:
					return
				default:
					_ = registry.GetCounters()
					_ = registry.GetGauges()
					_ = registry.GetHistograms()
					atomic.AddUint64(&readOps, 3)
				}
			}
		}()
	}

	// Start resetter goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopResetter:
				return
			case <-ticker.C:
				registry.Reset()
				atomic.AddUint64(&resetOps, 1)
			}
		}
	}()

	// Run for configured duration
	var testDuration time.Duration
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		testDuration = 5 * time.Second // Heavy mode: 5 seconds
	} else {
		testDuration = 500 * time.Millisecond // CI mode: 500ms
	}
	time.Sleep(testDuration)

	// Stop all operations
	close(stopWriters)
	close(stopReaders)
	close(stopResetter)

	// Give goroutines time to stop
	time.Sleep(100 * time.Millisecond)

	writes := atomic.LoadUint64(&writeOps)
	reads := atomic.LoadUint64(&readOps)
	resets := atomic.LoadUint64(&resetOps)

	t.Logf("Operations completed: Writes=%d, Reads=%d, Resets=%d", writes, reads, resets)

	// Test should complete without panic or deadlock
	// After all this chaos, registry should still be functional
	registry.Counter(FinalTestKey).Inc()
	if registry.Counter(FinalTestKey).Value() != 1 {
		t.Error("Registry not functional after stress test")
	}
}

// TestConcurrentMetricCreation tests creating metrics from many goroutines simultaneously.
func TestConcurrentMetricCreation(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	var workers, metricsPerWorker int
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		workers = 1000
		metricsPerWorker = 100
	} else {
		// CI mode: smaller scale but still tests concurrency
		workers = 50
		metricsPerWorker = 10
	}

	start := time.Now()

	// Use GenerateLoad for concurrent metric creation
	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    workers,
		Operations: metricsPerWorker,
		Operation: func(workerID, opID int) {
			// Each goroutine creates unique metrics
			counterName := fmt.Sprintf("counter_%d_%d", workerID, opID)
			gaugeName := fmt.Sprintf("gauge_%d_%d", workerID, opID)

			registry.Counter(metricz.Key(counterName)).Inc()
			registry.Gauge(metricz.Key(gaugeName)).Set(float64(workerID * opID))

			// Also access some shared metrics for contention
			registry.Counter(SharedCounterKey).Inc()
			registry.Gauge(SharedGaugeKey).Add(1)
		},
	})

	duration := time.Since(start)

	// Verify metrics were created
	counters := registry.GetCounters()
	gauges := registry.GetGauges()

	expectedCounters := workers*metricsPerWorker + 1 // +1 for shared_counter
	expectedGauges := workers*metricsPerWorker + 1   // +1 for shared_gauge

	if len(counters) != expectedCounters {
		t.Errorf("Expected %d counters, got %d", expectedCounters, len(counters))
	}

	if len(gauges) != expectedGauges {
		t.Errorf("Expected %d gauges, got %d", expectedGauges, len(gauges))
	}

	// Check shared counter value
	sharedCounter := registry.Counter(SharedCounterKey)
	if sharedCounter.Value() != float64(workers*metricsPerWorker) {
		t.Errorf("Shared counter should be %d, got %f",
			workers*metricsPerWorker, sharedCounter.Value())
	}

	t.Logf("Created %d metrics in %v", expectedCounters+expectedGauges, duration)
}

// TestChaosOperations performs random operations to find race conditions.
func TestChaosOperations(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	stop := make(chan bool)
	var operationCount uint64

	operations := []func(){
		// Counter operations
		func() {
			registry.Counter(metricz.Key("c" + strconv.Itoa(rand.Intn(100)))).Inc()
		},
		func() {
			registry.Counter(metricz.Key("c" + strconv.Itoa(rand.Intn(100)))).Add(rand.Float64())
		},

		// Gauge operations
		func() {
			registry.Gauge(metricz.Key("g" + strconv.Itoa(rand.Intn(100)))).Set(rand.Float64())
		},
		func() {
			registry.Gauge(metricz.Key("g" + strconv.Itoa(rand.Intn(100)))).Inc()
		},
		func() {
			registry.Gauge(metricz.Key("g" + strconv.Itoa(rand.Intn(100)))).Dec()
		},

		// Histogram operations
		func() {
			registry.Histogram(metricz.Key("h"+strconv.Itoa(rand.Intn(10))), metricz.DefaultLatencyBuckets).Observe(rand.Float64() * 1000)
		},

		// Timer operations
		func() {
			registry.Timer(metricz.Key("t" + strconv.Itoa(rand.Intn(10)))).Record(time.Duration(rand.Intn(1000)))
		},
		func() {
			sw := registry.Timer(metricz.Key("t" + strconv.Itoa(rand.Intn(10)))).Start()
			time.Sleep(time.Microsecond)
			sw.Stop()
		},

		// Reader operations
		func() {
			_ = registry.GetCounters()
		},
		func() {
			_ = registry.GetGauges()
		},
		func() {
			_ = registry.GetHistograms()
		},
		func() {
			_ = registry.GetTimers()
		},

		// Reset operation (less frequent)
		func() {
			if rand.Float32() < 0.01 { // 1% chance
				registry.Reset()
			}
		},

		// Package registration removed - Key type provides compile-time safety
		func() {
			// Previously registered packages, now just create metrics directly
		},
	}

	// Start chaos goroutines
	for i := 0; i < 100; i++ {
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
					// Pick random operation
					op := operations[rand.Intn(len(operations))]
					op()
					atomic.AddUint64(&operationCount, 1)
				}
			}
		}()
	}

	// Run chaos for configured duration
	var chaosDuration time.Duration
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		chaosDuration = 1 * time.Second
	} else {
		chaosDuration = 100 * time.Millisecond // Quick for CI
	}
	time.Sleep(chaosDuration)
	close(stop)

	// Give goroutines time to stop
	time.Sleep(100 * time.Millisecond)

	ops := atomic.LoadUint64(&operationCount)
	t.Logf("Completed %d random operations without panic", ops)

	// Registry should still be functional
	registry.Counter(SurvivorKey).Inc()
	if registry.Counter(SurvivorKey).Value() != 1 {
		t.Error("Registry not functional after chaos test")
	}
}

// TestHighFrequencyTimers tests timer under high-frequency recording.
func TestHighFrequencyTimers(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	timer := registry.Timer(LatencyKey)

	var workers, recordings int
	if os.Getenv("METRICZ_STRESS_TEST") == "true" {
		workers = 100
		recordings = 10000
	} else {
		// CI mode: still high frequency but manageable
		workers = 10
		recordings = 1000
	}

	start := time.Now()

	// Use GenerateLoad for high-frequency timer operations
	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    workers,
		Operations: recordings,
		Operation: func(_, opID int) { // workerID unused, opID used for timing variation
			// Mix of Record and Start/Stop
			if opID%2 == 0 {
				timer.Record(time.Duration(opID) * time.Microsecond)
			} else {
				sw := timer.Start()
				// Minimal work
				runtime.Gosched()
				sw.Stop()
			}
		},
	})

	duration := time.Since(start)

	expectedCount := uint64(workers) * uint64(recordings) //nolint:gosec // workers and recordings are always positive and bounded
	actualCount := timer.Count()

	if actualCount != expectedCount {
		t.Errorf("Expected %d timer recordings, got %d", expectedCount, actualCount)
	}

	recordingsPerSec := float64(expectedCount) / duration.Seconds()
	t.Logf("Timer handled %.0f recordings/sec", recordingsPerSec)
}

// TestConcurrentPackageRegistration was removed - RegisterPackage functionality eliminated.
// Key type provides compile-time safety, making package registration unnecessary.

// TestHighContentionCounter tests a single counter under extreme contention.
func TestHighContentionCounter(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	counter := registry.Counter(TestKey)

	workers, operations, _ := getLoadConfig()

	// Scale up for stress testing
	stressWorkers := workers * 10 // 10x workers for contention
	stressOps := operations

	start := time.Now()

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    stressWorkers,
		Operations: stressOps,
		Operation: func(_, _ int) {
			counter.Inc()
		},
	})

	duration := time.Since(start)
	expected := float64(stressWorkers * stressOps)

	if counter.Value() != expected {
		t.Errorf("Expected %f, got %f", expected, counter.Value())
	}

	t.Logf("High contention test completed in %v with %d workers", duration, stressWorkers)
}
