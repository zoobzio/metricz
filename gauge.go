package metricz

import "math"

// Gauge interface for metrics that can increase and decrease.
type Gauge interface {
	Set(float64)
	Add(float64)
	Inc()
	Dec()
	Value() float64
}

// gaugeImpl implements Gauge allowing both positive and negative changes.
type gaugeImpl struct {
	atomic *atomicFloat64
}

// newGauge creates a new gauge.
func newGauge() *gaugeImpl {
	return &gaugeImpl{
		atomic: newAtomicFloat64(),
	}
}

// Set sets the gauge to a specific value.
func (g *gaugeImpl) Set(value float64) {
	// Validate input
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	g.atomic.set(value)
}

// Add increases/decreases the gauge by delta (allows negative values).
func (g *gaugeImpl) Add(delta float64) {
	// Validate input
	if math.IsNaN(delta) || math.IsInf(delta, 0) {
		return
	}
	g.atomic.add(delta)
}

// Inc increments the gauge by 1.
func (g *gaugeImpl) Inc() {
	g.Add(1.0)
}

// Dec decrements the gauge by 1.
func (g *gaugeImpl) Dec() {
	g.Add(-1.0)
}

// Value returns the current gauge value.
func (g *gaugeImpl) Value() float64 {
	return g.atomic.get()
}
