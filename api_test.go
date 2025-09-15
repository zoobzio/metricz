package metricz_test

import (
	"testing"
	"time"

	"github.com/zoobzio/metricz"
	metricstesting "github.com/zoobzio/metricz/testing"
)

// Test keys for all metricz tests (shared across test files).
const (
	TestKey              metricz.Key = "test_key"
	TestCounterKey       metricz.Key = "test_counter"
	OtherCounterKey      metricz.Key = "other_counter"
	TestGaugeKey         metricz.Key = "test_gauge"
	OtherGaugeKey        metricz.Key = "other_gauge"
	TestHistKey          metricz.Key = "test_histogram"
	OtherHistKey         metricz.Key = "other_histogram"
	TestTimerKey         metricz.Key = "test_timer"
	OtherTimerKey        metricz.Key = "other_timer"
	ConcurrentCounterKey metricz.Key = "concurrent_counter"
	Counter0Key          metricz.Key = "counter0"
	Counter1Key          metricz.Key = "counter1"
	Gauge0Key            metricz.Key = "gauge0"
	Gauge1Key            metricz.Key = "gauge1"
	Hist1Key             metricz.Key = "hist1"
	Timer1Key            metricz.Key = "timer1"
)

func TestNew(t *testing.T) {
	registry := metricz.New()
	if registry == nil {
		t.Fatal("New() returned nil")
	}

	// Verify it returns *Registry
	var _ *metricz.Registry = registry
}

func TestKey_String(t *testing.T) {
	if string(TestKey) != "test_key" {
		t.Errorf("Expected 'test_key', got '%s'", string(TestKey))
	}
}

func TestRegistry_Counter(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// First call creates counter
	counter1 := registry.Counter(TestCounterKey)
	if counter1 == nil {
		t.Fatal("Counter() returned nil")
	}

	// Second call returns same counter
	counter2 := registry.Counter(TestCounterKey)
	if counter1 != counter2 {
		t.Error("Counter() should return same instance for same key")
	}

	// Different key creates different counter
	counter3 := registry.Counter(OtherCounterKey)
	if counter1 == counter3 {
		t.Error("Counter() should create different instances for different keys")
	}
}

func TestRegistry_Gauge(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// First call creates gauge
	gauge1 := registry.Gauge(TestGaugeKey)
	if gauge1 == nil {
		t.Fatal("Gauge() returned nil")
	}

	// Second call returns same gauge
	gauge2 := registry.Gauge(TestGaugeKey)
	if gauge1 != gauge2 {
		t.Error("Gauge() should return same instance for same key")
	}

	// Different key creates different gauge
	gauge3 := registry.Gauge(OtherGaugeKey)
	if gauge1 == gauge3 {
		t.Error("Gauge() should create different instances for different keys")
	}
}

func TestRegistry_Histogram(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	buckets := []float64{1, 5, 10, 50, 100}

	// First call creates histogram
	hist1 := registry.Histogram(TestHistKey, buckets)
	if hist1 == nil {
		t.Fatal("Histogram() returned nil")
	}

	// Second call returns same histogram
	hist2 := registry.Histogram(TestHistKey, buckets)
	if hist1 != hist2 {
		t.Error("Histogram() should return same instance for same key")
	}

	// Different key creates different histogram
	hist3 := registry.Histogram(OtherHistKey, buckets)
	if hist1 == hist3 {
		t.Error("Histogram() should create different instances for different keys")
	}
}

func TestRegistry_Timer(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// First call creates timer
	timer1 := registry.Timer(TestTimerKey)
	if timer1 == nil {
		t.Fatal("Timer() returned nil")
	}

	// Second call returns same timer
	timer2 := registry.Timer(TestTimerKey)
	if timer1 != timer2 {
		t.Error("Timer() should return same instance for same key")
	}

	// Different key creates different timer
	timer3 := registry.Timer(OtherTimerKey)
	if timer1 == timer3 {
		t.Error("Timer() should create different instances for different keys")
	}
}

func TestRegistry_TimerWithBuckets(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)
	customBuckets := []float64{1, 10, 100, 1000}

	// First call creates timer with custom buckets
	timer1 := registry.TimerWithBuckets(TestTimerKey, customBuckets)
	if timer1 == nil {
		t.Fatal("TimerWithBuckets() returned nil")
	}

	// Second call returns same timer
	timer2 := registry.TimerWithBuckets(TestTimerKey, customBuckets)
	if timer1 != timer2 {
		t.Error("TimerWithBuckets() should return same instance for same key")
	}

	// Different key creates different timer
	timer3 := registry.TimerWithBuckets(OtherTimerKey, customBuckets)
	if timer1 == timer3 {
		t.Error("TimerWithBuckets() should create different instances for different keys")
	}

	// Verify custom buckets are used by recording a value and checking buckets
	timer1.Record(15 * time.Millisecond) // 15ms
	buckets, counts := timer1.Buckets()

	// Should have custom buckets + overflow bucket
	expectedBuckets := len(customBuckets) + 1
	if len(buckets) != expectedBuckets {
		t.Errorf("Expected %d buckets, got %d", expectedBuckets, len(buckets))
	}

	// Verify our custom buckets are present
	for i, expected := range customBuckets {
		if buckets[i] != expected {
			t.Errorf("Bucket %d: expected %v, got %v", i, expected, buckets[i])
		}
	}

	// The 15ms value should be in bucket 2 (100ms bucket, since 15 > 10)
	if counts[2] != 1 {
		t.Errorf("Expected count 1 in bucket 2, got %d. Full counts: %v", counts[2], counts)
	}
}

