package metricz

import (
	"math"
	"sync/atomic"
)

// atomicFloat64 provides thread-safe float64 storage using atomic operations.
// This is the shared implementation used by Counter and Gauge.
type atomicFloat64 struct {
	value uint64 // Use uint64 for atomic operations
}

// newAtomicFloat64 creates a new atomic float64 with zero value.
func newAtomicFloat64() *atomicFloat64 {
	return &atomicFloat64{}
}

// set atomically sets the value.
func (a *atomicFloat64) set(value float64) {
	atomic.StoreUint64(&a.value, math.Float64bits(value))
}

// add atomically adds delta to the current value.
func (a *atomicFloat64) add(delta float64) {
	for {
		old := atomic.LoadUint64(&a.value)
		newVal := math.Float64bits(math.Float64frombits(old) + delta)
		if atomic.CompareAndSwapUint64(&a.value, old, newVal) {
			break
		}
	}
}

// get atomically gets the current value.
func (a *atomicFloat64) get() float64 {
	return math.Float64frombits(atomic.LoadUint64(&a.value))
}
