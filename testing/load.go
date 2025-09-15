package testing

import (
	"sync"
	"testing"
)

// LoadConfig configures concurrent load generation for stress testing.
// Provides standardized patterns for concurrent metric operations.
type LoadConfig struct {
	Setup      func(workerID int)       // Optional per-worker setup
	Operation  func(workerID, opID int) // Operation to execute
	Workers    int                      // Number of concurrent workers
	Operations int                      // Operations per worker
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
