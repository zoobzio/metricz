package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

func TestWorkerPoolBasic(t *testing.T) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   5,
		QueueSize: 10,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Submit a job
	job := Job{
		ID:   "test-1",
		Type: "test",
		Data: map[string]interface{}{"value": 42},
	}

	err := pool.Submit(job)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check metrics
	counters := registry.GetCounters()
	if total, exists := counters["jobs_total"]; !exists {
		t.Error("jobs_total counter not found")
	} else if total.Value() != 1 {
		t.Errorf("Expected 1 total job, got %f", total.Value())
	}

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := pool.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestWorkerPoolQueueFull(t *testing.T) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Fill the queue
	for i := 0; i < 5; i++ {
		job := Job{
			ID:   fmt.Sprintf("job-%d", i),
			Type: "slow",
			Data: map[string]interface{}{},
		}
		if err := pool.Submit(job); err != nil {
			t.Fatalf("Failed to submit job %d: %v", i, err)
		}
	}

	// Queue should be full now
	job := Job{
		ID:   "overflow",
		Type: "test",
		Data: map[string]interface{}{},
	}

	err := pool.Submit(job)
	if err != ErrPoolFull {
		t.Errorf("Expected ErrPoolFull, got %v", err)
	}

	// Check dropped counter
	counters := registry.GetCounters()
	if dropped, exists := counters["jobs_dropped"]; !exists {
		t.Error("jobs_dropped counter not found")
	} else if dropped.Value() != 1 {
		t.Errorf("Expected 1 dropped job, got %f", dropped.Value())
	}

	cancel()
}

func TestWorkerPoolConcurrent(t *testing.T) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   10,
		QueueSize: 100,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Submit many jobs concurrently
	var wg sync.WaitGroup
	numJobs := 100

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job := Job{
				ID:   fmt.Sprintf("concurrent-%d", id),
				Type: "test",
				Data: map[string]interface{}{"id": id},
			}
			pool.Submit(job)
		}(i)
	}

	wg.Wait()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Check total jobs
	counters := registry.GetCounters()
	if total, exists := counters["jobs_total"]; !exists {
		t.Error("jobs_total counter not found")
	} else if total.Value() != float64(numJobs) {
		t.Errorf("Expected %d total jobs, got %f", numJobs, total.Value())
	}

	// Check active workers gauge
	gauges := registry.GetGauges()
	if activeWorkers, exists := gauges["active_workers"]; exists {
		// During processing, some workers should have been active
		// This is just informational - workers may still be waiting for jobs
		t.Logf("Active workers: %f", activeWorkers.Value())
	}

	cancel()
}

func TestWorkerPoolMetricsByType(t *testing.T) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   5,
		QueueSize: 50,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Submit different job types
	jobTypes := map[string]int{
		"process_order":   10,
		"send_email":      15,
		"generate_report": 5,
	}

	for jobType, count := range jobTypes {
		for i := 0; i < count; i++ {
			job := Job{
				ID:   fmt.Sprintf("%s-%d", jobType, i),
				Type: jobType,
				Data: generateJobData(jobType),
			}
			if err := pool.Submit(job); err != nil {
				t.Errorf("Failed to submit %s job: %v", jobType, err)
			}
		}
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Check per-type counters
	counters := registry.GetCounters()
	for jobType, expectedCount := range jobTypes {
		key := fmt.Sprintf("jobs_%s", jobType)
		if counter, exists := counters[key]; !exists {
			t.Errorf("Counter %s not found", key)
		} else if counter.Value() != float64(expectedCount) {
			t.Errorf("Expected %d %s jobs, got %f",
				expectedCount, jobType, counter.Value())
		}
	}

	// Check per-type timers exist
	timers := registry.GetTimers()
	for jobType := range jobTypes {
		key := fmt.Sprintf("job_duration_%s", jobType)
		if timer, exists := timers[key]; !exists {
			t.Errorf("Timer %s not found", key)
		} else if timer.Count() == 0 {
			t.Errorf("Timer %s has no recordings", key)
		}
	}

	cancel()
}

func TestWorkerPoolGracefulShutdown(t *testing.T) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   3,
		QueueSize: 20,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Submit jobs that take time
	for i := 0; i < 10; i++ {
		job := Job{
			ID:   fmt.Sprintf("shutdown-test-%d", i),
			Type: "generate_report",
			Data: map[string]interface{}{"size": 10}, // 100ms each
		}
		pool.Submit(job)
	}

	// Start shutdown while jobs are processing
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err := pool.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify all submitted jobs were processed
	counters := registry.GetCounters()
	total := counters["jobs_total"].Value()
	success := counters["jobs_success"].Value()
	errors := counters["jobs_error"].Value()

	if total != success+errors {
		t.Errorf("Job accounting mismatch: total=%f, success=%f, errors=%f",
			total, success, errors)
	}

	// No jobs should be dropped during graceful shutdown
	if dropped, exists := counters["jobs_dropped"]; exists && dropped.Value() > 0 {
		t.Errorf("Jobs dropped during graceful shutdown: %f", dropped.Value())
	}
}

func TestWorkerPoolQueueDepth(t *testing.T) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   1, // Single worker to control processing
		QueueSize: 10,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Submit several slow jobs - use generate_report which takes longer
	for i := 0; i < 5; i++ {
		job := Job{
			ID:   fmt.Sprintf("queue-test-%d", i),
			Type: "generate_report",
			Data: map[string]interface{}{"size": 50}, // 500ms each
		}
		pool.Submit(job)
	}

	// Give queue monitor time to update
	time.Sleep(150 * time.Millisecond)

	// Check queue depth
	gauges := registry.GetGauges()
	if queueDepth, exists := gauges["queue_depth"]; !exists {
		t.Error("queue_depth gauge not found")
	} else if queueDepth.Value() < 3 {
		// Should have several jobs queued (worker is processing one)
		t.Errorf("Expected queue depth >= 3, got %f", queueDepth.Value())
	}

	cancel()
}

// Benchmark pool throughput
func BenchmarkWorkerPool(b *testing.B) {
	registry := metricz.New()
	config := PoolConfig{
		Workers:   10,
		QueueSize: 1000,
		Registry:  registry,
	}

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	b.ResetTimer()

	// Submit b.N jobs
	for i := 0; i < b.N; i++ {
		job := Job{
			ID:   fmt.Sprintf("bench-%d", i),
			Type: "test",
			Data: map[string]interface{}{"id": i},
		}
		pool.Submit(job)
	}

	// Wait for all jobs to complete
	for {
		counters := registry.GetCounters()
		completed := counters["jobs_success"].Value() + counters["jobs_error"].Value()

		if completed >= float64(b.N) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
}
