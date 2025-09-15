package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zoobzio/metricz"
)

// Metric keys for worker pool - the RIGHT way to use Key type
const (
	QueueDepthKey     metricz.Key = "queue_depth"
	ActiveWorkersKey  metricz.Key = "active_workers"
	ZombieWorkersKey  metricz.Key = "zombie_workers"
	HungJobsKey       metricz.Key = "hung_jobs"
	JobsTotalKey      metricz.Key = "jobs_total"
	JobsSuccessKey    metricz.Key = "jobs_success"
	JobsErrorKey      metricz.Key = "jobs_error"
	JobsDroppedKey    metricz.Key = "jobs_dropped"
	WorkerRestartsKey metricz.Key = "worker_restarts"

	// Job type specific keys - static, predefined
	GenerateReportJobsKey     metricz.Key = "jobs_generate_report"
	GenerateReportDurationKey metricz.Key = "job_duration_generate_report"

	SendEmailJobsKey     metricz.Key = "jobs_send_email"
	SendEmailDurationKey metricz.Key = "job_duration_send_email"

	ProcessOrderJobsKey     metricz.Key = "jobs_process_order"
	ProcessOrderDurationKey metricz.Key = "job_duration_process_order"

	SyncDataJobsKey     metricz.Key = "jobs_sync_data"
	SyncDataDurationKey metricz.Key = "job_duration_sync_data"

	ToxicJobsKey     metricz.Key = "jobs_toxic_operation"
	ToxicDurationKey metricz.Key = "job_duration_toxic_operation"

	DefaultJobsKey     metricz.Key = "jobs_default"
	DefaultDurationKey metricz.Key = "job_duration_default"
)

var (
	ErrPoolFull     = errors.New("worker pool queue is full")
	ErrPoolShutdown = errors.New("worker pool is shutting down")
)

// Job represents a unit of work
type Job struct {
	ID   string
	Type string
	Data map[string]interface{}
}

// Result represents job completion
type Result struct {
	JobID    string
	Success  bool
	Error    error
	Duration time.Duration
}

// PoolConfig configures the worker pool
type PoolConfig struct {
	Workers     int
	QueueSize   int
	Registry    *metricz.Registry
	ZombieMode  bool          // Simulate zombie workers
	MaxLifetime time.Duration // Worker max lifetime before restart
}

// WorkerPool manages concurrent job processing with metrics
type WorkerPool struct {
	config   PoolConfig
	jobs     chan Job
	results  chan Result
	shutdown chan struct{}
	wg       sync.WaitGroup

	// Metrics - static counters and timers for known job types
	registry       *metricz.Registry
	queueDepth     metricz.Gauge
	activeWorkers  metricz.Gauge
	zombieWorkers  metricz.Gauge
	hungJobs       metricz.Gauge
	totalJobs      metricz.Counter
	successJobs    metricz.Counter
	errorJobs      metricz.Counter
	droppedJobs    metricz.Counter
	workerRestarts metricz.Counter

	// Job type specific metrics - static, no dynamic creation
	generateReportJobs     metricz.Counter
	generateReportDuration metricz.Timer

	sendEmailJobs     metricz.Counter
	sendEmailDuration metricz.Timer

	processOrderJobs     metricz.Counter
	processOrderDuration metricz.Timer

	syncDataJobs     metricz.Counter
	syncDataDuration metricz.Timer

	toxicJobs     metricz.Counter
	toxicDuration metricz.Timer

	defaultJobs     metricz.Counter
	defaultDuration metricz.Timer

	// State
	isShuttingDown atomic.Bool

	// Zombie tracking
	zombieCount    atomic.Int32
	hungJobTracker map[string]time.Time
	hungMutex      sync.RWMutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config PoolConfig) *WorkerPool {
	return &WorkerPool{
		config:   config,
		jobs:     make(chan Job, config.QueueSize),
		results:  make(chan Result, config.Workers),
		shutdown: make(chan struct{}),
		registry: config.Registry,

		// Initialize static gauges and counters
		queueDepth:     config.Registry.Gauge(QueueDepthKey),
		activeWorkers:  config.Registry.Gauge(ActiveWorkersKey),
		zombieWorkers:  config.Registry.Gauge(ZombieWorkersKey),
		hungJobs:       config.Registry.Gauge(HungJobsKey),
		totalJobs:      config.Registry.Counter(JobsTotalKey),
		successJobs:    config.Registry.Counter(JobsSuccessKey),
		errorJobs:      config.Registry.Counter(JobsErrorKey),
		droppedJobs:    config.Registry.Counter(JobsDroppedKey),
		workerRestarts: config.Registry.Counter(WorkerRestartsKey),

		// Initialize job type specific metrics - static
		generateReportJobs:     config.Registry.Counter(GenerateReportJobsKey),
		generateReportDuration: config.Registry.Timer(GenerateReportDurationKey),

		sendEmailJobs:     config.Registry.Counter(SendEmailJobsKey),
		sendEmailDuration: config.Registry.Timer(SendEmailDurationKey),

		processOrderJobs:     config.Registry.Counter(ProcessOrderJobsKey),
		processOrderDuration: config.Registry.Timer(ProcessOrderDurationKey),

		syncDataJobs:     config.Registry.Counter(SyncDataJobsKey),
		syncDataDuration: config.Registry.Timer(SyncDataDurationKey),

		toxicJobs:     config.Registry.Counter(ToxicJobsKey),
		toxicDuration: config.Registry.Timer(ToxicDurationKey),

		defaultJobs:     config.Registry.Counter(DefaultJobsKey),
		defaultDuration: config.Registry.Timer(DefaultDurationKey),

		hungJobTracker: make(map[string]time.Time),
	}
}

