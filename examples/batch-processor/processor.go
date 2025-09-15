package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/zoobzio/metricz"
)

// Key constants for processor metrics.
// Define all metric keys as compile-time constants.
const (
	// Progress and batch tracking
	BatchProgress    metricz.Key = "batch_progress"
	CurrentBatchSize metricz.Key = "current_batch_size"

	// Record processing metrics
	RecordsProcessed metricz.Key = "records_processed"
	RecordsSuccess   metricz.Key = "records_success"
	RecordsError     metricz.Key = "records_error"

	// Batch metrics
	BatchesTotal   metricz.Key = "batches_total"
	BatchRetries   metricz.Key = "batch_retries"
	BatchDuration  metricz.Key = "batch_duration"
	RecordDuration metricz.Key = "record_duration"

	// Record type metrics - predefined for known types
	RecordsUser        metricz.Key = "records_user"
	RecordsTransaction metricz.Key = "records_transaction"
	RecordsAnalytics   metricz.Key = "records_analytics"
	RecordsInventory   metricz.Key = "records_inventory"
	RecordsAudit       metricz.Key = "records_audit"
)

// ProcessorConfig configures the batch processor
type ProcessorConfig struct {
	BatchSize      int
	MaxRetries     int
	RetryDelay     time.Duration
	Registry       *metricz.Registry
	CheckpointFile string
}

// BatchProcessor processes data files in batches with metrics
type BatchProcessor struct {
	config ProcessorConfig

	// Metrics
	registry           *metricz.Registry
	batchProgress      metricz.Gauge
	currentBatchSize   metricz.Gauge
	recordsProcessed   metricz.Counter
	recordsSuccess     metricz.Counter
	recordsError       metricz.Counter
	batchesTotal       metricz.Counter
	batchRetries       metricz.Counter
	batchDuration      metricz.Timer
	recordDuration     metricz.Timer
	recordTypeCounters map[string]metricz.Counter

	// Checkpoint
	checkpoint *Checkpoint
}

// Checkpoint tracks processing progress
type Checkpoint struct {
	File        string    `json:"file"`
	Offset      int64     `json:"offset"`
	RecordsRead int       `json:"records_read"`
	LastUpdate  time.Time `json:"last_update"`
}

// Record represents a data record
type Record struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(config ProcessorConfig) *BatchProcessor {
	return &BatchProcessor{
		config:             config,
		registry:           config.Registry,
		batchProgress:      config.Registry.Gauge(BatchProgress),
		currentBatchSize:   config.Registry.Gauge(CurrentBatchSize),
		recordsProcessed:   config.Registry.Counter(RecordsProcessed),
		recordsSuccess:     config.Registry.Counter(RecordsSuccess),
		recordsError:       config.Registry.Counter(RecordsError),
		batchesTotal:       config.Registry.Counter(BatchesTotal),
		batchRetries:       config.Registry.Counter(BatchRetries),
		batchDuration:      config.Registry.Timer(BatchDuration),
		recordDuration:     config.Registry.Timer(RecordDuration),
		recordTypeCounters: make(map[string]metricz.Counter),
	}
}

// ProcessFile processes a data file in batches
func (p *BatchProcessor) ProcessFile(ctx context.Context, filename string) error {
	// Load or create checkpoint
	checkpoint, err := p.loadCheckpoint(filename)
	if err != nil {
		return fmt.Errorf("checkpoint error: %w", err)
	}
	p.checkpoint = checkpoint

	// Open file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Get file size for progress tracking
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	totalSize := stat.Size()

	// Seek to checkpoint offset
	if checkpoint.Offset > 0 {
		if _, err := file.Seek(checkpoint.Offset, 0); err != nil {
			return fmt.Errorf("seek to checkpoint: %w", err)
		}
	}

	reader := bufio.NewReader(file)
	batch := make([]Record, 0, p.config.BatchSize)

	for {
		select {
		case <-ctx.Done():
			// Save checkpoint before exit
			p.saveCheckpoint()
			return ctx.Err()
		default:
		}

		// Read line
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			// Process final batch
			if len(batch) > 0 {
				p.processBatchWithRetry(ctx, batch)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		// Parse record
		var record Record
		if err := json.Unmarshal(line, &record); err != nil {
			p.recordsError.Inc()
			continue
		}

		batch = append(batch, record)
		p.checkpoint.RecordsRead++

		// Update progress
		currentPos, _ := file.Seek(0, 1) // Get current position
		progress := float64(currentPos) / float64(totalSize) * 100
		p.batchProgress.Set(progress)

		// Process batch when full
		if len(batch) >= p.config.BatchSize {
			p.processBatchWithRetry(ctx, batch)
			batch = make([]Record, 0, p.config.BatchSize)

			// Update checkpoint
			p.checkpoint.Offset = currentPos
			p.saveCheckpoint()
		}
	}

	// Clear progress on completion
	p.batchProgress.Set(100.0)
	p.removeCheckpoint()

	return nil
}

// processBatchWithRetry processes a batch with retry logic
func (p *BatchProcessor) processBatchWithRetry(ctx context.Context, batch []Record) {
	p.currentBatchSize.Set(float64(len(batch)))
	defer p.currentBatchSize.Set(0)

	retries := 0
	backoff := p.config.RetryDelay

	for retries <= p.config.MaxRetries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if retries > 0 {
			p.batchRetries.Inc()
			// Exponential backoff with jitter
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			time.Sleep(backoff + jitter)
			backoff *= 2
		}

		err := p.processBatch(ctx, batch)
		if err == nil {
			return
		}

		retries++
	}
}

