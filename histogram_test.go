package metricz_test

import (
	"math"
	"testing"

	"github.com/zoobzio/metricz"
	metricstesting "github.com/zoobzio/metricz/testing"
)

func TestHistogram_Observe(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	buckets := []float64{1, 5, 10, 50, 100}
	hist := registry.Histogram(TestHistKey, buckets)

	// Initial state
	if hist.Count() != 0 {
		t.Errorf("Initial histogram count should be 0, got %d", hist.Count())
	}
	if hist.Sum() != 0 {
		t.Errorf("Initial histogram sum should be 0, got %f", hist.Sum())
	}

	// Observe a value
	hist.Observe(3.0)

	if hist.Count() != 1 {
		t.Errorf("After one observation, count should be 1, got %d", hist.Count())
	}
	if hist.Sum() != 3.0 {
		t.Errorf("After observing 3.0, sum should be 3.0, got %f", hist.Sum())
	}

	// Observe another value
	hist.Observe(7.0)

	if hist.Count() != 2 {
		t.Errorf("After two observations, count should be 2, got %d", hist.Count())
	}
	if hist.Sum() != 10.0 {
		t.Errorf("After observing 3.0 and 7.0, sum should be 10.0, got %f", hist.Sum())
	}
}

func TestHistogram_Buckets(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	buckets := []float64{1, 5, 10, 50, 100}
	hist := registry.Histogram(TestHistKey, buckets)

	// Test observations in different buckets
	hist.Observe(0.5)   // bucket 1
	hist.Observe(3.0)   // bucket 5
	hist.Observe(7.0)   // bucket 10
	hist.Observe(25.0)  // bucket 50
	hist.Observe(75.0)  // bucket 100
	hist.Observe(200.0) // beyond all buckets

	returnedBuckets, counts := hist.Buckets()

	// Verify bucket boundaries are correct (including overflow bucket)
	expectedBuckets := len(buckets) + 1 // +1 for overflow bucket
	if len(returnedBuckets) != expectedBuckets {
		t.Errorf("Expected %d buckets (including overflow), got %d", expectedBuckets, len(returnedBuckets))
	}

	for i, expected := range buckets {
		if returnedBuckets[i] != expected {
			t.Errorf("Bucket %d: expected %f, got %f", i, expected, returnedBuckets[i])
		}
	}

	// Verify counts (including overflow bucket)
	expectedCounts := []uint64{1, 1, 1, 1, 1, 1} // One observation per bucket + 1 overflow

	for i, expected := range expectedCounts {
		if counts[i] != expected {
			t.Errorf("Bucket %d count: expected %d, got %d", i, expected, counts[i])
		}
	}

	// Verify total count
	if hist.Count() != 6 {
		t.Errorf("Total count should be 6, got %d", hist.Count())
	}
}

func TestHistogram_BucketAssignment(t *testing.T) {
	buckets := []float64{1, 5, 10}

	cases := []struct {
		value  float64
		bucket int // Expected bucket index, -1 means no bucket
	}{
		{0.5, 0},   // <= 1
		{1.0, 0},   // <= 1 (boundary)
		{2.0, 1},   // <= 5
		{5.0, 1},   // <= 5 (boundary)
		{7.5, 2},   // <= 10
		{10.0, 2},  // <= 10 (boundary)
		{15.0, -1}, // > 10 (no bucket)
	}

	for _, tc := range cases {
		// Create fresh registry for test isolation
		registry := metricstesting.NewTestRegistry(t)
		hist := registry.Histogram(TestHistKey, buckets)
		hist.Observe(tc.value)

		_, counts := hist.Buckets()

		// Check that exactly one bucket has count 1
		foundBucket := -1
		totalCount := uint64(0)
		for i, count := range counts {
			totalCount += count
			if count == 1 {
				if foundBucket != -1 {
					t.Errorf("Value %f: multiple buckets have count 1", tc.value)
				}
				foundBucket = i
			}
		}

		if tc.bucket == -1 {
			// Value should be in overflow bucket (last bucket)
			if counts[len(counts)-1] != 1 {
				t.Errorf("Value %f: expected overflow bucket assignment, but found count %d",
					tc.value, counts[len(counts)-1])
			}
		} else {
			// Value should be in the expected bucket
			if foundBucket != tc.bucket {
				t.Errorf("Value %f: expected bucket %d, got bucket %d",
					tc.value, tc.bucket, foundBucket)
			}
		}
	}
}