// Start begins processing with worker goroutines
func (p *WorkerPool) Start(ctx context.Context) {
	// Start workers
	for i := 0; i < p.config.Workers; i++ {
		p.wg.Add(1)
		if p.config.ZombieMode {
			// In zombie mode, workers might fail and need restart
			go p.workerWithLifecycle(ctx, i)
		} else {
			go p.worker(ctx, i)
		}
	}

	// Start result collector
	p.wg.Add(1)
	go p.resultCollector(ctx)

	// Start queue depth monitor
	p.wg.Add(1)
	go p.queueMonitor(ctx)

	// Start hung job detector if in zombie mode
	if p.config.ZombieMode {
		p.wg.Add(1)
		go p.hungJobDetector(ctx)
	}
}

// Submit adds a job to the queue
func (p *WorkerPool) Submit(job Job) error {
	if p.isShuttingDown.Load() {
		return ErrPoolShutdown
	}

	select {
	case p.jobs <- job:
		// Track job submission
		p.totalJobs.Inc()
		p.getJobCounter(job.Type).Inc()
		return nil
	default:
		// Queue is full
		p.droppedJobs.Inc()
		return ErrPoolFull
	}
}

// Shutdown gracefully stops the pool
func (p *WorkerPool) Shutdown(ctx context.Context) error {
	p.isShuttingDown.Store(true)
	close(p.jobs)

	// Wait for workers to finish or timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(p.results) // Close results after workers done
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker processes jobs
func (p *WorkerPool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-p.jobs:
			if !ok {
				// Channel closed, shutdown
				return
			}

			// Track active worker
			p.activeWorkers.Inc()

			// Track job start time for hung detection
			if p.config.ZombieMode {
				p.hungMutex.Lock()
				p.hungJobTracker[job.ID] = time.Now()
				p.hungMutex.Unlock()
			}

			// Process job
			result := p.processJob(job)

			// Remove from hung tracker
			if p.config.ZombieMode {
				p.hungMutex.Lock()
				delete(p.hungJobTracker, job.ID)
				p.hungMutex.Unlock()
			}

			// Send result - don't block forever
			select {
			case p.results <- result:
				// Success
			case <-ctx.Done():
				p.activeWorkers.Dec()
				return
			case <-time.After(1 * time.Second):
				// Result channel might be blocked, continue
			}

			// Worker idle again
			p.activeWorkers.Dec()
		}
	}
}

// workerWithLifecycle processes jobs but might become a zombie
func (p *WorkerPool) workerWithLifecycle(ctx context.Context, id int) {
	defer p.wg.Done()

	// Worker lifetime - after this, worker becomes zombie
	lifetime := time.After(p.config.MaxLifetime)
	workerStarted := time.Now()

	for {
		select {
		case <-ctx.Done():
			return

		case <-lifetime:
			// Worker has exceeded its lifetime - becomes zombie
			if time.Now().UnixNano()%3 == 0 { // 33% chance
				// This worker becomes a zombie - it hangs forever
				p.zombieCount.Add(1)
				p.zombieWorkers.Set(float64(p.zombieCount.Load()))
				fmt.Printf("Worker %d became a zombie after %v\n", id, time.Since(workerStarted))

				// Spawn replacement (but zombie still consumes resources)
				p.workerRestarts.Inc()
				p.wg.Add(1)
				go p.workerWithLifecycle(ctx, id+p.config.Workers)

				// Zombie hangs forever
				select {
				case <-ctx.Done():
					return
				}
			}
			// Otherwise, restart normally
			p.workerRestarts.Inc()
			return

		case job, ok := <-p.jobs:
			if !ok {
				return
			}

			// Track active worker
			p.activeWorkers.Inc()

			// Track job start time
			p.hungMutex.Lock()
			p.hungJobTracker[job.ID] = time.Now()
			p.hungMutex.Unlock()

			// Process job - might hang if toxic
			if job.Type == "toxic_operation" {
				// Toxic job - might kill the worker
				if hangTime, ok := job.Data["hang_time"].(int); ok {
					if time.Now().UnixNano()%2 == 0 { // 50% chance to hang
						fmt.Printf("Worker %d hanging on toxic job for %ds...\n", id, hangTime)
						time.Sleep(time.Duration(hangTime) * time.Second)
						// This worker is now effectively dead
						p.zombieCount.Add(1)
						p.zombieWorkers.Set(float64(p.zombieCount.Load()))

						// Spawn replacement
						p.workerRestarts.Inc()
						p.wg.Add(1)
						go p.workerWithLifecycle(ctx, id+p.config.Workers*2)

						// Zombie continues to hang
						select {
						case <-ctx.Done():
							p.activeWorkers.Dec()
							return
						}
					}
				}
			}

			result := p.processJob(job)

			// Remove from hung tracker
			p.hungMutex.Lock()
			delete(p.hungJobTracker, job.ID)
			p.hungMutex.Unlock()

			// Send result
			select {
			case p.results <- result:
			case <-ctx.Done():
				p.activeWorkers.Dec()
				return
			case <-time.After(1 * time.Second):
			}

			p.activeWorkers.Dec()
		}
	}
}

