package metricz_test

import (
	"testing"
	"time"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/metricz"
	metricstesting "github.com/zoobzio/metricz/testing"
)

func TestTimer_Record(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Initial state
	if timer.Count() != 0 {
		t.Errorf("Initial timer count should be 0, got %d", timer.Count())
	}
	if timer.Sum() != 0 {
		t.Errorf("Initial timer sum should be 0, got %f", timer.Sum())
	}

	// Record a duration
	duration := 100 * time.Millisecond
	timer.Record(duration)

	if timer.Count() != 1 {
		t.Errorf("After one record, count should be 1, got %d", timer.Count())
	}

	// Sum should be approximately 100ms (converted to milliseconds)
	expectedSum := 100.0 // milliseconds
	if timer.Sum() != expectedSum {
		t.Errorf("After recording 100ms, sum should be %f ms, got %f ms",
			expectedSum, timer.Sum())
	}
}

func TestTimer_RecordMultiple(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Record multiple durations
	durations := []time.Duration{
		50 * time.Millisecond,
		200 * time.Millisecond,
		1 * time.Second,
	}

	for _, duration := range durations {
		timer.Record(duration)
	}

	if timer.Count() != uint64(len(durations)) {
		t.Errorf("After recording %d durations, count should be %d, got %d",
			len(durations), len(durations), timer.Count())
	}

	// Expected sum: 50 + 200 + 1000 = 1250 ms
	expectedSum := 50.0 + 200.0 + 1000.0
	if timer.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms, got %f ms", expectedSum, timer.Sum())
	}
}

func TestTimer_Start_Stop(t *testing.T) {
	clock := clockz.NewFakeClock()
	registry := metricstesting.NewTestRegistryWithClock(t, clock)
	timer := registry.Timer(TestTimerKey)

	// Start timing
	stopwatch := timer.Start()

	// Advance clock by exactly 10ms
	clock.Advance(10 * time.Millisecond)

	// Stop timing
	stopwatch.Stop()

	// Should have recorded one measurement
	if timer.Count() != 1 {
		t.Errorf("After Start/Stop, count should be 1, got %d", timer.Count())
	}

	// Sum should be exactly 10ms with FakeClock
	sum := timer.Sum()
	if sum != 10.0 {
		t.Errorf("Expected exactly 10ms, got %f ms", sum)
	}
}

func TestTimer_MultipleStopwatches(t *testing.T) {
	clock := clockz.NewFakeClock()
	registry := metricstesting.NewTestRegistryWithClock(t, clock)
	timer := registry.Timer(TestTimerKey)

	const numStopwatches = 5
	stopwatches := make([]*metricz.Stopwatch, numStopwatches)

	// Start multiple stopwatches
	for i := range stopwatches {
		stopwatches[i] = timer.Start()
		clock.Advance(1 * time.Millisecond) // Stagger start times
	}

	// Stop them in reverse order
	for i := len(stopwatches) - 1; i >= 0; i-- {
		stopwatches[i].Stop()
		clock.Advance(1 * time.Millisecond) // Allow time between stops
	}

	if timer.Count() != numStopwatches {
		t.Errorf("Expected count %d, got %d", numStopwatches, timer.Count())
	}

	// Each stopwatch should record its accumulated time
	// Stopwatch 0: started at 0ms, stopped at 9ms = 9ms
	// Stopwatch 1: started at 1ms, stopped at 8ms = 7ms
	// Stopwatch 2: started at 2ms, stopped at 7ms = 5ms
	// Stopwatch 3: started at 3ms, stopped at 6ms = 3ms
	// Stopwatch 4: started at 4ms, stopped at 5ms = 1ms
	// Total: 9+7+5+3+1 = 25ms
	expectedSum := 25.0
	if timer.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms, got %f ms", expectedSum, timer.Sum())
	}
}

func TestTimer_Buckets(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Record some durations that should fall in different buckets
	timer.Record(500 * time.Microsecond) // 0.5 ms - first bucket
	timer.Record(3 * time.Millisecond)   // 3 ms - second/third bucket
	timer.Record(50 * time.Millisecond)  // 50 ms - middle bucket
	timer.Record(2 * time.Second)        // 2000 ms - high bucket

	buckets, counts := timer.Buckets()

	// Should use metricz.DefaultLatencyBuckets + overflow bucket
	expectedBuckets := len(metricz.DefaultLatencyBuckets) + 1
	if len(buckets) != expectedBuckets {
		t.Errorf("Expected %d buckets, got %d", expectedBuckets, len(buckets))
	}

	// Verify buckets match default
	for i, expected := range metricz.DefaultLatencyBuckets {
		if buckets[i] != expected {
			t.Errorf("Bucket %d: expected %f, got %f", i, expected, buckets[i])
		}
	}

	// Verify at least some buckets have counts
	totalCount := uint64(0)
	for _, count := range counts {
		totalCount += count
	}

	if totalCount != 4 {
		t.Errorf("Expected total count 4, got %d", totalCount)
	}
}

