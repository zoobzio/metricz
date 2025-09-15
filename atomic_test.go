package metricz

import (
	"math"
	"runtime"
	"sync"
	"testing"
)

func TestNewAtomicFloat64(t *testing.T) {
	a := newAtomicFloat64()
	if a == nil {
		t.Fatal("newAtomicFloat64() returned nil")
	}
	if got := a.get(); got != 0.0 {
		t.Errorf("newAtomicFloat64().get() = %v, want 0.0", got)
	}
}

func TestAtomicFloat64_Set(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{"zero", 0.0},
		{"positive", 42.5},
		{"negative", -17.3},
		{"large", 1e15},
		{"small", 1e-15},
		{"max", math.MaxFloat64},
		{"smallest_normal", math.SmallestNonzeroFloat64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAtomicFloat64()
			a.set(tt.value)
			if got := a.get(); got != tt.value {
				t.Errorf("set(%v) then get() = %v, want %v", tt.value, got, tt.value)
			}
		})
	}
}

func TestAtomicFloat64_SetSpecialValues(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{"positive_infinity", math.Inf(1)},
		{"negative_infinity", math.Inf(-1)},
		{"nan", math.NaN()},
		{"negative_zero", math.Copysign(0, -1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAtomicFloat64()
			a.set(tt.value)
			got := a.get()

			if tt.name == "nan" {
				if !math.IsNaN(got) {
					t.Errorf("set(NaN) then get() = %v, want NaN", got)
				}
			} else {
				if got != tt.value {
					t.Errorf("set(%v) then get() = %v, want %v", tt.value, got, tt.value)
				}
			}
		})
	}
}

func TestAtomicFloat64_Add(t *testing.T) {
	tests := []struct {
		name     string
		initial  float64
		delta    float64
		expected float64
	}{
		{"zero_plus_zero", 0.0, 0.0, 0.0},
		{"zero_plus_positive", 0.0, 5.5, 5.5},
		{"zero_plus_negative", 0.0, -3.2, -3.2},
		{"positive_plus_positive", 10.0, 5.0, 15.0},
		{"positive_plus_negative", 10.0, -3.0, 7.0},
		{"negative_plus_positive", -5.0, 8.0, 3.0},
		{"negative_plus_negative", -5.0, -3.0, -8.0},
		{"large_numbers", 1e10, 1e9, 1.1e10},
		{"precision_boundary", 0.1, 0.2, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAtomicFloat64()
			a.set(tt.initial)
			a.add(tt.delta)
			got := a.get()

			// Use a small epsilon for floating point comparison
			const epsilon = 1e-14
			if math.Abs(got-tt.expected) > epsilon {
				t.Errorf("set(%v) then add(%v) = %v, want %v", tt.initial, tt.delta, got, tt.expected)
			}
		})
	}
}

func TestAtomicFloat64_AddSpecialValues(t *testing.T) {
	tests := []struct {
		validate func(float64) bool
		name     string
		initial  float64
		delta    float64
	}{
		{
			func(result float64) bool { return math.IsInf(result, 1) },
			"add_to_infinity",
			math.Inf(1),
			5.0,
		},
		{
			func(result float64) bool { return math.IsInf(result, 1) },
			"add_infinity_to_finite",
			10.0,
			math.Inf(1),
		},
		{
			math.IsNaN,
			"add_to_nan",
			math.NaN(),
			5.0,
		},
		{
			math.IsNaN,
			"add_nan_to_finite",
			10.0,
			math.NaN(),
		},
		{
			math.IsNaN,
			"infinity_minus_infinity",
			math.Inf(1),
			math.Inf(-1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAtomicFloat64()
			a.set(tt.initial)
			a.add(tt.delta)
			got := a.get()

			if !tt.validate(got) {
				t.Errorf("set(%v) then add(%v) = %v, validation failed", tt.initial, tt.delta, got)
			}
		})
	}
}

func TestAtomicFloat64_Get(t *testing.T) {
	a := newAtomicFloat64()

	// Test that get returns correct initial value
	if got := a.get(); got != 0.0 {
		t.Errorf("initial get() = %v, want 0.0", got)
	}

	// Test that get is idempotent
	value := 42.5
	a.set(value)
	got1 := a.get()
	got2 := a.get()

	if got1 != value {
		t.Errorf("first get() = %v, want %v", got1, value)
	}
	if got2 != value {
		t.Errorf("second get() = %v, want %v", got2, value)
	}
	if got1 != got2 {
		t.Errorf("get() not idempotent: %v != %v", got1, got2)
	}
}

func TestAtomicFloat64_ConcurrentSet(t *testing.T) {
	a := newAtomicFloat64()
	const numGoroutines = 100
	var wg sync.WaitGroup

	// All goroutines set the same value
	targetValue := 42.0
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			a.set(targetValue)
		}()
	}

	wg.Wait()

	if got := a.get(); got != targetValue {
		t.Errorf("concurrent set resulted in %v, want %v", got, targetValue)
	}
}

func TestAtomicFloat64_ConcurrentAdd(t *testing.T) {
	a := newAtomicFloat64()
	const numGoroutines = 100
	const addValue = 1.0
	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			a.add(addValue)
		}()
	}

	wg.Wait()

	expected := float64(numGoroutines) * addValue
	if got := a.get(); got != expected {
		t.Errorf("concurrent add resulted in %v, want %v", got, expected)
	}
}

func TestAtomicFloat64_ConcurrentMixed(t *testing.T) {
	a := newAtomicFloat64()
	const numGoroutines = 50
	var wg sync.WaitGroup

	wg.Add(numGoroutines * 3) // 3 operations per iteration

	for i := 0; i < numGoroutines; i++ {
		// Concurrent set, add, and get operations
		go func(val float64) {
			defer wg.Done()
			a.set(val)
		}(float64(i))

		go func() {
			defer wg.Done()
			a.add(1.0)
		}()

		go func() {
			defer wg.Done()
			_ = a.get()
		}()
	}

	wg.Wait()

	// Just verify that we can still get a value (no crashes/deadlocks)
	final := a.get()
	if math.IsNaN(final) {
		t.Error("concurrent mixed operations resulted in NaN")
	}
}

func TestAtomicFloat64_CASLoopBehavior(t *testing.T) {
	// Test that the CAS loop in add() works correctly under contention
	a := newAtomicFloat64()
	a.set(100.0)

	const numGoroutines = 1000
	const addValue = 0.001
	var wg sync.WaitGroup

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// Force more contention by yielding
			runtime.Gosched()
			a.add(addValue)
		}()
	}

	wg.Wait()

	expected := 100.0 + (float64(numGoroutines) * addValue)
	got := a.get()

	// Allow for tiny floating point precision errors
	const epsilon = 1e-10
	if math.Abs(got-expected) > epsilon {
		t.Errorf("CAS loop under contention: got %v, want %v (diff: %v)", got, expected, math.Abs(got-expected))
	}
}

func TestAtomicFloat64_PrecisionBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		value1 float64
		value2 float64
	}{
		{"float64_precision_limit", 1.0, 1.0 + 1e-15},
		{"large_small_addition", 1e15, 1.0},
		{"denormal_numbers", math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAtomicFloat64()
			a.set(tt.value1)
			a.add(tt.value2)

			// Just verify the operation completes without panic
			result := a.get()
			if math.IsNaN(result) && !math.IsNaN(tt.value1) && !math.IsNaN(tt.value2) {
				t.Errorf("precision boundary test resulted in unexpected NaN")
			}
		})
	}
}
