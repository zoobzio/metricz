package integration

import (
	gotesting "testing"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// TestCompleteInstanceIsolation verifies that registry instances are completely isolated.
func TestCompleteInstanceIsolation(t *gotesting.T) {
	registry1 := testing.NewTestRegistry(t)
	registry2 := testing.NewTestRegistry(t)

	// Modify registry1
	registry1.Counter(SharedNameKey).Add(10.0)
	registry1.Gauge(SharedGaugeKey).Set(20.0)
	registry1.Histogram(SharedHistKey, metricz.DefaultLatencyBuckets).Observe(30.0)
	registry1.Timer(SharedTimerKey).Record(100)

	// Registry2 should be completely unaffected
	if registry2.Counter(SharedNameKey).Value() != 0 {
		t.Error("Registry2 counter should be unaffected by registry1 operations")
	}

	if registry2.Gauge(SharedGaugeKey).Value() != 0 {
		t.Error("Registry2 gauge should be unaffected by registry1 operations")
	}

	if registry2.Histogram(SharedHistKey, metricz.DefaultLatencyBuckets).Count() != 0 {
		t.Error("Registry2 histogram should be unaffected by registry1 operations")
	}

	if registry2.Timer(SharedTimerKey).Count() != 0 {
		t.Error("Registry2 timer should be unaffected by registry1 operations")
	}

	// Modify registry2 - should not affect registry1
	registry2.Counter(SharedNameKey).Add(5.0)

	if registry1.Counter(SharedNameKey).Value() != 10.0 {
		t.Error("Registry1 should maintain its values when registry2 is modified")
	}

	if registry2.Counter(SharedNameKey).Value() != 5.0 {
		t.Error("Registry2 should have its own independent values")
	}
}

// TestPackageRegistrationIsolation was removed - RegisterPackage functionality eliminated.
// This test is no longer relevant as Key type provides compile-time safety.

// TestResetIsolation tests that Reset() only affects the specific registry.
func TestResetIsolation(t *gotesting.T) {
	registry1 := testing.NewTestRegistry(t)
	registry2 := testing.NewTestRegistry(t)

	// Populate both registries
	registry1.Counter(Counter1Key).Add(10.0)
	registry1.Gauge(Gauge1Key).Set(20.0)

	registry2.Counter(Counter1Key).Add(30.0)
	registry2.Gauge(Gauge1Key).Set(40.0)

	// Reset only registry1
	registry1.Reset()

	// Registry1 should be cleared
	if registry1.Counter(Counter1Key).Value() != 0 {
		t.Error("Registry1 counter should be reset to zero")
	}

	if registry1.Gauge(Gauge1Key).Value() != 0 {
		t.Error("Registry1 gauge should be reset to zero")
	}

	// Registry2 should be unchanged
	if registry2.Counter(Counter1Key).Value() != 30.0 {
		t.Error("Registry2 counter should be unaffected by registry1 reset")
	}

	if registry2.Gauge(Gauge1Key).Value() != 40.0 {
		t.Error("Registry2 gauge should be unaffected by registry1 reset")
	}
}

// TestConcurrentMultipleInstances tests many instances working concurrently.
func TestConcurrentMultipleInstances(t *gotesting.T) {
	const instances = 50
	const operations = 100

	registries := testing.NewTestRegistries(t, instances)

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    instances,
		Operations: operations,
		Operation: func(workerID, opID int) {
			registry := registries[workerID]

			// Each instance does its own operations
			registry.Counter(OpsKey).Inc()
			registry.Gauge(StatusKey).Set(float64(workerID))
			registry.Histogram(LatencyKey, metricz.DefaultLatencyBuckets).Observe(float64(opID))
		},
	})

	// Verify each instance has independent values
	for i, registry := range registries {
		// Each should have exactly 'operations' counter increments
		if registry.Counter(OpsKey).Value() != float64(operations) {
			t.Errorf("Instance %d: expected %d operations, got %f",
				i, operations, registry.Counter(OpsKey).Value())
		}

		// Each should have gauge set to its instance ID
		if registry.Gauge(StatusKey).Value() != float64(i) {
			t.Errorf("Instance %d: expected gauge value %d, got %f",
				i, i, registry.Gauge(StatusKey).Value())
		}

		// Each should have exactly 'operations' histogram observations
		if registry.Histogram(LatencyKey, metricz.DefaultLatencyBuckets).Count() != uint64(operations) {
			t.Errorf("Instance %d: expected %d histogram observations, got %d",
				i, operations, registry.Histogram(LatencyKey, metricz.DefaultLatencyBuckets).Count())
		}
	}
}

// TestInstanceLifecycle tests that instances can be created and destroyed independently.
func TestInstanceLifecycle(t *gotesting.T) {
	// Create initial instance
	registry1 := testing.NewTestRegistry(t)
	registry1.Counter(TestKey).Inc()

	// Store reference to counter
	counter1 := registry1.Counter(TestKey)
	if counter1.Value() != 1.0 {
		t.Fatal("Initial counter setup failed")
	}

	// Create second instance with same metric name
	registry2 := testing.NewTestRegistry(t)
	counter2 := registry2.Counter(TestKey)

	// Should be completely separate
	if counter2.Value() != 0.0 {
		t.Error("Second instance should have independent counter")
	}

	// Modify second instance
	counter2.Add(5.0)

	// First instance should be unaffected
	if counter1.Value() != 1.0 {
		t.Error("First instance should be unaffected by second instance")
	}

	// Let registry1 go out of scope (simulated by setting to nil)
	registry1 = nil //nolint:wastedassign // Intentional nil assignment to test metric lifecycle

	// counter1 should still work (it has its own lifecycle)
	if counter1.Value() != 1.0 {
		t.Error("Counter should survive registry going out of scope")
	}

	// registry2 should still work normally
	if counter2.Value() != 5.0 {
		t.Error("Second registry should be unaffected by first registry cleanup")
	}
}