func TestTimer_ConcurrentRecording(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	const workers = 50
	const recordings = 10

	// Use GenerateLoad for standardized concurrent testing
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers,
		Operations: recordings,
		Operation: func(_, opID int) {
			duration := time.Duration(opID+1) * time.Millisecond
			timer.Record(duration)
		},
	})

	expectedCount := uint64(workers * recordings)
	if timer.Count() != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, timer.Count())
	}

	// Sum should be positive
	if timer.Sum() <= 0 {
		t.Error("Timer sum should be positive after concurrent recordings")
	}
}

func TestTimer_ConcurrentStartStop(t *testing.T) {
	clock := clockz.NewFakeClock()
	registry := metricstesting.NewTestRegistryWithClock(t, clock)
	timer := registry.Timer(TestTimerKey)

	const workers = 20

	// Use GenerateLoad for Start/Stop pattern testing
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers,
		Operations: 1, // One Start/Stop per worker
		Operation: func(workerID, _ int) {
			stopwatch := timer.Start()
			// Record a duration based on worker ID for deterministic results
			duration := time.Duration(workerID+1) * time.Millisecond
			timer.Record(duration)
			// Also test Stop functionality without clock advancement
			stopwatch.Stop() // This will record 0ms since no time elapsed
		},
	})

	if timer.Count() != workers*2 { // Each worker records twice: once via Record, once via Stop
		t.Errorf("Expected count %d, got %d", workers*2, timer.Count())
	}

	// Sum should be 1+2+3+...+20 = 210ms from Record calls, plus 0ms from Stop calls
	expectedSum := float64(workers * (workers + 1) / 2)
	if timer.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms, got %f ms", expectedSum, timer.Sum())
	}
}

func TestTimer_NanosecondPrecision(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Test very short duration
	shortDuration := 500 * time.Nanosecond
	timer.Record(shortDuration)

	// Should be converted to milliseconds: 0.0005 ms
	expectedSum := 0.0005
	if timer.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms for 500ns, got %f ms", expectedSum, timer.Sum())
	}
}

func TestTimer_LargeDurations(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Test large duration
	largeDuration := 1 * time.Hour
	timer.Record(largeDuration)

	// Should be converted to milliseconds: 3,600,000 ms
	expectedSum := 3600000.0
	if timer.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms for 1 hour, got %f ms", expectedSum, timer.Sum())
	}
}

func TestTimer_ZeroDuration(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Test zero duration
	timer.Record(0)

	if timer.Count() != 1 {
		t.Error("Zero duration should still be counted")
	}

	if timer.Sum() != 0.0 {
		t.Error("Zero duration should contribute 0 to sum")
	}
}

func TestStopwatch_MultipleStops(t *testing.T) {
	clock := clockz.NewFakeClock()
	registry := metricstesting.NewTestRegistryWithClock(t, clock)
	timer := registry.Timer(TestTimerKey)

	stopwatch := timer.Start()
	clock.Advance(1 * time.Millisecond)

	// First stop should record the duration
	stopwatch.Stop()

	if timer.Count() != 1 {
		t.Error("First stop should record measurement")
	}

	firstSum := timer.Sum()
	if firstSum != 1.0 {
		t.Errorf("First stop should record exactly 1ms, got %f ms", firstSum)
	}

	// Second stop on same stopwatch should record again
	clock.Advance(1 * time.Millisecond)
	stopwatch.Stop()

	if timer.Count() != 2 {
		t.Error("Second stop should record another measurement")
	}

	// Sum should be 1ms + 2ms = 3ms (stopwatch measures from original start time)
	expectedSum := 3.0
	if timer.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms after second stop, got %f ms", expectedSum, timer.Sum())
	}
}

func TestTimer_Interface(t *testing.T) {
	clock := clockz.NewFakeClock()
	registry := metricstesting.NewTestRegistryWithClock(t, clock)
	var tm metricz.Timer = registry.Timer(TestTimerKey)

	// Test interface methods
	duration := 100 * time.Millisecond
	tm.Record(duration)

	if tm.Count() != 1 {
		t.Error("Timer interface Count() failed")
	}

	if tm.Sum() != 100.0 {
		t.Error("Timer interface Sum() failed")
	}

	stopwatch := tm.Start()
	clock.Advance(1 * time.Millisecond)
	stopwatch.Stop()

	if tm.Count() != 2 {
		t.Error("Timer interface Start/Stop failed")
	}

	// Sum should be 100ms + 1ms = 101ms
	expectedSum := 101.0
	if tm.Sum() != expectedSum {
		t.Errorf("Expected sum %f ms, got %f ms", expectedSum, tm.Sum())
	}

	buckets, counts := tm.Buckets()
	if len(buckets) == 0 {
		t.Error("Timer interface Buckets() failed")
	}

	totalCount := uint64(0)
	for _, count := range counts {
		totalCount += count
	}

	if totalCount != 2 {
		t.Error("Timer interface bucket counts don't match total count")
	}
}
