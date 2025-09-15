package integration

import (
	"fmt"
	"sync"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// TestTableDrivenIsolation demonstrates isolated metrics in table-driven tests.
func TestTableDrivenIsolation(t *gotesting.T) {
	tests := []struct { //nolint:govet // fieldalignment - test table readability over memory optimization
		name           string
		operations     int
		errorRate      float64
		expectedHealth string
	}{
		{
			name:           "healthy_service",
			operations:     100,
			errorRate:      0.01,
			expectedHealth: "healthy",
		},
		{
			name:           "degraded_service",
			operations:     100,
			errorRate:      0.10,
			expectedHealth: "degraded",
		},
		{
			name:           "critical_service",
			operations:     100,
			errorRate:      0.25,
			expectedHealth: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *gotesting.T) {
			// Each test gets its own registry - complete isolation
			registry := testing.NewTestRegistry(t)

			// Simulate service operations
			for i := 0; i < tt.operations; i++ {
				registry.Counter("ops.total").Inc()

				// Simulate errors based on rate
				if float64(i)/float64(tt.operations) < tt.errorRate {
					registry.Counter("ops.errors").Inc()
				}
			}

			// Calculate health status
			total := registry.Counter("ops.total").Value()
			errors := registry.Counter("ops.errors").Value()
			errorPercent := (errors / total) * 100

			var health string
			switch {
			case errorPercent < 5:
				health = "healthy"
			case errorPercent < 15:
				health = "degraded"
			default:
				health = "critical"
			}

			if health != tt.expectedHealth {
				t.Errorf("Expected health status %s, got %s (error rate: %.2f%%)",
					tt.expectedHealth, health, errorPercent)
			}

			// Verify metrics are isolated between test cases
			if total != float64(tt.operations) {
				t.Errorf("Registry contamination detected: expected %d ops, got %f",
					tt.operations, total)
			}
		})
	}
}

// TestParallelTestIsolation demonstrates safe parallel test execution.
func TestParallelTestIsolation(t *gotesting.T) {
	// Run multiple tests in parallel, each with its own metrics
	testCases := []string{"api", "auth", "database", "cache", "queue"}

	for _, name := range testCases {
		t.Run(name, func(t *gotesting.T) {
			t.Parallel() // Run in parallel

			// Each parallel test has its own registry
			registry := testing.NewTestRegistry(t)

			// Unique metric pattern per test
			metricName := name + ".operations"
			expectedValue := float64(len(name) * 10)

			// Perform operations
			for i := 0; i < len(name)*10; i++ {
				registry.Counter(metricz.Key(metricName)).Inc()
			}

			// Add some timing
			for i := 0; i < 5; i++ {
				registry.Timer(metricz.Key(name + ".latency")).Record(
					time.Duration(i) * time.Millisecond,
				)
			}

			// Verify isolation
			actual := registry.Counter(metricz.Key(metricName)).Value()
			if actual != expectedValue {
				t.Errorf("Test %s: expected %f operations, got %f",
					name, expectedValue, actual)
			}

			// Check no cross-contamination
			for _, other := range testCases {
				if other != name {
					otherMetric := other + ".operations"
					if registry.Counter(metricz.Key(otherMetric)).Value() != 0 {
						t.Errorf("Test %s contaminated by %s metrics", name, other)
					}
				}
			}
		})
	}
}

// TestBenchmarkIsolation demonstrates metric isolation in benchmarks.
func TestBenchmarkIsolation(t *gotesting.T) {
	// Simulate benchmark scenarios
	runBenchmark := func(b *gotesting.B, registry *metricz.Registry) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Benchmark operation
			registry.Counter("benchmark.ops").Inc()

			// Simulate some work
			registry.Timer("benchmark.duration").Record(100 * time.Nanosecond)
		}

		b.StopTimer()

		// Report metrics
		ops := registry.Counter("benchmark.ops").Value()
		b.ReportMetric(ops/float64(b.N), "ops/iteration")

		timings := registry.Timer("benchmark.duration").Count()
		b.ReportMetric(float64(timings), "timings")
	}

	// Run multiple benchmark scenarios
	b1 := &gotesting.B{N: 100}
	registry1 := testing.NewTestRegistry(t)
	runBenchmark(b1, registry1)

	b2 := &gotesting.B{N: 1000}
	registry2 := testing.NewTestRegistry(t)
	runBenchmark(b2, registry2)

	// Verify isolation
	if registry1.Counter("benchmark.ops").Value() != 100 {
		t.Error("Benchmark 1 metrics incorrect")
	}

	if registry2.Counter("benchmark.ops").Value() != 1000 {
		t.Error("Benchmark 2 metrics incorrect")
	}
}

