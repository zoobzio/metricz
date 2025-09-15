package testing

import (
	"testing"

	"github.com/zoobzio/metricz"
)

// Example showing how to use the testing helpers.
// This demonstrates the cleanup and load generation utilities.
func TestExampleTestingHelpers(t *testing.T) {
	// Create registry with automatic cleanup
	registry := NewTestRegistry(t)

	// Use load generator for concurrent operations
	config := LoadConfig{
		Workers:    10,
		Operations: 100,
		Operation: func(workerID, _ int) {
			// Each worker increments its own counter
			key := metricz.Key("worker_" + string(rune('0'+workerID)))
			registry.Counter(key).Inc()
		},
	}

	GenerateLoad(t, config)

	// Verify each worker executed all operations
	for i := 0; i < 10; i++ {
		key := metricz.Key("worker_" + string(rune('0'+i)))
		counter := registry.Counter(key)
		expected := 100.0

		if counter.Value() != expected {
			t.Errorf("Worker %d counter expected %f, got %f", i, expected, counter.Value())
		}
	}

	// Registry will be automatically reset after test via t.Cleanup
}

// Example showing multiple isolated registries.
func TestExampleMultipleRegistries(t *testing.T) {
	registries := NewTestRegistries(t, 3)

	// Each registry tracks different metrics independently
	for i, reg := range registries {
		counter := reg.Counter(metricz.Key("instance_counter"))
		counter.Add(float64(i + 1))
	}

	// Verify isolation - each registry has different values
	for i, reg := range registries {
		counter := reg.Counter(metricz.Key("instance_counter"))
		expected := float64(i + 1)

		if counter.Value() != expected {
			t.Errorf("Registry %d counter expected %f, got %f", i, expected, counter.Value())
		}
	}

	// All registries will be automatically reset after test
}
