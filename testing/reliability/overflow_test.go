package reliability

import (
	"math"
	gotesting "testing"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// Test metric keys - the RIGHT way to use Key type.
const (
	OverflowTestKey metricz.Key = "test"
	Test2Key        metricz.Key = "test2"
	NaNCounterKey   metricz.Key = "nan_counter"
	NaNGaugeKey     metricz.Key = "nan_gauge"
	NaNHistKey      metricz.Key = "nan_hist"
	InfCounterKey   metricz.Key = "inf_counter"
	InfGaugeKey     metricz.Key = "inf_gauge"
	InfHistKey      metricz.Key = "inf_hist"
	NegInfGaugeKey  metricz.Key = "neg_inf_gauge"
	EmptyKey        metricz.Key = "empty"
	SingleKey       metricz.Key = "single"
)

// TestHistogramValueOverflow verifies overflow bucket captures values exceeding all buckets.
func TestHistogramValueOverflow(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	hist := registry.Histogram(OverflowTestKey, []float64{1, 10, 100})

	// Observe values within buckets
	hist.Observe(0.5) // Should go in bucket 1
	hist.Observe(5)   // Should go in bucket 10
	hist.Observe(50)  // Should go in bucket 100

	// Observe values EXCEEDING all buckets - should go to overflow bucket
	hist.Observe(1000)            // Exceeds all buckets -> overflow
	hist.Observe(10000)           // Exceeds all buckets -> overflow
	hist.Observe(math.MaxFloat64) // Extreme overflow -> overflow

	// Check total count
	if hist.Count() != 6 {
		t.Errorf("Count should be 6 (all observations), got %d", hist.Count())
	}

	// Check sum includes overflow values
	expectedMinSum := 0.5 + 5 + 50 + 1000 + 10000 // Ignoring MaxFloat64 for comparison
	if hist.Sum() < expectedMinSum {
		t.Errorf("Sum should include overflow values, got %f", hist.Sum())
	}

	// FIXED: Check bucket counts including overflow bucket
	_, counts := hist.Buckets()
	totalInBuckets := uint64(0)
	for _, count := range counts {
		totalInBuckets += count
	}

	// With overflow bucket fix, ALL 6 values should be in buckets
	if totalInBuckets != 6 {
		t.Errorf("Expected all 6 values in buckets (including overflow), got %d", totalInBuckets)
	} else {
		t.Log("VERIFIED: Overflow bucket captures values exceeding all buckets")
		t.Log("No data loss: All 6 observations accounted for in buckets")
	}

	// Verify overflow bucket specifically has 3 values
	if len(counts) != 4 { // 3 original buckets + 1 overflow
		t.Errorf("Expected 4 buckets (3 + overflow), got %d", len(counts))
	}
	if counts[3] != 3 { // Overflow bucket should have 3 values
		t.Errorf("Expected 3 values in overflow bucket, got %d", counts[3])
	}
}

// TestFloat64PrecisionLoss documents expected precision loss with large counter values.
func TestFloat64PrecisionLoss(t *gotesting.T) {
	counter := NewTestRegistry(t).Counter(OverflowTestKey)

	// Set counter to near the precision limit of float64
	// 2^53 is the largest integer that can be exactly represented
	largeValue := math.Pow(2, 53) - 100
	counter.Add(largeValue)

	initial := counter.Value()

	// Try to increment by 1 repeatedly
	for i := 0; i < 10; i++ {
		counter.Inc()
	}

	expected := initial + 10
	actual := counter.Value()

	if actual != expected {
		t.Logf("EXPECTED BEHAVIOR: Precision loss detected at high values")
		t.Logf("Expected %f, got %f (lost %f)", expected, actual, expected-actual)
		t.Logf("After 2^53, float64 cannot accurately represent every integer")
	}

	// Now test at the boundary where precision is definitely lost
	counter2 := NewTestRegistry(t).Counter(Test2Key)
	counter2.Add(math.Pow(2, 53)) // Exactly 2^53

	before := counter2.Value()
	counter2.Inc() // Add 1
	after := counter2.Value()

	if after == before {
		t.Log("EXPECTED LIMITATION: Incrementing by 1 has no effect at 2^53 boundary")
		t.Logf("This is a fundamental limitation of float64, not a bug")
		t.Logf("Counter value stuck at: %f", before)
		// This is expected behavior, not an error
	}
}

// TestNaNAndInfinity verifies NaN and Infinity are properly rejected.
func TestNaNAndInfinity(t *gotesting.T) {
	registry := NewTestRegistry(t)

	// Test NaN rejection
	t.Run("NaN rejection", func(t *gotesting.T) {
		counter := registry.Counter(NaNCounterKey)
		gauge := registry.Gauge(NaNGaugeKey)
		hist := registry.Histogram(NaNHistKey, []float64{1, 10, 100})

		// Set initial valid values
		counter.Add(10)
		gauge.Set(20)
		hist.Observe(30)

		// Attempt to add NaN - should be rejected
		counter.Add(math.NaN())
		gauge.Set(math.NaN())
		gauge.Add(math.NaN())
		hist.Observe(math.NaN())

		// FIXED: NaN should be rejected, values should remain valid
		counterVal := counter.Value()
		gaugeVal := gauge.Value()
		histSum := hist.Sum()
		histCount := hist.Count()

		if math.IsNaN(counterVal) {
			t.Error("Counter became NaN - validation failed")
		}
		if counterVal != 10 {
			t.Errorf("Counter should remain 10 after NaN rejection, got %f", counterVal)
		}

		if math.IsNaN(gaugeVal) {
			t.Error("Gauge became NaN - validation failed")
		}
		if gaugeVal != 20 {
			t.Errorf("Gauge should remain 20 after NaN rejection, got %f", gaugeVal)
		}

		if math.IsNaN(histSum) {
			t.Error("Histogram sum became NaN - validation failed")
		}
		if histSum != 30 {
			t.Errorf("Histogram sum should remain 30 after NaN rejection, got %f", histSum)
		}
		if histCount != 1 {
			t.Errorf("Histogram count should remain 1 after NaN rejection, got %d", histCount)
		}

		t.Log("VERIFIED: NaN values are properly rejected by all metric types")
	})

	// Test Infinity rejection
	t.Run("Infinity rejection", func(t *gotesting.T) {
		counter := registry.Counter(InfCounterKey)
		gauge := registry.Gauge(InfGaugeKey)
		hist := registry.Histogram(InfHistKey, []float64{1, 10, 100})

		// Set initial valid values
		counter.Add(100)
		gauge.Set(200)
		hist.Observe(300)

		// Attempt to add positive infinity - should be rejected
		counter.Add(math.Inf(1))
		gauge.Set(math.Inf(1))
		gauge.Add(math.Inf(1))
		hist.Observe(math.Inf(1))

		// FIXED: Infinity should be rejected, values should remain valid
		if math.IsInf(counter.Value(), 1) {
			t.Error("Counter became +Inf - validation failed")
		}
		if counter.Value() != 100 {
			t.Errorf("Counter should remain 100 after +Inf rejection, got %f", counter.Value())
		}

		if math.IsInf(gauge.Value(), 1) {
			t.Error("Gauge became +Inf - validation failed")
		}
		if gauge.Value() != 200 {
			t.Errorf("Gauge should remain 200 after +Inf rejection, got %f", gauge.Value())
		}

		if math.IsInf(hist.Sum(), 1) {
			t.Error("Histogram sum became +Inf - validation failed")
		}
		if hist.Sum() != 300 {
			t.Errorf("Histogram sum should remain 300 after +Inf rejection, got %f", hist.Sum())
		}
		if hist.Count() != 1 {
			t.Errorf("Histogram count should remain 1 after +Inf rejection, got %d", hist.Count())
		}

		// Test negative infinity rejection on gauge
		gauge2 := registry.Gauge(NegInfGaugeKey)
		gauge2.Set(500)
		gauge2.Set(math.Inf(-1))
		gauge2.Add(math.Inf(-1))

		if math.IsInf(gauge2.Value(), -1) {
			t.Error("Gauge became -Inf - validation failed")
		}
		if gauge2.Value() != 500 {
			t.Errorf("Gauge should remain 500 after -Inf rejection, got %f", gauge2.Value())
		}

		t.Log("VERIFIED: Infinity values are properly rejected by all metric types")
	})
}

// TestZeroAndNegativeValues tests edge cases with zero and negative values.
func TestZeroAndNegativeValues(t *gotesting.T) {
	registry := NewTestRegistry(t)

	t.Run("Counter with negatives", func(t *gotesting.T) {
		counter := registry.Counter(OverflowTestKey)

		// Negative values should be ignored
		counter.Add(-10)
		if counter.Value() != 0 {
			t.Error("Counter should ignore negative values")
		}

		// Zero should work
		counter.Add(0)
		if counter.Value() != 0 {
			t.Error("Counter should accept zero")
		}

		// Mix positive and negative
		counter.Add(5)
		counter.Add(-3) // Should be ignored
		counter.Add(2)
		if counter.Value() != 7 {
			t.Errorf("Expected 7 (5+2), got %f", counter.Value())
		}
	})

	t.Run("Gauge with negatives", func(t *gotesting.T) {
		gauge := registry.Gauge(OverflowTestKey)

		// Negative values should work
		gauge.Set(-10)
		if gauge.Value() != -10 {
			t.Error("Gauge should accept negative values")
		}

		gauge.Add(-5)
		if gauge.Value() != -15 {
			t.Error("Gauge should handle negative additions")
		}

		gauge.Dec() // Should subtract 1
		if gauge.Value() != -16 {
			t.Error("Gauge Dec should work with negative values")
		}
	})

	t.Run("Histogram with negatives", func(t *gotesting.T) {
		hist := registry.Histogram(OverflowTestKey, []float64{-10, 0, 10, 100})

		// Observe negative values
		hist.Observe(-15) // Less than -10
		hist.Observe(-5)  // Between -10 and 0
		hist.Observe(5)   // Between 0 and 10
		hist.Observe(50)  // Between 10 and 100
		hist.Observe(150) // Exceeds all buckets -> overflow

		if hist.Count() != 5 {
			t.Errorf("Expected 5 observations, got %d", hist.Count())
		}

		expectedSum := -15 + (-5) + 5 + 50 + 150
		if hist.Sum() != float64(expectedSum) {
			t.Errorf("Expected sum %d, got %f", expectedSum, hist.Sum())
		}

		// Check bucket distribution
		_, counts := hist.Buckets()
		// -15 -> bucket -10 (first value <= -10)
		// -5 -> bucket 0
		// 5 -> bucket 10
		// 50 -> bucket 100
		// 150 -> overflow bucket (FIXED)

		if len(counts) != 5 { // 4 original buckets + 1 overflow
			t.Errorf("Expected 5 buckets (4 + overflow), got %d", len(counts))
		}

		if counts[0] != 1 { // -10 bucket
			t.Errorf("Expected 1 in -10 bucket, got %d", counts[0])
		}
		if counts[1] != 1 { // 0 bucket
			t.Errorf("Expected 1 in 0 bucket, got %d", counts[1])
		}
		if counts[2] != 1 { // 10 bucket
			t.Errorf("Expected 1 in 10 bucket, got %d", counts[2])
		}
		if counts[3] != 1 { // 100 bucket
			t.Errorf("Expected 1 in 100 bucket, got %d", counts[3])
		}
		if counts[4] != 1 { // overflow bucket
			t.Errorf("Expected 1 in overflow bucket, got %d", counts[4])
		}
	})
}

// TestTimerPrecision tests timer precision with very small and large durations.
func TestTimerPrecision(t *gotesting.T) {
	registry := NewTestRegistry(t)
	timer := registry.Timer(OverflowTestKey)

	// Test very small durations (nanoseconds)
	timer.Record(1) // 1 nanosecond
	timer.Record(10)
	timer.Record(100)

	// Values are converted to milliseconds, so these become very small
	if timer.Count() != 3 {
		t.Errorf("Expected 3 recordings, got %d", timer.Count())
	}

	// Sum should be in milliseconds: (1 + 10 + 100) / 1e6
	expectedSum := 111.0 / 1e6
	if math.Abs(timer.Sum()-expectedSum) > 0.000001 {
		t.Errorf("Expected sum ~%f ms, got %f ms", expectedSum, timer.Sum())
	}

	// Test very large duration
	timer2 := registry.Timer(Test2Key)
	timer2.Record(24 * 60 * 60 * 1e9) // 24 hours in nanoseconds

	// Should be 24 * 60 * 60 * 1000 milliseconds
	expectedMs := 24.0 * 60 * 60 * 1000
	if timer2.Sum() != expectedMs {
		t.Errorf("Expected %f ms for 24 hours, got %f ms", expectedMs, timer2.Sum())
	}
}

// TestEmptyHistogramBuckets tests histogram with empty bucket list.
func TestEmptyHistogramBuckets(t *gotesting.T) {
	registry := NewTestRegistry(t)

	// Create histogram with no buckets
	hist := registry.Histogram(EmptyKey, []float64{})

	// Observe values - all should go to overflow bucket since no regular buckets exist
	hist.Observe(1)
	hist.Observe(100)
	hist.Observe(1000)

	if hist.Count() != 3 {
		t.Errorf("Expected count 3, got %d", hist.Count())
	}

	if hist.Sum() != 1101 {
		t.Errorf("Expected sum 1101, got %f", hist.Sum())
	}

	// With overflow bucket fix, should have 1 overflow bucket with all values
	buckets, counts := hist.Buckets()
	if len(buckets) != 1 || len(counts) != 1 {
		t.Errorf("Empty histogram should have 1 overflow bucket, got %d buckets", len(buckets))
	}

	// All 3 values should be in the overflow bucket
	if len(counts) > 0 && counts[0] != 3 {
		t.Errorf("Expected 3 values in overflow bucket, got %d", counts[0])
	}
}

// TestSingleBucketHistogram tests edge case of single bucket with overflow.
func TestSingleBucketHistogram(t *gotesting.T) {
	registry := NewTestRegistry(t)
	hist := registry.Histogram(SingleKey, []float64{100})

	hist.Observe(50)  // Less than 100
	hist.Observe(100) // Exactly 100
	hist.Observe(150) // Greater than 100 -> overflow bucket

	if hist.Count() != 3 {
		t.Errorf("Expected count 3, got %d", hist.Count())
	}

	_, counts := hist.Buckets()
	if len(counts) != 2 { // 1 original bucket + 1 overflow
		t.Errorf("Should have exactly 2 buckets (1 + overflow), got %d", len(counts))
	}

	// Values <= 100 should be in first bucket
	if counts[0] != 2 {
		t.Errorf("Expected 2 values in first bucket, got %d", counts[0])
	}

	// Value > 100 should be in overflow bucket
	if counts[1] != 1 {
		t.Errorf("Expected 1 value in overflow bucket, got %d", counts[1])
	}

	t.Log("VERIFIED: Single bucket histogram correctly uses overflow bucket")
}