// TestTestHelperPattern demonstrates using metrics in test helpers.
func TestTestHelperPattern(t *gotesting.T) {
	// Test helper that uses metrics
	type testHelper struct {
		t        *gotesting.T
		registry *metricz.Registry
	}

	newHelper := func(t *gotesting.T) *testHelper {
		return &testHelper{
			t:        t,
			registry: testing.NewTestRegistry(t), // Each helper gets its own registry
		}
	}

	// Helper methods that track their usage
	createUser := func(helper *testHelper, _ int) { // id unused in user creation
		helper.registry.Counter(HelperUsersCreatedKey).Inc()
		helper.registry.Timer(HelperUserCreationKey).Record(10 * time.Millisecond)
	}

	deleteUser := func(helper *testHelper, _ int) { // id unused in user deletion
		helper.registry.Counter(HelperUsersDeletedKey).Inc()
	}

	verifyMetrics := func(helper *testHelper) {
		created := helper.registry.Counter(HelperUsersCreatedKey).Value()
		deleted := helper.registry.Counter(HelperUsersDeletedKey).Value()

		if created != deleted {
			helper.t.Errorf("Resource leak: created %f users but deleted %f",
				created, deleted)
		}
	}

	// Test using the helper
	t.Run("test_with_helper", func(t *gotesting.T) {
		helper := newHelper(t)

		// Create and delete users
		for i := 0; i < 10; i++ {
			createUser(helper, i)
		}

		for i := 0; i < 10; i++ {
			deleteUser(helper, i)
		}

		verifyMetrics(helper) // Should pass
	})

	t.Run("test_with_leak", func(t *gotesting.T) {
		helper := newHelper(t)

		// Create users but forget to delete some
		for i := 0; i < 10; i++ {
			createUser(helper, i)
		}

		for i := 0; i < 7; i++ { // Only delete 7
			deleteUser(helper, i)
		}

		// This would fail if uncommented:
		// verifyMetrics(helper) // Would detect the leak

		// For test purposes, verify the leak is detected
		created := helper.registry.Counter(HelperUsersCreatedKey).Value()
		deleted := helper.registry.Counter(HelperUsersDeletedKey).Value()
		if created == deleted {
			t.Error("Should have detected resource leak")
		}
	})
}

// TestFixturePattern demonstrates metrics in test fixtures.
func TestFixturePattern(t *gotesting.T) {
	// Test fixture with built-in metrics
	type fixture struct { //nolint:govet // fieldalignment - logical field order more important in test fixture
		registry *metricz.Registry
		mu       sync.Mutex
		data     map[string]interface{}
	}

	newFixture := func() *fixture {
		return &fixture{
			registry: testing.NewTestRegistry(t),
			data:     make(map[string]interface{}),
		}
	}

	// Fixture operations with metrics
	setup := func(f *fixture) {
		f.mu.Lock()
		defer f.mu.Unlock()

		timer := f.registry.Timer("fixture.setup.duration").Start()
		defer timer.Stop()

		// Simulate setup work
		f.data["config"] = "test"
		f.data["connection"] = "established"

		f.registry.Counter("fixture.setups").Inc()
		f.registry.Gauge("fixture.resources").Set(float64(len(f.data)))
	}

	teardown := func(f *fixture) {
		f.mu.Lock()
		defer f.mu.Unlock()

		timer := f.registry.Timer("fixture.teardown.duration").Start()
		defer timer.Stop()

		// Cleanup
		for key := range f.data {
			delete(f.data, key)
		}

		f.registry.Counter("fixture.teardowns").Inc()
		f.registry.Gauge("fixture.resources").Set(0)
	}

	// Test using fixtures
	t.Run("single_test", func(t *gotesting.T) {
		fx := newFixture()
		setup(fx)
		defer teardown(fx)

		// Test operations
		fx.registry.Counter("test.operations").Add(10)

		// Verify fixture metrics
		if fx.registry.Counter("fixture.setups").Value() != 1 {
			t.Error("Setup not tracked")
		}

		if fx.registry.Gauge("fixture.resources").Value() != 2 {
			t.Error("Resources not tracked correctly")
		}
	})

	// Multiple tests with separate fixtures
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("repeated_test_%d", i), func(t *gotesting.T) {
			fx := newFixture()
			setup(fx)
			defer teardown(fx)

			// Each test has isolated metrics
			for j := 0; j < i+1; j++ {
				fx.registry.Counter("test.iterations").Inc()
			}

			expected := float64(i + 1)
			actual := fx.registry.Counter("test.iterations").Value()
			if actual != expected {
				t.Errorf("Test %d: expected %f iterations, got %f", i, expected, actual)
			}
		})
	}
}

