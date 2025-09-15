package testing

import (
	"testing"

	"github.com/zoobzio/metricz"
)

// NewTestRegistry creates a registry with automatic cleanup.
// Uses t.Cleanup to ensure Reset() is called after test completion,
// preventing test contamination from lingering metric state.
func NewTestRegistry(t *testing.T) *metricz.Registry {
	r := metricz.New()
	t.Cleanup(func() {
		r.Reset()
	})
	return r
}

// NewTestRegistries creates multiple isolated registries with automatic cleanup.
// Each registry gets its own cleanup handler to ensure proper isolation.
func NewTestRegistries(t *testing.T, count int) []*metricz.Registry {
	registries := make([]*metricz.Registry, count)
	for i := range registries {
		registries[i] = NewTestRegistry(t) // Each gets individual cleanup
	}
	return registries
}
