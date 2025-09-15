package metricz

import "math"

// Counter interface for metrics that only increase.
type Counter interface {
	Inc()
	Add(float64)
	Value() float64
}

// counterImpl implements Counter with restriction on negative values.
type counterImpl struct {
	atomic *atomicFloat64
}

// newCounter creates a new counter.
func newCounter() *counterImpl {
	return &counterImpl{
		atomic: newAtomicFloat64(),
	}
}

// Inc increments the counter by 1.
func (c *counterImpl) Inc() {
	c.Add(1.0)
}

// Add increases the counter by delta (only positive values).
func (c *counterImpl) Add(delta float64) {
	// Validate input
	if delta < 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return
	}
	c.atomic.add(delta)
}

// Value returns the current counter value.
func (c *counterImpl) Value() float64 {
	return c.atomic.get()
}