// TestSubtestIsolation demonstrates metric isolation in subtests.
func TestSubtestIsolation(t *gotesting.T) {
	// Parent test with its own metrics
	parentRegistry := testing.NewTestRegistry(t)
	parentRegistry.Counter("parent.setup").Inc()

	// Run subtests
	subtests := []struct {
		name       string
		operations int
	}{
		{"small", 10},
		{"medium", 50},
		{"large", 100},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *gotesting.T) {
			// Each subtest gets its own registry
			registry := testing.NewTestRegistry(t)

			// Subtest operations
			for i := 0; i < st.operations; i++ {
				registry.Counter("subtest.ops").Inc()
			}

			// Verify isolation from parent
			if registry.Counter("parent.setup").Value() != 0 {
				t.Error("Subtest should not see parent metrics")
			}

			// Verify expected operations
			if registry.Counter("subtest.ops").Value() != float64(st.operations) {
				t.Errorf("Expected %d ops, got %f",
					st.operations, registry.Counter("subtest.ops").Value())
			}
		})
	}

	// Parent metrics should be unchanged by subtests
	if parentRegistry.Counter("parent.setup").Value() != 1 {
		t.Error("Parent metrics contaminated by subtests")
	}
}

// TestCleanupPattern demonstrates proper metric cleanup in tests.
func TestCleanupPattern(t *gotesting.T) {
	// Test that demonstrates cleanup patterns
	type testContext struct {
		registry *metricz.Registry
		cleanup  []func()
	}

	setupTest := func(t *gotesting.T) *testContext {
		ctx := &testContext{
			registry: testing.NewTestRegistry(t),
			cleanup:  make([]func(), 0),
		}

		// Register cleanup
		t.Cleanup(func() {
			// Run all cleanup functions in reverse order
			for i := len(ctx.cleanup) - 1; i >= 0; i-- {
				ctx.cleanup[i]()
			}

			// Verify cleanup metrics
			if ctx.registry.Gauge("resources.active").Value() != 0 {
				t.Error("Resources not properly cleaned up")
			}
		})

		return ctx
	}

	// Test with proper cleanup
	t.Run("with_cleanup", func(t *gotesting.T) {
		ctx := setupTest(t)

		// Allocate resources
		for i := 0; i < 5; i++ {
			ctx.registry.Gauge("resources.active").Inc()

			// Register cleanup for this resource
			resourceID := i
			ctx.cleanup = append(ctx.cleanup, func() {
				ctx.registry.Gauge("resources.active").Dec()
				ctx.registry.Counter("cleanup.executed").Inc()
				t.Logf("Cleaned up resource %d", resourceID)
			})
		}

		// Verify resources are allocated
		if ctx.registry.Gauge("resources.active").Value() != 5 {
			t.Error("Resources not properly allocated")
		}

		// Cleanup will run automatically via t.Cleanup
	})
}

// TestMockingPattern demonstrates using metrics to verify mock behavior.
func TestMockingPattern(t *gotesting.T) {
	// Mock service with metrics
	type mockService struct {
		registry *metricz.Registry
		behavior map[string]func() error
	}

	newMockService := func() *mockService {
		return &mockService{
			registry: testing.NewTestRegistry(t),
			behavior: make(map[string]func() error),
		}
	}

	// Mock method that tracks calls per method
	call := func(m *mockService, method string) error {
		// Track calls per method using a composite key
		callKey := metricz.Key(fmt.Sprintf("mock.calls.%s", method))
		m.registry.Counter(callKey).Inc()

		timer := m.registry.Timer(MockLatencyKey).Start()
		defer timer.Stop()

		if fn, exists := m.behavior[method]; exists {
			err := fn()
			if err != nil {
				// Track errors per method
				errorKey := metricz.Key(fmt.Sprintf("mock.errors.%s", method))
				m.registry.Counter(errorKey).Inc()
			}
			return err
		}

		return nil
	}

	// Verification helper
	verifyCalled := func(m *mockService, t *gotesting.T, method string, times float64) {
		callKey := metricz.Key(fmt.Sprintf("mock.calls.%s", method))
		actual := m.registry.Counter(callKey).Value()
		if actual != times {
			t.Errorf("Method %s: expected %f calls, got %f", method, times, actual)
		}
	}

	// Test using the mock
	t.Run("mock_verification", func(t *gotesting.T) {
		mock := newMockService()

		// Configure behavior
		mock.behavior["save"] = func() error { return nil }
		mock.behavior["delete"] = func() error { return fmt.Errorf("not allowed") }

		// Use the mock
		call(mock, "save")   //nolint:errcheck // Testing mock behavior, error intentionally ignored
		call(mock, "save")   //nolint:errcheck // Testing mock behavior, error intentionally ignored
		call(mock, "delete") //nolint:errcheck // Testing mock behavior, error intentionally ignored
		call(mock, "query")  //nolint:errcheck // Testing mock behavior, error intentionally ignored

		// Verify calls
		verifyCalled(mock, t, "save", 2)
		verifyCalled(mock, t, "delete", 1)
		verifyCalled(mock, t, "query", 1)

		// Verify errors
		if mock.registry.Counter("mock.errors.delete").Value() != 1 {
			t.Error("Delete error not tracked")
		}

		if mock.registry.Counter("mock.errors.save").Value() != 0 {
			t.Error("Save should not have errors")
		}
	})
}
