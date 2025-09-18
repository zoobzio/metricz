package metricz_test

import (
	"math"
	"sync"
	"testing"

	"github.com/zoobzio/metricz"
)

func TestGauge_Set(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Initial value should be zero
	if gauge.Value() != 0 {
		t.Errorf("Initial gauge value should be 0, got %f", gauge.Value())
	}

	// Set to positive value
	gauge.Set(42.5)
	if gauge.Value() != 42.5 {
		t.Errorf("After Set(42.5), gauge should be 42.5, got %f", gauge.Value())
	}

	// Set to negative value
	gauge.Set(-15.3)
	if gauge.Value() != -15.3 {
		t.Errorf("After Set(-15.3), gauge should be -15.3, got %f", gauge.Value())
	}

	// Set to zero
	gauge.Set(0.0)
	if gauge.Value() != 0.0 {
		t.Errorf("After Set(0.0), gauge should be 0.0, got %f", gauge.Value())
	}
}

func TestGauge_Add(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Add positive value
	gauge.Add(5.5)
	if gauge.Value() != 5.5 {
		t.Errorf("After Add(5.5), gauge should be 5.5, got %f", gauge.Value())
	}

	// Add negative value (unlike Counter, Gauge allows this)
	gauge.Add(-2.0)
	expected := 5.5 - 2.0
	if gauge.Value() != expected {
		t.Errorf("After Add(-2.0), gauge should be %f, got %f", expected, gauge.Value())
	}

	// Add zero
	gauge.Add(0.0)
	if gauge.Value() != expected {
		t.Errorf("After Add(0.0), gauge should remain %f, got %f", expected, gauge.Value())
	}
}

func TestGauge_Inc(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Increment from zero
	gauge.Inc()
	if gauge.Value() != 1.0 {
		t.Errorf("After Inc(), gauge should be 1.0, got %f", gauge.Value())
	}

	// Increment again
	gauge.Inc()
	if gauge.Value() != 2.0 {
		t.Errorf("After second Inc(), gauge should be 2.0, got %f", gauge.Value())
	}

	// Set to negative value and increment
	gauge.Set(-5.0)
	gauge.Inc()
	if gauge.Value() != -4.0 {
		t.Errorf("After Inc() from -5.0, gauge should be -4.0, got %f", gauge.Value())
	}
}

func TestGauge_Dec(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Decrement from zero
	gauge.Dec()
	if gauge.Value() != -1.0 {
		t.Errorf("After Dec(), gauge should be -1.0, got %f", gauge.Value())
	}

	// Decrement again
	gauge.Dec()
	if gauge.Value() != -2.0 {
		t.Errorf("After second Dec(), gauge should be -2.0, got %f", gauge.Value())
	}

	// Set to positive value and decrement
	gauge.Set(10.0)
	gauge.Dec()
	if gauge.Value() != 9.0 {
		t.Errorf("After Dec() from 10.0, gauge should be 9.0, got %f", gauge.Value())
	}
}

func TestGauge_CombinedOperations(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Test sequence of operations
	gauge.Set(10.0)
	gauge.Inc()     // 11.0
	gauge.Dec()     // 10.0
	gauge.Add(5.5)  // 15.5
	gauge.Add(-3.2) // 12.3

	expected := 12.3
	if gauge.Value() != expected {
		t.Errorf("After combined operations, gauge should be %f, got %f",
			expected, gauge.Value())
	}
}

func TestGauge_ConcurrentAccess(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	const workers = 100
	const operations = 100

	// Manual concurrent testing with three operation types
	var wg sync.WaitGroup
	// Workers that increment
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				gauge.Inc()
			}
		}()
	}
	// Workers that decrement
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				gauge.Dec()
			}
		}()
	}
	// Workers that add 0.5
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				gauge.Add(0.5)
			}
		}()
	}
	wg.Wait()

	// Expected: +workers*operations -workers*operations +workers*operations*0.5
	// = 0 + workers*operations*0.5
	expected := float64(workers*operations) * 0.5
	if gauge.Value() != expected {
		t.Errorf("Expected gauge value %f after concurrent operations, got %f",
			expected, gauge.Value())
	}
}

