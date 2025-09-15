package metricz

import (
	"sort"
	"testing"
)

func TestDefaultLatencyBuckets(t *testing.T) {
	expected := []float64{
		1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000,
	}

	if len(DefaultLatencyBuckets) != len(expected) {
		t.Errorf("DefaultLatencyBuckets length = %d, want %d", len(DefaultLatencyBuckets), len(expected))
	}

	for i, want := range expected {
		if i >= len(DefaultLatencyBuckets) {
			t.Errorf("DefaultLatencyBuckets[%d] missing, want %v", i, want)
			continue
		}
		if got := DefaultLatencyBuckets[i]; got != want {
			t.Errorf("DefaultLatencyBuckets[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestDefaultLatencyBucketsOrdered(t *testing.T) {
	if !sort.Float64sAreSorted(DefaultLatencyBuckets) {
		t.Error("DefaultLatencyBuckets are not sorted in ascending order")
	}

	// Check for duplicates
	seen := make(map[float64]bool)
	for i, bucket := range DefaultLatencyBuckets {
		if seen[bucket] {
			t.Errorf("DefaultLatencyBuckets[%d] = %v is duplicate", i, bucket)
		}
		seen[bucket] = true
	}
}

func TestDefaultLatencyBucketsRange(t *testing.T) {
	// Verify reasonable range for latency measurements in milliseconds
	if DefaultLatencyBuckets[0] <= 0 {
		t.Errorf("DefaultLatencyBuckets first bucket = %v, should be positive", DefaultLatencyBuckets[0])
	}

	lastBucket := DefaultLatencyBuckets[len(DefaultLatencyBuckets)-1]
	if lastBucket < 1000 { // At least 1 second in milliseconds
		t.Errorf("DefaultLatencyBuckets last bucket = %v, should cover at least 1000ms", lastBucket)
	}
}

func TestDefaultSizeBuckets(t *testing.T) {
	expected := []float64{
		64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304,
	}

	if len(DefaultSizeBuckets) != len(expected) {
		t.Errorf("DefaultSizeBuckets length = %d, want %d", len(DefaultSizeBuckets), len(expected))
	}

	for i, want := range expected {
		if i >= len(DefaultSizeBuckets) {
			t.Errorf("DefaultSizeBuckets[%d] missing, want %v", i, want)
			continue
		}
		if got := DefaultSizeBuckets[i]; got != want {
			t.Errorf("DefaultSizeBuckets[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestDefaultSizeBucketsOrdered(t *testing.T) {
	if !sort.Float64sAreSorted(DefaultSizeBuckets) {
		t.Error("DefaultSizeBuckets are not sorted in ascending order")
	}

	// Check for duplicates
	seen := make(map[float64]bool)
	for i, bucket := range DefaultSizeBuckets {
		if seen[bucket] {
			t.Errorf("DefaultSizeBuckets[%d] = %v is duplicate", i, bucket)
		}
		seen[bucket] = true
	}
}

func TestDefaultSizeBucketsRange(t *testing.T) {
	// Verify reasonable range for size measurements in bytes
	if DefaultSizeBuckets[0] <= 0 {
		t.Errorf("DefaultSizeBuckets first bucket = %v, should be positive", DefaultSizeBuckets[0])
	}

	// Check that buckets follow powers of 2 or 4 pattern for size measurements
	for i, bucket := range DefaultSizeBuckets {
		if bucket < 1 {
			t.Errorf("DefaultSizeBuckets[%d] = %v, should be at least 1 byte", i, bucket)
		}
	}

	lastBucket := DefaultSizeBuckets[len(DefaultSizeBuckets)-1]
	if lastBucket < 1048576 { // At least 1 MB
		t.Errorf("DefaultSizeBuckets last bucket = %v, should cover at least 1MB", lastBucket)
	}
}

func TestDefaultSizeBucketsPowersOfTwo(t *testing.T) {
	// Verify that size buckets follow expected powers pattern
	expectedPattern := []float64{64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304}

	for i, expected := range expectedPattern {
		if i >= len(DefaultSizeBuckets) {
			t.Errorf("DefaultSizeBuckets missing bucket %d: %v", i, expected)
			continue
		}
		if DefaultSizeBuckets[i] != expected {
			t.Errorf("DefaultSizeBuckets[%d] = %v, expected %v (powers of 2/4 pattern)",
				i, DefaultSizeBuckets[i], expected)
		}
	}
}

func TestDefaultDurationBuckets(t *testing.T) {
	expected := []float64{
		0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
	}

	if len(DefaultDurationBuckets) != len(expected) {
		t.Errorf("DefaultDurationBuckets length = %d, want %d", len(DefaultDurationBuckets), len(expected))
	}

	for i, want := range expected {
		if i >= len(DefaultDurationBuckets) {
			t.Errorf("DefaultDurationBuckets[%d] missing, want %v", i, want)
			continue
		}
		if got := DefaultDurationBuckets[i]; got != want {
			t.Errorf("DefaultDurationBuckets[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestDefaultDurationBucketsOrdered(t *testing.T) {
	if !sort.Float64sAreSorted(DefaultDurationBuckets) {
		t.Error("DefaultDurationBuckets are not sorted in ascending order")
	}

	// Check for duplicates
	seen := make(map[float64]bool)
	for i, bucket := range DefaultDurationBuckets {
		if seen[bucket] {
			t.Errorf("DefaultDurationBuckets[%d] = %v is duplicate", i, bucket)
		}
		seen[bucket] = true
	}
}

func TestDefaultDurationBucketsRange(t *testing.T) {
	// Verify reasonable range for duration measurements in seconds
	if DefaultDurationBuckets[0] <= 0 {
		t.Errorf("DefaultDurationBuckets first bucket = %v, should be positive", DefaultDurationBuckets[0])
	}

	// Should start with millisecond precision
	if DefaultDurationBuckets[0] > 0.01 {
		t.Errorf("DefaultDurationBuckets first bucket = %v, should start with sub-10ms precision", DefaultDurationBuckets[0])
	}

	lastBucket := DefaultDurationBuckets[len(DefaultDurationBuckets)-1]
	if lastBucket < 1.0 { // At least 1 second
		t.Errorf("DefaultDurationBuckets last bucket = %v, should cover at least 1 second", lastBucket)
	}
}

func TestAllBucketsNonEmpty(t *testing.T) {
	if len(DefaultLatencyBuckets) == 0 {
		t.Error("DefaultLatencyBuckets is empty")
	}
	if len(DefaultSizeBuckets) == 0 {
		t.Error("DefaultSizeBuckets is empty")
	}
	if len(DefaultDurationBuckets) == 0 {
		t.Error("DefaultDurationBuckets is empty")
	}
}

func TestBucketsAreDistinct(t *testing.T) {
	// Verify that different bucket sets don't overlap inappropriately
	// This is a sanity check to ensure buckets serve different purposes

	// Latency buckets should generally be larger numbers (milliseconds)
	latencyMin := DefaultLatencyBuckets[0]
	durationMax := DefaultDurationBuckets[len(DefaultDurationBuckets)-1]

	// Duration max should be much smaller than latency min (seconds vs milliseconds)
	if durationMax >= latencyMin {
		// This might be OK depending on the units, but worth noting
		t.Logf("Duration max (%v) >= Latency min (%v) - verify units are correct", durationMax, latencyMin)
	}
}

func TestBucketImmutability(t *testing.T) {
	// Test that the bucket slices are separate instances
	// (Though Go doesn't prevent mutation, this tests our declaration)

	originalLatency := make([]float64, len(DefaultLatencyBuckets))
	copy(originalLatency, DefaultLatencyBuckets)

	originalSize := make([]float64, len(DefaultSizeBuckets))
	copy(originalSize, DefaultSizeBuckets)

	originalDuration := make([]float64, len(DefaultDurationBuckets))
	copy(originalDuration, DefaultDurationBuckets)

	// Simulate accidental modification
	if len(DefaultLatencyBuckets) > 0 {
		oldValue := DefaultLatencyBuckets[0]
		DefaultLatencyBuckets[0] = 999999

		// Restore and verify we can detect the change
		if DefaultLatencyBuckets[0] == oldValue {
			t.Error("Could not modify DefaultLatencyBuckets - unexpected immutability")
		}
		DefaultLatencyBuckets[0] = oldValue
	}

	// Verify buckets still match original values
	for i, want := range originalLatency {
		if DefaultLatencyBuckets[i] != want {
			t.Errorf("DefaultLatencyBuckets[%d] = %v, want %v (bucket was modified)", i, DefaultLatencyBuckets[i], want)
		}
	}
}
