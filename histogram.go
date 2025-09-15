package metricz

import (
	"math"
	"slices"
	"sort"
	"sync"
)

// Histogram interface for distributional metrics.
type Histogram interface {
	Observe(float64)
	Buckets() ([]float64, []uint64)
	Sum() float64
	Count() uint64
	Overflow() uint64
}

// histogram implements Histogram using bucket arrays.
type histogram struct {
	buckets  []float64
	counts   []uint64
	mu       sync.RWMutex
	sum      float64
	overflow uint64 // Count values > largest bucket
	total    uint64
}

// newHistogram creates a new histogram with the given buckets.
func newHistogram(buckets []float64) *histogram {
	return &histogram{
		buckets:  slices.Clone(buckets),
		counts:   make([]uint64, len(buckets)),
		overflow: 0, // Initialize overflow counter
	}
}

// Observe records a value in the histogram.
func (h *histogram) Observe(value float64) {
	// Validate input
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.total++

	// Find bucket using binary search and increment
	// SearchFloat64s returns the first index where h.buckets[i] >= value
	i := sort.SearchFloat64s(h.buckets, value)
	if i < len(h.buckets) {
		// Found a bucket where h.buckets[i] >= value, which means value <= h.buckets[i]
		h.counts[i]++
		return
	}

	// Handle overflow values
	h.overflow++
}

// Buckets returns the bucket boundaries and counts.
func (h *histogram) Buckets() (buckets []float64, counts []uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Include implicit +Inf bucket for overflow
	buckets = make([]float64, len(h.buckets)+1)
	counts = make([]uint64, len(h.counts)+1)

	copy(buckets, h.buckets)
	copy(counts, h.counts)

	// Add overflow bucket as +Inf
	buckets[len(buckets)-1] = math.Inf(1)
	counts[len(counts)-1] = h.overflow

	return
}

// Sum returns the sum of all observed values.
func (h *histogram) Sum() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sum
}

// Count returns the total number of observed values.
func (h *histogram) Count() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.total
}

// Overflow returns the number of values that exceeded the largest bucket.
func (h *histogram) Overflow() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.overflow
}
