package metricz_test

import (
	"testing"
	"time"

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
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	// Start timing
	stopwatch := timer.Start()

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// Stop timing
	stopwatch.Stop()

	// Should have recorded one measurement
	if timer.Count() != 1 {
		t.Errorf("After Start/Stop, count should be 1, got %d", timer.Count())
	}

	// Sum should be approximately 10ms (allow for timing variance)
	sum := timer.Sum()
	if sum < 5.0 || sum > 50.0 { // Allow generous range for timing variance
		t.Errorf("Expected sum around 10ms, got %f ms", sum)
	}
}

func TestTimer_MultipleStopwatches(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	const numStopwatches = 5
	stopwatches := make([]*metricz.Stopwatch, numStopwatches)

	// Start multiple stopwatches
	for i := range stopwatches {
		stopwatches[i] = timer.Start()
		time.Sleep(1 * time.Millisecond) // Stagger start times
	}

	// Stop them in reverse order
	for i := len(stopwatches) - 1; i >= 0; i-- {
		stopwatches[i].Stop()
		time.Sleep(1 * time.Millisecond) // Allow time between stops
	}

	if timer.Count() != numStopwatches {
		t.Errorf("Expected count %d, got %d", numStopwatches, timer.Count())
	}

	// All measurements should be recorded
	if timer.Sum() <= 0 {
		t.Error("Timer sum should be positive after multiple measurements")
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
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	const workers = 20

	// Use GenerateLoad for Start/Stop pattern testing
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers,
		Operations: 1, // One Start/Stop per worker
		Operation: func(_, _ int) {
			stopwatch := timer.Start()
			time.Sleep(1 * time.Millisecond)
			stopwatch.Stop()
		},
	})

	if timer.Count() != workers {
		t.Errorf("Expected count %d, got %d", workers, timer.Count())
	}

	// All measurements should have some duration
	if timer.Sum() <= 0 {
		t.Error("Timer sum should be positive after concurrent start/stop")
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
	registry := metricstesting.NewTestRegistry(t)
	timer := registry.Timer(TestTimerKey)

	stopwatch := timer.Start()
	time.Sleep(1 * time.Millisecond)

	// First stop should record the duration
	stopwatch.Stop()

	if timer.Count() != 1 {
		t.Error("First stop should record measurement")
	}

	firstSum := timer.Sum()

	// Second stop on same stopwatch should record again
	time.Sleep(1 * time.Millisecond)
	stopwatch.Stop()

	if timer.Count() != 2 {
		t.Error("Second stop should record another measurement")
	}

	// Sum should have increased
	if timer.Sum() <= firstSum {
		t.Error("Second stop should add to the sum")
	}
}

func TestTimer_Interface(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
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
	time.Sleep(1 * time.Millisecond)
	stopwatch.Stop()

	if tm.Count() != 2 {
		t.Error("Timer interface Start/Stop failed")
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