// processBatch processes a single batch
func (p *BatchProcessor) processBatch(ctx context.Context, batch []Record) error {
	stopwatch := p.batchDuration.Start()
	defer stopwatch.Stop()

	p.batchesTotal.Inc()

	var batchError error
	successCount := 0

	for _, record := range batch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		recordStopwatch := p.recordDuration.Start()
		err := p.processRecord(record)
		recordStopwatch.Stop()

		p.recordsProcessed.Inc()
		p.getRecordTypeCounter(record.Type).Inc()

		if err != nil {
			p.recordsError.Inc()
			batchError = err
		} else {
			p.recordsSuccess.Inc()
			successCount++
		}
	}

	// Consider batch successful if >50% records succeed
	if successCount < len(batch)/2 && batchError != nil {
		return fmt.Errorf("batch failed: %w", batchError)
	}

	return nil
}

// processRecord simulates processing a single record
func (p *BatchProcessor) processRecord(record Record) error {
	// Simulate processing based on record type
	switch record.Type {
	case "user":
		time.Sleep(5 * time.Millisecond)
		// 2% error rate for user records
		if rand.Float32() < 0.02 {
			return errors.New("user validation failed")
		}

	case "transaction":
		time.Sleep(10 * time.Millisecond)
		// 5% error rate for transactions
		if rand.Float32() < 0.05 {
			return errors.New("transaction processing failed")
		}

	case "analytics":
		time.Sleep(2 * time.Millisecond)
		// 1% error rate for analytics
		if rand.Float32() < 0.01 {
			return errors.New("analytics aggregation failed")
		}

	default:
		time.Sleep(3 * time.Millisecond)
	}

	// Simulate occasional slow records
	if rand.Float32() < 0.05 {
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

// getRecordTypeCounter returns a counter for a known record type
func (p *BatchProcessor) getRecordTypeCounter(recordType string) metricz.Counter {
	if counter, exists := p.recordTypeCounters[recordType]; exists {
		return counter
	}

	// Map record types to predefined keys
	var key metricz.Key
	switch recordType {
	case "user":
		key = RecordsUser
	case "transaction":
		key = RecordsTransaction
	case "analytics":
		key = RecordsAnalytics
	case "inventory":
		key = RecordsInventory
	case "audit":
		key = RecordsAudit
	default:
		// For unknown types, return a no-op counter
		// In production, you'd define all expected types as constants
		return &noopCounter{}
	}

	counter := p.registry.Counter(key)
	p.recordTypeCounters[recordType] = counter
	return counter
}

// noopCounter is a counter that does nothing for unknown types
type noopCounter struct{}

func (n *noopCounter) Inc()           {}
func (n *noopCounter) Add(v float64)  {}
func (n *noopCounter) Value() float64 { return 0 }

// loadCheckpoint loads or creates a checkpoint
func (p *BatchProcessor) loadCheckpoint(filename string) (*Checkpoint, error) {
	data, err := os.ReadFile(p.config.CheckpointFile)
	if os.IsNotExist(err) {
		// New checkpoint
		return &Checkpoint{
			File:       filename,
			Offset:     0,
			LastUpdate: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, err
	}

	// Check if it's for the same file
	if checkpoint.File != filename {
		// Different file, start fresh
		return &Checkpoint{
			File:       filename,
			Offset:     0,
			LastUpdate: time.Now(),
		}, nil
	}

	return &checkpoint, nil
}

// saveCheckpoint saves the current checkpoint
func (p *BatchProcessor) saveCheckpoint() error {
	if p.checkpoint == nil {
		return nil
	}

	p.checkpoint.LastUpdate = time.Now()
	data, err := json.Marshal(p.checkpoint)
	if err != nil {
		return err
	}

	return os.WriteFile(p.config.CheckpointFile, data, 0644)
}

// removeCheckpoint removes the checkpoint file
func (p *BatchProcessor) removeCheckpoint() {
	os.Remove(p.config.CheckpointFile)
}

// EstimateTimeRemaining estimates completion time based on current progress
func (p *BatchProcessor) EstimateTimeRemaining() time.Duration {
	progress := p.batchProgress.Value()
	if progress <= 0 {
		return 0
	}

	processed := p.recordsProcessed.Value()
	if processed <= 0 {
		return 0
	}

	// Calculate rate
	duration := p.batchDuration.Sum() // milliseconds
	if duration <= 0 {
		return 0
	}

	rate := processed / (duration / 1000) // records per second
	remaining := (100 - progress) / progress * processed

	return time.Duration(remaining/rate) * time.Second
}

// GetThroughput calculates current processing throughput
func (p *BatchProcessor) GetThroughput() float64 {
	processed := p.recordsProcessed.Value()
	duration := p.batchDuration.Sum() / 1000 // convert to seconds

	if duration <= 0 {
		return 0
	}

	return processed / duration
}

// GetErrorRate calculates the current error rate
func (p *BatchProcessor) GetErrorRate() float64 {
	total := p.recordsProcessed.Value()
	if total <= 0 {
		return 0
	}

	errors := p.recordsError.Value()
	return (errors / total) * 100
}

// GetBatchSuccessRate calculates batch success rate
func (p *BatchProcessor) GetBatchSuccessRate() float64 {
	total := p.batchesTotal.Value()
	if total <= 0 {
		return 100
	}

	retries := p.batchRetries.Value()
	// Assuming retries indicate initial failures
	failures := math.Min(retries, total)
	successes := total - failures

	return (successes / total) * 100
}