func TestGauge_ConcurrentSetAndRead(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	const workers = 50
	const operations = 100

	// Manual concurrent testing with set and read operations
	var wg sync.WaitGroup
	// Workers that set values
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				gauge.Set(float64(workerID))
			}
		}(i)
	}
	// Workers that read values
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_ = gauge.Value() // Just read, don't check specific value
			}
		}()
	}
	wg.Wait()

	// The final value should be one of the set values (0 to workers-1)
	final := gauge.Value()
	if final < 0 || final >= float64(workers) {
		t.Errorf("Final gauge value %f should be between 0 and %d", final, workers-1)
	}
}

func TestGauge_LargeValues(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Test with large positive value
	largeValue := 1e15
	gauge.Set(largeValue)

	if gauge.Value() != largeValue {
		t.Errorf("Gauge should handle large positive values, expected %e, got %e",
			largeValue, gauge.Value())
	}

	// Test with large negative value
	gauge.Set(-largeValue)

	if gauge.Value() != -largeValue {
		t.Errorf("Gauge should handle large negative values, expected %e, got %e",
			-largeValue, gauge.Value())
	}
}

func TestGauge_SmallIncrements(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	// Test with very small increments and decrements
	smallValue := 1e-10
	iterations := 1000

	for i := 0; i < iterations; i++ {
		gauge.Add(smallValue)
	}

	for i := 0; i < iterations/2; i++ {
		gauge.Add(-smallValue)
	}

	expected := float64(iterations/2) * smallValue
	result := gauge.Value()

	// Allow for small floating point precision errors
	diff := result - expected
	if diff < 0 {
		diff = -diff
	}

	if diff > 1e-9 { // Tolerance for floating point precision
		t.Errorf("Gauge precision test failed, expected ~%e, got %e (diff: %e)",
			expected, result, diff)
	}
}

func TestGauge_Interface(t *testing.T) {
	registry := metricz.New()
	var g metricz.Gauge = registry.Gauge(TestGaugeKey)

	// Test that all interface methods work correctly
	g.Set(10.0)
	if g.Value() != 10.0 {
		t.Error("Gauge interface Set() failed")
	}

	g.Inc()
	if g.Value() != 11.0 {
		t.Error("Gauge interface Inc() failed")
	}

	g.Dec()
	if g.Value() != 10.0 {
		t.Error("Gauge interface Dec() failed")
	}

	g.Add(5.5)
	if g.Value() != 15.5 {
		t.Error("Gauge interface Add() with positive value failed")
	}

	g.Add(-3.0)
	if g.Value() != 12.5 {
		t.Error("Gauge interface Add() with negative value failed")
	}
}

func TestGaugeNaNInfinityRejection(t *testing.T) {
	registry := metricz.New()
	gauge := registry.Gauge(TestGaugeKey)

	gauge.Set(5.0)
	if gauge.Value() != 5.0 {
		t.Errorf("Expected gauge value 5.0, got %f", gauge.Value())
	}

	// These should be rejected silently
	gauge.Set(math.NaN())
	gauge.Set(math.Inf(1))
	gauge.Set(math.Inf(-1))

	// Value unchanged
	if gauge.Value() != 5.0 {
		t.Errorf("Expected gauge value 5.0 after invalid Set(), got %f", gauge.Value())
	}

	// Test Add() rejection
	gauge.Add(math.NaN())
	gauge.Add(math.Inf(1))
	gauge.Add(math.Inf(-1))

	// Value still unchanged
	if gauge.Value() != 5.0 {
		t.Errorf("Expected gauge value 5.0 after invalid Add(), got %f", gauge.Value())
	}
	if math.IsNaN(gauge.Value()) {
		t.Error("Gauge value should not be NaN after rejecting NaN input")
	}
	if math.IsInf(gauge.Value(), 0) {
		t.Error("Gauge value should not be Inf after rejecting Inf input")
	}
}