func TestHistogram_ConcurrentObserve(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	buckets := []float64{1, 5, 10, 50, 100}
	hist := registry.Histogram(TestHistKey, buckets)

	const workers = 100
	const observations = 100

	// Use GenerateLoad for standardized concurrent testing
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers,
		Operations: observations,
		Operation: func(workerID, _ int) {
			value := float64(workerID % 20) // Values 0-19, spread across buckets
			hist.Observe(value)
		},
	})

	expectedCount := uint64(workers * observations)
	if hist.Count() != expectedCount {
		t.Errorf("Expected total count %d, got %d", expectedCount, hist.Count())
	}

	// Sum should be calculable
	expectedSum := 0.0
	for i := 0; i < workers; i++ {
		value := float64(i % 20)
		expectedSum += value * float64(observations)
	}

	if hist.Sum() != expectedSum {
		t.Errorf("Expected sum %f, got %f", expectedSum, hist.Sum())
	}
}

func TestHistogram_EdgeCases(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	buckets := []float64{1, 5, 10}
	hist := registry.Histogram(TestHistKey, buckets)

	// Test negative values
	hist.Observe(-5.0)
	if hist.Count() != 1 {
		t.Error("Histogram should accept negative values")
	}
	if hist.Sum() != -5.0 {
		t.Error("Histogram sum should include negative values")
	}

	// Test zero
	hist.Observe(0.0)
	if hist.Count() != 2 {
		t.Error("Histogram should accept zero")
	}

	// Test very large values
	hist.Observe(1e6)
	if hist.Count() != 3 {
		t.Error("Histogram should accept very large values")
	}
}

func TestHistogram_EmptyBuckets(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	// Test with empty buckets array
	hist := registry.Histogram(TestHistKey, []float64{})

	hist.Observe(5.0)

	if hist.Count() != 1 {
		t.Error("Empty bucket histogram should still count observations")
	}
	if hist.Sum() != 5.0 {
		t.Error("Empty bucket histogram should still sum observations")
	}

	buckets, counts := hist.Buckets()
	// Even empty bucket histogram has overflow bucket
	if len(buckets) != 1 || len(counts) != 1 {
		t.Errorf("Empty bucket histogram should return 1 bucket (overflow), got %d buckets, %d counts", len(buckets), len(counts))
	}
	if !math.IsInf(buckets[0], 1) {
		t.Error("Empty bucket histogram should have overflow bucket as +Inf")
	}
	if counts[0] != 1 { // The one observation went to overflow
		t.Errorf("Empty bucket histogram should have 1 count in overflow, got %d", counts[0])
	}
}

func TestHistogram_BucketImmutability(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	originalBuckets := []float64{1, 5, 10}
	hist := registry.Histogram(TestHistKey, originalBuckets)

	// Modify the original buckets array
	originalBuckets[0] = 999.0

	// Get buckets from histogram
	returnedBuckets, _ := hist.Buckets()

	if returnedBuckets[0] == 999.0 {
		t.Error("Histogram buckets should be immutable from external modification")
	}

	// Modify the returned buckets array
	returnedBuckets[1] = 888.0

	// Get buckets again
	newBuckets, _ := hist.Buckets()

	if newBuckets[1] == 888.0 {
		t.Error("Returned bucket arrays should be copies, not references")
	}

	if newBuckets[1] != 5.0 {
		t.Error("Histogram internal buckets should be unchanged")
	}
}

func TestHistogram_Interface(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	buckets := []float64{1, 5, 10, 50, 100}
	var h metricz.Histogram = registry.Histogram(TestHistKey, buckets)

	// Test interface methods
	h.Observe(7.5)

	if h.Count() != 1 {
		t.Error("Histogram interface Count() failed")
	}

	if h.Sum() != 7.5 {
		t.Error("Histogram interface Sum() failed")
	}

	if h.Overflow() != 0 {
		t.Error("Histogram interface Overflow() should be 0 initially")
	}

	returnedBuckets, counts := h.Buckets()
	if len(returnedBuckets) != 6 { // 5 original + 1 overflow
		t.Errorf("Histogram interface Buckets() failed to return correct bucket count, expected 6, got %d", len(returnedBuckets))
	}

	// Value 7.5 should be in bucket index 2 (bucket <= 10)
	if counts[2] != 1 {
		t.Error("Histogram interface bucket assignment failed")
	}
}

