package metricz_test

import (
	"math"
	"testing"

	"github.com/zoobzio/metricz"
	metricstesting "github.com/zoobzio/metricz/testing"
)

func TestCounter_Inc(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	// Initial value should be zero
	if counter.Value() != 0 {
		t.Errorf("Initial counter value should be 0, got %f", counter.Value())
	}

	// Increment once
	counter.Inc()
	if counter.Value() != 1.0 {
		t.Errorf("After Inc(), counter should be 1.0, got %f", counter.Value())
	}

	// Increment multiple times
	counter.Inc()
	counter.Inc()
	if counter.Value() != 3.0 {
		t.Errorf("After 3 Inc() calls, counter should be 3.0, got %f", counter.Value())
	}
}

func TestCounter_Add(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	// Add positive value
	counter.Add(5.5)
	if counter.Value() != 5.5 {
		t.Errorf("After Add(5.5), counter should be 5.5, got %f", counter.Value())
	}

	// Add another positive value
	counter.Add(2.3)
	expected := 5.5 + 2.3
	if counter.Value() != expected {
		t.Errorf("After Add(2.3), counter should be %f, got %f", expected, counter.Value())
	}

	// Add zero (should be no-op but not error)
	counter.Add(0)
	if counter.Value() != expected {
		t.Errorf("After Add(0), counter should remain %f, got %f", expected, counter.Value())
	}
}

func TestCounter_Add_NegativeValues(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	// Set initial value
	counter.Add(10.0)
	initial := counter.Value()

	// Try to add negative value - should be ignored
	counter.Add(-5.0)
	if counter.Value() != initial {
		t.Errorf("Counter should ignore negative values, expected %f, got %f",
			initial, counter.Value())
	}

	// Try to add negative zero - should be ignored
	//nolint:staticcheck // Testing negative zero behavior explicitly
	counter.Add(-0.0)
	if counter.Value() != initial {
		t.Errorf("Counter should ignore negative zero, expected %f, got %f",
			initial, counter.Value())
	}
}

func TestCounter_ConcurrentAccess(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	const workers = 100
	const operations = 1000

	// Use GenerateLoad for standardized concurrent testing
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers,
		Operations: operations,
		Operation: func(_, _ int) {
			counter.Inc()
		},
	})

	expected := float64(workers * operations)
	if counter.Value() != expected {
		t.Errorf("Expected counter value %f after concurrent increments, got %f",
			expected, counter.Value())
	}
}

func TestCounter_ConcurrentAddAndInc(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	const workers = 50
	const operations = 100

	// Use GenerateLoad with Setup to configure operation type per worker
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers * 2, // Double workers for two operation types
		Operations: operations,
		Operation: func(workerID, _ int) {
			// First half of workers use Inc(), second half use Add(2.5)
			if workerID < workers {
				counter.Inc()
			} else {
				counter.Add(2.5)
			}
		},
	})

	// Expected: workers * operations * 1.0 + workers * operations * 2.5
	expected := float64(workers*operations) + float64(workers*operations)*2.5
	if counter.Value() != expected {
		t.Errorf("Expected counter value %f after concurrent mixed operations, got %f",
			expected, counter.Value())
	}
}

func TestCounter_LargeValues(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	// Test with large float values
	largeValue := 1e15
	counter.Add(largeValue)

	if counter.Value() != largeValue {
		t.Errorf("Counter should handle large values, expected %e, got %e",
			largeValue, counter.Value())
	}

	// Add another large value
	counter.Add(largeValue)
	expected := largeValue * 2

	if counter.Value() != expected {
		t.Errorf("Counter should handle addition of large values, expected %e, got %e",
			expected, counter.Value())
	}
}

func TestCounter_SmallIncrements(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	// Test with very small increments
	smallValue := 1e-10
	iterations := 1000

	for i := 0; i < iterations; i++ {
		counter.Add(smallValue)
	}

	expected := float64(iterations) * smallValue
	result := counter.Value()

	// Allow for small floating point precision errors
	diff := result - expected
	if diff < 0 {
		diff = -diff
	}

	if diff > 1e-9 { // Tolerance for floating point precision
		t.Errorf("Counter precision test failed, expected ~%e, got %e (diff: %e)",
			expected, result, diff)
	}
}

func TestCounter_Interface(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	var c metricz.Counter = registry.Counter(TestCounterKey)

	// Test that the interface methods work correctly
	c.Inc()
	if c.Value() != 1.0 {
		t.Error("Counter interface Inc() failed")
	}

	c.Add(5.0)
	if c.Value() != 6.0 {
		t.Error("Counter interface Add() failed")
	}

	// Ensure negative add is ignored through interface
	c.Add(-1.0)
	if c.Value() != 6.0 {
		t.Error("Counter interface should ignore negative Add()")
	}
}

func TestCounterNaNInfinityRejection(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	counter := registry.Counter(TestCounterKey)

	counter.Add(5.0)
	if counter.Value() != 5.0 {
		t.Errorf("Expected counter value 5.0, got %f", counter.Value())
	}

	// These should be rejected silently
	counter.Add(math.NaN())
	counter.Add(math.Inf(1))
	counter.Add(math.Inf(-1))

	// Value unchanged
	if counter.Value() != 5.0 {
		t.Errorf("Expected counter value 5.0 after invalid inputs, got %f", counter.Value())
	}
	if math.IsNaN(counter.Value()) {
		t.Error("Counter value should not be NaN after rejecting NaN input")
	}
	if math.IsInf(counter.Value(), 0) {
		t.Error("Counter value should not be Inf after rejecting Inf input")
	}
}
