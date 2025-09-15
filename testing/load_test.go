package testing

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGenerateLoad_BasicOperation(t *testing.T) {
	var counter int64

	config := LoadConfig{
		Workers:    5,
		Operations: 10,
		Operation: func(_, _ int) {
			atomic.AddInt64(&counter, 1)
		},
	}

	GenerateLoad(t, config)

	expected := int64(5 * 10) // workers * operations
	if counter != expected {
		t.Errorf("Expected %d operations, got %d", expected, counter)
	}
}

func TestGenerateLoad_WithSetup(t *testing.T) {
	var setupCalls int64
	var operationCalls int64

	config := LoadConfig{
		Workers:    3,
		Operations: 5,
		Setup: func(_ int) {
			atomic.AddInt64(&setupCalls, 1)
		},
		Operation: func(_, _ int) {
			atomic.AddInt64(&operationCalls, 1)
		},
	}

	GenerateLoad(t, config)

	if setupCalls != 3 {
		t.Errorf("Expected 3 setup calls, got %d", setupCalls)
	}

	if operationCalls != 15 { // 3 workers * 5 operations
		t.Errorf("Expected 15 operation calls, got %d", operationCalls)
	}
}

func TestGenerateLoad_WorkerIsolation(t *testing.T) {
	workerCounts := make(map[int]int64)
	var mu sync.Mutex

	config := LoadConfig{
		Workers:    4,
		Operations: 10,
		Operation: func(workerID, _ int) {
			mu.Lock()
			workerCounts[workerID]++
			mu.Unlock()
		},
	}

	GenerateLoad(t, config)

	// Each worker should have executed exactly 10 operations
	for workerID := 0; workerID < 4; workerID++ {
		count := workerCounts[workerID]
		if count != 10 {
			t.Errorf("Worker %d executed %d operations, expected 10", workerID, count)
		}
	}
}

func TestGenerateLoad_NoSetup(t *testing.T) {
	var operationCalls int64

	config := LoadConfig{
		Workers:    2,
		Operations: 3,
		// Setup is nil
		Operation: func(_, _ int) {
			atomic.AddInt64(&operationCalls, 1)
		},
	}

	// Should not panic with nil Setup
	GenerateLoad(t, config)

	if operationCalls != 6 { // 2 workers * 3 operations
		t.Errorf("Expected 6 operation calls, got %d", operationCalls)
	}
}

func TestGenerateLoad_ZeroWorkers(t *testing.T) {
	var operationCalls int64

	config := LoadConfig{
		Workers:    0,
		Operations: 5,
		Operation: func(_, _ int) {
			atomic.AddInt64(&operationCalls, 1)
		},
	}

	GenerateLoad(t, config)

	if operationCalls != 0 {
		t.Errorf("Expected 0 operation calls with 0 workers, got %d", operationCalls)
	}
}

func TestGenerateLoad_ZeroOperations(t *testing.T) {
	var operationCalls int64

	config := LoadConfig{
		Workers:    3,
		Operations: 0,
		Operation: func(_, _ int) {
			atomic.AddInt64(&operationCalls, 1)
		},
	}

	GenerateLoad(t, config)

	if operationCalls != 0 {
		t.Errorf("Expected 0 operation calls with 0 operations, got %d", operationCalls)
	}
}
