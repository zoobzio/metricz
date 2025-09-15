package reliability

import (
	"sync"
	"testing"

	"github.com/zoobzio/metricz"
)

// LoadConfig configures concurrent load generation for stress testing.
// Provides standardized patterns for concurrent metric operations.
type LoadConfig struct { //nolint:govet // fieldalignment - API design clarity over memory optimization
	Workers    int                      // Number of concurrent workers
	Operations int                      // Operations per worker
	Setup      func(workerID int)       // Optional per-worker setup
	Operation  func(workerID, opID int) // Operation to execute
}

// GenerateLoad runs concurrent operations using the provided configuration.
// Eliminates WaitGroup boilerplate and standardizes stress testing patterns.
// Captures worker ID to prevent closure issues in concurrent execution.
func GenerateLoad(_ *testing.T, config LoadConfig) {
	var wg sync.WaitGroup

	for w := 0; w < config.Workers; w++ {
		wg.Add(1)
		workerID := w // Capture for closure

		go func() {
			defer wg.Done()

			// Optional per-worker setup (e.g., local variables)
			if config.Setup != nil {
				config.Setup(workerID)
			}

			// Execute operations for this worker
			for op := 0; op < config.Operations; op++ {
				config.Operation(workerID, op)
			}
		}()
	}

	wg.Wait()
}

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
