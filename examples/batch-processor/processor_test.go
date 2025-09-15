package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

func TestBatchProcessorBasic(t *testing.T) {
	// Create test data file
	testFile := "test_data.json"
	defer os.Remove(testFile)

	generator := NewDataGenerator(testFile)
	if err := generator.GenerateRecords(50); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	// Create processor
	registry := metricz.New()
	config := ProcessorConfig{
		BatchSize:      10,
		MaxRetries:     2,
		RetryDelay:     100 * time.Millisecond,
		Registry:       registry,
		CheckpointFile: "test_checkpoint.json",
	}
	defer os.Remove(config.CheckpointFile)

	processor := NewBatchProcessor(config)

	// Process file
	ctx := context.Background()
	if err := processor.ProcessFile(ctx, testFile); err != nil {
		t.Fatalf("Failed to process file: %v", err)
	}

	// Verify metrics
	counters := registry.GetCounters()

	if processed, exists := counters["records_processed"]; !exists {
		t.Error("records_processed counter not found")
	} else if processed.Value() != 50 {
		t.Errorf("Expected 50 records processed, got %f", processed.Value())
	}

	if batches, exists := counters["batches_total"]; !exists {
		t.Error("batches_total counter not found")
	} else if batches.Value() != 5 {
		t.Errorf("Expected 5 batches, got %f", batches.Value())
	}

	// Check success rate (should be high given low error rates)
	success := counters["records_success"].Value()
	errors := counters["records_error"].Value()

	if success+errors != 50 {
		t.Errorf("Success + errors should equal total: %f + %f != 50",
			success, errors)
	}

	// Success rate should be > 90%
	successRate := (success / 50) * 100
	if successRate < 90 {
		t.Errorf("Success rate too low: %.2f%%", successRate)
	}
}

func TestBatchProcessorCheckpoint(t *testing.T) {
	// Create test data file
	testFile := "test_checkpoint_data.json"
	defer os.Remove(testFile)

	generator := NewDataGenerator(testFile)
	if err := generator.GenerateRecords(100); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	// Create processor
	registry := metricz.New()
	config := ProcessorConfig{
		BatchSize:      20,
		MaxRetries:     1,
		RetryDelay:     50 * time.Millisecond,
		Registry:       registry,
		CheckpointFile: "test_checkpoint_resume.json",
	}
	defer os.Remove(config.CheckpointFile)

	processor := NewBatchProcessor(config)

	// Process partially with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	processor.ProcessFile(ctx, testFile) // Will be interrupted
	cancel()

	// Check that some records were processed
	counters := registry.GetCounters()
	firstRun := counters["records_processed"].Value()

	if firstRun == 0 {
		t.Error("No records processed in first run")
	}
	if firstRun >= 100 {
		t.Error("All records processed despite cancellation")
	}

	// Check checkpoint exists
	if _, err := os.Stat(config.CheckpointFile); os.IsNotExist(err) {
		t.Error("Checkpoint file not created")
	}

	// Resume processing
	ctx = context.Background()
	if err := processor.ProcessFile(ctx, testFile); err != nil {
		t.Fatalf("Failed to resume processing: %v", err)
	}

	// Verify all records processed
	finalCount := counters["records_processed"].Value()
	if finalCount != 100 {
		t.Errorf("Expected 100 total records, got %f", finalCount)
	}

	// Checkpoint should be removed after completion
	if _, err := os.Stat(config.CheckpointFile); !os.IsNotExist(err) {
		t.Error("Checkpoint file not removed after completion")
	}
}

func TestBatchProcessorMetricsByType(t *testing.T) {
	// Create test data with known types
	testFile := "test_types_data.json"
	defer os.Remove(testFile)

	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write specific record types
	types := map[string]int{
		"user":        20,
		"transaction": 15,
		"analytics":   25,
	}

	id := 0
	for recordType, count := range types {
		for i := 0; i < count; i++ {
			record := Record{
				ID:        fmt.Sprintf("%s-%d", recordType, id),
				Type:      recordType,
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"test": true},
			}

			data, _ := json.Marshal(record)
			file.Write(append(data, '\n'))
			id++
		}
	}
	file.Close()

	// Process file
	registry := metricz.New()
	config := ProcessorConfig{
		BatchSize:      10,
		MaxRetries:     1,
		RetryDelay:     50 * time.Millisecond,
		Registry:       registry,
		CheckpointFile: "test_types_checkpoint.json",
	}
	defer os.Remove(config.CheckpointFile)

	processor := NewBatchProcessor(config)

	ctx := context.Background()
	if err := processor.ProcessFile(ctx, testFile); err != nil {
		t.Fatalf("Failed to process file: %v", err)
	}

	// Verify type-specific counters
	counters := registry.GetCounters()

	for recordType, expectedCount := range types {
		key := fmt.Sprintf("records_%s", recordType)
		if counter, exists := counters[key]; !exists {
			t.Errorf("Counter %s not found", key)
		} else if counter.Value() != float64(expectedCount) {
			t.Errorf("Expected %d %s records, got %f",
				expectedCount, recordType, counter.Value())
		}
	}
}

