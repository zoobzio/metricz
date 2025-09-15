package testing

import (
	"testing"

	"github.com/zoobzio/metricz"
)

func TestNewTestRegistry(t *testing.T) {
	// Create test registry
	r := NewTestRegistry(t)

	// Verify it's a valid registry
	if r == nil {
		t.Fatal("NewTestRegistry returned nil")
	}

	// Verify it behaves like normal registry
	counter := r.Counter(metricz.Key("test_counter"))
	counter.Inc()

	if counter.Value() != 1.0 {
		t.Errorf("Expected counter value 1.0, got %f", counter.Value())
	}

	// Note: Cleanup verification happens automatically via t.Cleanup
}

func TestNewTestRegistries(t *testing.T) {
	const count = 3
	registries := NewTestRegistries(t, count)

	// Verify correct count
	if len(registries) != count {
		t.Fatalf("Expected %d registries, got %d", count, len(registries))
	}

	// Verify all are valid and isolated
	for i, r := range registries {
		if r == nil {
			t.Fatalf("Registry %d is nil", i)
		}

		// Each registry should be independent
		counter := r.Counter(metricz.Key("test_counter"))
		counter.Add(float64(i + 1)) // Different value per registry

		expected := float64(i + 1)
		if counter.Value() != expected {
			t.Errorf("Registry %d counter expected %f, got %f", i, expected, counter.Value())
		}
	}

	// Verify registries are distinct objects
	if registries[0] == registries[1] {
		t.Error("Registries should be distinct objects")
	}
}

func TestNewTestRegistries_ZeroCount(t *testing.T) {
	registries := NewTestRegistries(t, 0)

	if len(registries) != 0 {
		t.Errorf("Expected empty slice, got %d registries", len(registries))
	}
}