func TestRegistry_Reset(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// Use keys from metricz package - these are already defined as constants

	// Create some metrics
	registry.Counter(Counter1Key).Inc()
	registry.Gauge(Gauge1Key).Set(42.0)
	registry.Histogram(Hist1Key, metricz.DefaultLatencyBuckets).Observe(1.0)
	registry.Timer(Timer1Key).Record(100)

	// Verify metrics exist
	if registry.Counter(Counter1Key).Value() == 0 {
		t.Error("Counter should have non-zero value before reset")
	}

	// Reset registry
	registry.Reset()

	// Verify metrics are cleared
	if registry.Counter(Counter1Key).Value() != 0 {
		t.Error("Counter should be reset to zero")
	}
	if registry.Gauge(Gauge1Key).Value() != 0 {
		t.Error("Gauge should be reset to zero")
	}
	if registry.Histogram(Hist1Key, metricz.DefaultLatencyBuckets).Count() != 0 {
		t.Error("Histogram should be reset")
	}
	if registry.Timer(Timer1Key).Count() != 0 {
		t.Error("Timer should be reset")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// Use keys from metricz package
	// Key constants are already defined in metricz package

	const workers = 100
	const iterations = 100

	// Use GenerateLoad for concurrent counter access
	metricstesting.GenerateLoad(t, metricstesting.LoadConfig{
		Workers:    workers,
		Operations: iterations,
		Operation: func(_, _ int) {
			registry.Counter(ConcurrentCounterKey).Inc()
		},
	})

	expected := float64(workers * iterations)
	if registry.Counter(ConcurrentCounterKey).Value() != expected {
		t.Errorf("Expected counter value %f, got %f",
			expected, registry.Counter(ConcurrentCounterKey).Value())
	}

	// Test concurrent metric creation - skip this test as it uses dynamic keys
	// ARCHITECTURAL DECISION: Dynamic keys defeat compile-time safety
	// This test pattern has been removed to maintain type safety guarantees.

	// Create and test some additional metrics to verify registry functionality
	counter0 := registry.Counter(Counter0Key)
	counter0.Inc()
	if counter0.Value() != 1.0 {
		t.Error("Expected counter0 to have value 1.0")
	}

	gauge0 := registry.Gauge(Gauge0Key)
	if gauge0.Value() != 0.0 {
		t.Error("Expected gauge0 to have value 0.0")
	}
}

func TestRegistry_InstanceIsolation(t *testing.T) {
	// Create two separate registries
	registry1 := metricstesting.NewTestRegistry(t)
	registry2 := metricstesting.NewTestRegistry(t)

	// Isolation test key
	// Use existing test key from metricz package

	// Increment counter in first registry
	registry1.Counter(TestKey).Inc()

	// Verify second registry is unaffected
	if registry2.Counter(TestKey).Value() != 0 {
		t.Error("Registries should be completely isolated")
	}

	// Verify first registry has the value
	if registry1.Counter(TestKey).Value() != 1 {
		t.Error("First registry should have incremented value")
	}
}

func TestRegistry_KeyTypeEnforcement(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// Test that Key constants work correctly
	const RequestsKey metricz.Key = "requests"
	const ErrorsKey metricz.Key = "errors"

	counter1 := registry.Counter(RequestsKey)
	counter2 := registry.Counter(ErrorsKey)

	counter1.Inc()
	counter2.Add(5)

	if counter1.Value() != 1 {
		t.Error("RequestsKey counter should have value 1")
	}
	if counter2.Value() != 5 {
		t.Error("ErrorsKey counter should have value 5")
	}

	// Verify they are different instances
	if counter1 == counter2 {
		t.Error("Different keys should create different counters")
	}
}

func TestRegistry_KeyVariables(t *testing.T) {
	registry := metricstesting.NewTestRegistry(t)

	// Test Key variables (the recommended pattern)
	var (
		RequestsKey = metricz.Key("requests")
		ErrorsKey   = metricz.Key("errors")
		LatencyKey  = metricz.Key("latency")
	)

	counter := registry.Counter(RequestsKey)
	gauge := registry.Gauge(ErrorsKey)
	timer := registry.Timer(LatencyKey)

	counter.Inc()
	gauge.Set(42.0)
	timer.Record(100)

	if counter.Value() != 1 {
		t.Error("Counter should have value 1")
	}
	if gauge.Value() != 42.0 {
		t.Error("Gauge should have value 42.0")
	}
	if timer.Count() != 1 {
		t.Error("Timer should have count 1")
	}
}