func TestBatchProcessorRetry(t *testing.T) {
	// Create test data that will trigger retries
	testFile := "test_retry_data.json"
	defer os.Remove(testFile)

	// Generate records that will have higher error rate
	generator := NewDataGenerator(testFile)
	if err := generator.GenerateRecords(20); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	// Create processor with small batch size for more retries
	registry := metricz.New()
	config := ProcessorConfig{
		BatchSize:      5,
		MaxRetries:     3,
		RetryDelay:     10 * time.Millisecond,
		Registry:       registry,
		CheckpointFile: "test_retry_checkpoint.json",
	}
	defer os.Remove(config.CheckpointFile)

	processor := NewBatchProcessor(config)

	ctx := context.Background()
	if err := processor.ProcessFile(ctx, testFile); err != nil {
		t.Fatalf("Failed to process file: %v", err)
	}

	// Check retry counter
	counters := registry.GetCounters()

	// With random failures, we might have some retries
	if retries, exists := counters["batch_retries"]; exists && retries.Value() > 0 {
		t.Logf("Batch retries occurred: %f", retries.Value())

		// Retries should be reasonable (not every batch)
		batches := counters["batches_total"].Value()
		if retries.Value() > batches {
			t.Errorf("Too many retries: %f retries for %f batches",
				retries.Value(), batches)
		}
	}
}

func TestBatchProcessorProgress(t *testing.T) {
	// Create test data
	testFile := "test_progress_data.json"
	defer os.Remove(testFile)

	generator := NewDataGenerator(testFile)
	if err := generator.GenerateRecords(100); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	// Create processor
	registry := metricz.New()
	config := ProcessorConfig{
		BatchSize:      25,
		MaxRetries:     1,
		RetryDelay:     50 * time.Millisecond,
		Registry:       registry,
		CheckpointFile: "test_progress_checkpoint.json",
	}
	defer os.Remove(config.CheckpointFile)

	processor := NewBatchProcessor(config)

	// Track progress updates
	progressChecks := make([]float64, 0)

	// Process file in background
	ctx := context.Background()
	done := make(chan error)
	go func() {
		done <- processor.ProcessFile(ctx, testFile)
	}()

	// Monitor progress
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gauges := registry.GetGauges()
			if progress, exists := gauges["batch_progress"]; exists {
				value := progress.Value()
				if value > 0 {
					progressChecks = append(progressChecks, value)
				}
			}

		case err := <-done:
			if err != nil {
				t.Fatalf("Processing failed: %v", err)
			}

			// Verify progress was tracked
			if len(progressChecks) == 0 {
				t.Error("No progress updates captured")
			}

			// Progress should increase
			for i := 1; i < len(progressChecks); i++ {
				if progressChecks[i] < progressChecks[i-1] {
					t.Errorf("Progress decreased: %.2f -> %.2f",
						progressChecks[i-1], progressChecks[i])
				}
			}

			// Final progress should be 100
			gauges := registry.GetGauges()
			if progress, exists := gauges["batch_progress"]; exists {
				if progress.Value() != 100.0 {
					t.Errorf("Final progress not 100%%: %.2f", progress.Value())
				}
			}

			return
		}
	}
}

// Benchmark batch processing
func BenchmarkBatchProcessor(b *testing.B) {
	// Create test data
	testFile := "bench_data.json"
	defer os.Remove(testFile)

	generator := NewDataGenerator(testFile)
	generator.GenerateRecords(1000)

	// Create processor
	registry := metricz.New()
	config := ProcessorConfig{
		BatchSize:      100,
		MaxRetries:     0,
		RetryDelay:     0,
		Registry:       registry,
		CheckpointFile: "bench_checkpoint.json",
	}
	defer os.Remove(config.CheckpointFile)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		processor := NewBatchProcessor(config)
		ctx := context.Background()
		processor.ProcessFile(ctx, testFile)

		// Reset registry for next iteration
		registry.Reset()
	}
}