func TestHistogram_RepeatedBuckets(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	// Test with duplicate bucket values
	buckets := []float64{1, 5, 5, 10, 10, 10}
	hist := registry.Histogram(TestHistKey, buckets)

	// Observe value that fits in duplicate buckets
	hist.Observe(5.0)

	_, counts := hist.Buckets()

	// Should be assigned to first matching bucket (index 1)
	if counts[1] != 1 {
		t.Error("Value should be assigned to first matching bucket")
	}

	// Other duplicate buckets should remain empty
	for i, count := range counts {
		if i != 1 && count != 0 {
			t.Errorf("Duplicate bucket %d should be empty, got count %d", i, count)
		}
	}
}

func TestHistogramOverflowBucket(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	h := registry.Histogram(TestHistKey, []float64{1.0, 5.0, 10.0})

	// Observe values including overflow
	h.Observe(0.5)  // bucket 0
	h.Observe(3.0)  // bucket 1
	h.Observe(15.0) // overflow
	h.Observe(20.0) // overflow

	buckets, counts := h.Buckets()

	// Verify overflow bucket exists
	if len(buckets) != 4 {
		t.Errorf("Expected 4 buckets (3 defined + 1 overflow), got %d", len(buckets))
	}
	if !math.IsInf(buckets[3], 1) {
		t.Errorf("Expected overflow bucket to be +Inf, got %f", buckets[3])
	}
	if counts[3] != 2 {
		t.Errorf("Expected overflow bucket count 2, got %d", counts[3])
	}

	// Verify total count matches
	if h.Count() != 4 {
		t.Errorf("Expected total count 4, got %d", h.Count())
	}
}

func TestHistogram_Overflow(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	h := registry.Histogram(TestHistKey, []float64{1.0, 5.0, 10.0})

	// Initial overflow should be zero
	if h.Overflow() != 0 {
		t.Errorf("Expected initial overflow 0, got %d", h.Overflow())
	}

	// Observe values within buckets
	h.Observe(0.5) // bucket 0
	h.Observe(3.0) // bucket 1
	h.Observe(7.0) // bucket 2

	// Overflow should still be zero
	if h.Overflow() != 0 {
		t.Errorf("Expected overflow 0 after in-bucket observations, got %d", h.Overflow())
	}

	// Observe overflow values
	h.Observe(15.0) // overflow
	if h.Overflow() != 1 {
		t.Errorf("Expected overflow 1 after first overflow observation, got %d", h.Overflow())
	}

	h.Observe(25.0)  // overflow
	h.Observe(100.0) // overflow
	if h.Overflow() != 3 {
		t.Errorf("Expected overflow 3 after three overflow observations, got %d", h.Overflow())
	}

	// Verify overflow matches bucket count
	_, counts := h.Buckets()
	overflowBucketCount := counts[len(counts)-1] // Last bucket is overflow
	if h.Overflow() != overflowBucketCount {
		t.Errorf("Overflow() returned %d but overflow bucket count is %d", h.Overflow(), overflowBucketCount)
	}

	// Verify total count includes overflow
	expectedTotal := uint64(3 + 3) // 3 in-bucket + 3 overflow
	if h.Count() != expectedTotal {
		t.Errorf("Expected total count %d, got %d", expectedTotal, h.Count())
	}
}

func TestHistogramNaNInfinityRejection(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	h := registry.Histogram(TestHistKey, []float64{1.0, 5.0, 10.0})

	h.Observe(5.0)
	if h.Count() != 1 {
		t.Errorf("Expected count 1 after valid observation, got %d", h.Count())
	}
	if h.Sum() != 5.0 {
		t.Errorf("Expected sum 5.0 after valid observation, got %f", h.Sum())
	}

	// These should be rejected silently
	h.Observe(math.NaN())
	h.Observe(math.Inf(1))
	h.Observe(math.Inf(-1))

	// Count and sum unchanged
	if h.Count() != 1 {
		t.Errorf("Expected count 1 after invalid observations, got %d", h.Count())
	}
	if h.Sum() != 5.0 {
		t.Errorf("Expected sum 5.0 after invalid observations, got %f", h.Sum())
	}
	if math.IsNaN(h.Sum()) {
		t.Error("Sum should not be NaN after rejecting NaN input")
	}
	if math.IsInf(h.Sum(), 0) {
		t.Error("Sum should not be Inf after rejecting Inf input")
	}
}
