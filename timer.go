package metricz

import (
	"time"

	"github.com/zoobzio/clockz"
)

// Timer interface for timing metrics.
type Timer interface {
	Record(time.Duration)
	Start() *Stopwatch
	// Access underlying histogram data
	Sum() float64
	Count() uint64
	Buckets() ([]float64, []uint64)
}

// timer implements Timer as a duration histogram.
type timer struct {
	histogram *histogram
	clock     clockz.Clock
}

// newTimer creates a new timer with default latency buckets.
// Uses the provided clock for all timing operations.
func newTimer(clock clockz.Clock) *timer {
	return &timer{
		histogram: newHistogram(DefaultLatencyBuckets),
		clock:     clock,
	}
}

// newTimerWithBuckets creates a new timer with custom buckets.
// Uses the provided clock for all timing operations.
func newTimerWithBuckets(buckets []float64, clock clockz.Clock) *timer {
	return &timer{
		histogram: newHistogram(buckets),
		clock:     clock,
	}
}

// Record records a duration in the timer.
func (t *timer) Record(duration time.Duration) {
	// Convert to milliseconds for storage
	t.histogram.Observe(float64(duration.Nanoseconds()) / 1e6)
}

// Start returns a stopwatch for timing operations.
// Uses injected clock for deterministic timing.
func (t *timer) Start() *Stopwatch {
	return &Stopwatch{
		start: t.clock.Now(),
		timer: t,
	}
}

// Sum returns the sum of all recorded durations in milliseconds.
func (t *timer) Sum() float64 {
	return t.histogram.Sum()
}

// Count returns the total number of recorded durations.
func (t *timer) Count() uint64 {
	return t.histogram.Count()
}

// Buckets returns the bucket boundaries and counts.
func (t *timer) Buckets() (buckets []float64, counts []uint64) {
	return t.histogram.Buckets()
}

// Stopwatch provides convenient timing functionality.
type Stopwatch struct {
	start time.Time
	timer *timer
}

// Stop records the elapsed time since Start().
// Uses injected clock for deterministic timing.
func (s *Stopwatch) Stop() {
	s.timer.Record(s.timer.clock.Now().Sub(s.start))
}