// TestNoSharedState verifies there's no hidden global state.
func TestNoSharedState(t *gotesting.T) {
	// Create many instances quickly
	const count = 100
	instances := testing.NewTestRegistries(t, count)

	for i := 0; i < count; i++ {

		// Each gets a unique "signature"
		instances[i].Counter(InstanceIDKey).Add(float64(i))
	}

	// Verify each instance maintained its signature
	for i, instance := range instances {
		expectedValue := float64(i)
		actualValue := instance.Counter(InstanceIDKey).Value()

		if actualValue != expectedValue {
			t.Errorf("Instance %d: expected signature %f, got %f",
				i, expectedValue, actualValue)
		}
	}

	// Cross-contamination test: modify one instance
	instances[0].Counter(ContaminationKey).Add(999.0)

	// Verify no other instance is affected
	for i := 1; i < count; i++ {
		if instances[i].Counter(ContaminationKey).Value() != 0.0 {
			t.Errorf("Instance %d was contaminated by instance 0", i)
		}
	}
}

// TestRegistryReaderIsolation tests that RegistryReader views are isolated.
func TestRegistryReaderIsolation(t *gotesting.T) {
	registry1 := testing.NewTestRegistry(t)
	registry2 := testing.NewTestRegistry(t)

	// Populate with different metrics
	registry1.Counter(Counter1Key).Add(10.0)
	registry1.Gauge(Gauge1Key).Set(20.0)

	registry2.Counter(Counter2Key).Add(30.0)
	registry2.Gauge(Gauge2Key).Set(40.0)

	// Get reader views
	counters1 := registry1.GetCounters()
	gauges1 := registry1.GetGauges()

	counters2 := registry2.GetCounters()
	gauges2 := registry2.GetGauges()

	// Registry1 readers should only see registry1 metrics
	if len(counters1) != 1 || len(gauges1) != 1 {
		t.Error("Registry1 should see exactly 1 counter and 1 gauge")
	}

	if _, exists := counters1["counter1"]; !exists {
		t.Error("Registry1 should see counter1")
	}

	if _, exists := counters1["counter2"]; exists {
		t.Error("Registry1 should not see counter2")
	}

	// Registry2 readers should only see registry2 metrics
	if len(counters2) != 1 || len(gauges2) != 1 {
		t.Error("Registry2 should see exactly 1 counter and 1 gauge")
	}

	if _, exists := counters2["counter2"]; !exists {
		t.Error("Registry2 should see counter2")
	}

	if _, exists := counters2["counter1"]; exists {
		t.Error("Registry2 should not see counter1")
	}
}

// TestParallelTestSafety ensures multiple test functions can run in parallel.
func TestParallelTestSafety(t *gotesting.T) {
	// This test should be safe to run with t.Parallel()

	subtests := []struct { //nolint:govet // fieldalignment - test struct readability over memory optimization
		name string
		fn   func(*gotesting.T)
	}{
		{"subtest1", func(t *gotesting.T) {
			registry := testing.NewTestRegistry(t)
			registry.Counter(Test1Key).Inc()
			if registry.Counter(Test1Key).Value() != 1.0 {
				t.Error("Subtest1 failed")
			}
		}},
		{"subtest2", func(t *gotesting.T) {
			registry := testing.NewTestRegistry(t)
			registry.Gauge(Test2Key).Set(42.0)
			if registry.Gauge(Test2Key).Value() != 42.0 {
				t.Error("Subtest2 failed")
			}
		}},
		{"subtest3", func(t *gotesting.T) {
			registry := testing.NewTestRegistry(t)
			registry.Histogram(Test3Key, metricz.DefaultLatencyBuckets).Observe(5.0)
			if registry.Histogram(Test3Key, metricz.DefaultLatencyBuckets).Count() != 1 {
				t.Error("Subtest3 failed")
			}
		}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *gotesting.T) {
			t.Parallel() // This should be safe with instance isolation
			st.fn(t)
		})
	}
}

// TestNoGlobalRegistryAccess verifies there's no way to access a global registry.
func TestNoGlobalRegistryAccess(t *gotesting.T) {
	// This is a compilation test - if there was a global Default variable,
	// the following code would compile. Since it doesn't exist, this test
	// serves as documentation that global access is impossible.

	registry := testing.NewTestRegistry(t)
	_ = registry // Use the instance

	// The following would not compile if uncommented:
	// metricz.Default.Counter("test") // No Default variable exists
	// metricz.GlobalCounter("test")   // No global functions exist

	// Only instance-based access is possible:
	counter := registry.Counter(TestKey)
	counter.Inc()

	if counter.Value() != 1.0 {
		t.Error("Instance-based access should work")
	}
}