// processJob simulates job processing
func (p *WorkerPool) processJob(job Job) Result {
	start := time.Now()
	timer := p.getJobTimer(job.Type)

	// Simulate processing based on job type
	var err error
	processingTime := p.simulateWork(job)
	time.Sleep(processingTime)

	// Simulate failures (5% error rate)
	if time.Now().UnixNano()%20 == 0 {
		err = fmt.Errorf("processing failed for job %s", job.ID)
	}

	// Special case: some job types are more error-prone
	if job.Type == "send_email" && time.Now().UnixNano()%10 == 0 {
		err = errors.New("SMTP server unavailable")
	}

	duration := time.Since(start)
	timer.Record(duration)

	return Result{
		JobID:    job.ID,
		Success:  err == nil,
		Error:    err,
		Duration: duration,
	}
}

// simulateWork returns processing time based on job type
func (p *WorkerPool) simulateWork(job Job) time.Duration {
	baseTime := 10 * time.Millisecond

	switch job.Type {
	case "generate_report":
		// Reports take longer, scale with size
		if complex, ok := job.Data["complex"].(bool); ok && complex {
			// Complex reports can hang in zombie mode
			if p.config.ZombieMode && time.Now().UnixNano()%5 == 0 {
				return 30 * time.Second // Hung report generation
			}
		}
		if size, ok := job.Data["size"].(int); ok {
			return baseTime * time.Duration(size)
		}
		return 500 * time.Millisecond

	case "send_email":
		// Email is usually fast
		return baseTime * 2

	case "process_order":
		// Orders have variable complexity
		if items, ok := job.Data["items"].(int); ok {
			return baseTime * time.Duration(items)
		}
		return 100 * time.Millisecond

	case "sync_data":
		// Data sync is slow
		return 200 * time.Millisecond

	case "toxic_operation":
		// Toxic operations are designed to hang
		return 10 * time.Second

	default:
		// Default processing time
		return baseTime * 5
	}
}

// resultCollector processes job results
func (p *WorkerPool) resultCollector(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		case result, ok := <-p.results:
			if !ok {
				return
			}

			// Update metrics
			if result.Success {
				p.successJobs.Inc()
			} else {
				p.errorJobs.Inc()
				// Log errors for debugging
				if result.Error != nil {
					// In production, this would go to structured logging
					fmt.Printf("Job %s failed: %v\n", result.JobID, result.Error)
				}
			}
		}
	}
}

// queueMonitor tracks queue depth
func (p *WorkerPool) queueMonitor(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.queueDepth.Set(float64(len(p.jobs)))
		}
	}
}

// getJobTimer returns static timer for a job type
func (p *WorkerPool) getJobTimer(jobType string) metricz.Timer {
	switch jobType {
	case "generate_report":
		return p.generateReportDuration
	case "send_email":
		return p.sendEmailDuration
	case "process_order":
		return p.processOrderDuration
	case "sync_data":
		return p.syncDataDuration
	case "toxic_operation":
		return p.toxicDuration
	default:
		return p.defaultDuration
	}
}

// getJobCounter returns static counter for a job type
func (p *WorkerPool) getJobCounter(jobType string) metricz.Counter {
	switch jobType {
	case "generate_report":
		return p.generateReportJobs
	case "send_email":
		return p.sendEmailJobs
	case "process_order":
		return p.processOrderJobs
	case "sync_data":
		return p.syncDataJobs
	case "toxic_operation":
		return p.toxicJobs
	default:
		return p.defaultJobs
	}
}

// hungJobDetector monitors for jobs that have been processing too long
func (p *WorkerPool) hungJobDetector(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			var hungCount int

			p.hungMutex.RLock()
			for jobID, startTime := range p.hungJobTracker {
				if now.Sub(startTime) > 30*time.Second {
					hungCount++
					fmt.Printf("Job %s has been processing for %v\n", jobID, now.Sub(startTime))
				}
			}
			p.hungMutex.RUnlock()

			p.hungJobs.Set(float64(hungCount))
		}
	}
}
