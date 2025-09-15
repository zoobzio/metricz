package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zoobzio/metricz"
)

func main() {
	// === SCENARIO: The Zombie Worker Apocalypse ===
	//
	// Tuesday, 3:47 AM. Pager goes off.
	// "Service degraded. Jobs backing up."
	//
	// Check the dashboard. Worker pool metrics look normal.
	// 10 workers running. Queue depth reasonable.
	// But customers complaining about delays.
	//
	// SSH into production. Run htop.
	// 47 worker processes. Should be 10.
	// Memory usage: 8.7GB. Should be 400MB.
	//
	// The zombie apocalypse has begun.

	// Parse command line flags
	var (
		scenario = "normal" // or "zombie"
		workers  = 10
	)

	if len(os.Args) > 1 {
		scenario = os.Args[1]
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	if scenario == "zombie" {
		fmt.Println("SCENARIO: Zombie Worker Apocalypse")
		fmt.Println("Simulating workers that hang and never complete...")
		fmt.Println("Watch as 'dead' workers accumulate!")
	} else {
		fmt.Println("SCENARIO: Normal Operations")
		fmt.Println("Healthy worker pool with proper lifecycle management")
	}
	fmt.Println(strings.Repeat("=", 60) + "\n")

	// Create metrics registry
	registry := metricz.New()

	// Create worker pool
	config := PoolConfig{
		Workers:     workers,
		QueueSize:   100,
		Registry:    registry,
		ZombieMode:  scenario == "zombie",
		MaxLifetime: 30 * time.Second, // Workers should recycle
	}

	pool := NewWorkerPool(config)

	// Start the pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Start metrics reporter
	stopReporter := startMetricsReporter(registry, pool, 5*time.Second)
	defer close(stopReporter)

	// Start job generator
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		generateJobs(ctx, pool)
	}()

	// Handle shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	<-shutdown
	log.Println("Shutting down...")

	// Stop generating jobs
	cancel()

	// Wait for generator to stop
	wg.Wait()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	// Final metrics report
	log.Println("\n=== Final Metrics ===")
	reportFinalMetrics(registry)
}

// generateJobs creates various job types
func generateJobs(ctx context.Context, pool *WorkerPool) {
	jobTypes := []string{
		"process_order",
		"send_email",
		"generate_report",
		"update_inventory",
		"calculate_shipping",
		"validate_payment",
		"sync_data",
		"cleanup_temp",
	}

	jobID := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Burst pattern - occasionally send many jobs at once
	burstTicker := time.NewTicker(10 * time.Second)
	defer burstTicker.Stop()

	// Toxic job pattern - jobs that might hang
	toxicTicker := time.NewTicker(15 * time.Second)
	defer toxicTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Regular job submission
			jobID++
			jobType := jobTypes[rand.Intn(len(jobTypes))]

			job := Job{
				ID:   fmt.Sprintf("job-%d", jobID),
				Type: jobType,
				Data: generateJobData(jobType),
			}

			select {
			case <-ctx.Done():
				return
			default:
				if err := pool.Submit(job); err != nil {
					if err != ErrPoolFull {
						log.Printf("Failed to submit job %s: %v", job.ID, err)
					}
					// Else silently drop - queue is full
				}
			}

		case <-burstTicker.C:
			// Burst submission
			log.Println("📈 Generating burst traffic...")
			for i := 0; i < 50; i++ {
				jobID++
				jobType := jobTypes[rand.Intn(len(jobTypes))]

				job := Job{
					ID:   fmt.Sprintf("burst-job-%d", jobID),
					Type: jobType,
					Data: generateJobData(jobType),
				}

				// Try to submit, ignore if full
				pool.Submit(job)
			}

		case <-toxicTicker.C:
			if pool.config.ZombieMode {
				// Submit toxic jobs that might hang workers
				log.Println("☠️  Injecting toxic job (might hang worker)...")
				jobID++
				job := Job{
					ID:   fmt.Sprintf("toxic-job-%d", jobID),
					Type: "toxic_operation",
					Data: map[string]interface{}{
						"toxic":     true,
						"hang_time": rand.Intn(60) + 30, // 30-90 seconds
					},
				}
				pool.Submit(job)
			}
		}
	}
}

// generateJobData creates realistic job payloads
func generateJobData(jobType string) map[string]interface{} {
	switch jobType {
	case "process_order":
		return map[string]interface{}{
			"order_id":     rand.Intn(1000000),
			"customer_id":  rand.Intn(10000),
			"total_amount": rand.Float64() * 1000,
			"items":        rand.Intn(20) + 1,
		}

	case "send_email":
		return map[string]interface{}{
			"recipient": fmt.Sprintf("user%d@example.com", rand.Intn(1000)),
			"template":  "order_confirmation",
			"retry":     rand.Intn(3),
		}

	case "generate_report":
		return map[string]interface{}{
			"report_type": "daily_sales",
			"date_range":  "last_7_days",
			"format":      "pdf",
			"size":        rand.Intn(100) + 10,  // Size affects processing time
			"complex":     rand.Float32() < 0.1, // 10% are complex reports
		}

	default:
		return map[string]interface{}{
			"type": jobType,
			"data": rand.Intn(1000),
		}
	}
}

// startMetricsReporter periodically logs metrics
func startMetricsReporter(registry *metricz.Registry, pool *WorkerPool, interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				reportMetrics(registry, pool)
			case <-stop:
				return
			}
		}
	}()

	return stop
}

// reportMetrics displays current metrics
func reportMetrics(registry *metricz.Registry, pool *WorkerPool) {
	fmt.Println("\n=== Worker Pool Metrics ===")
	fmt.Println(time.Now().Format("15:04:05"))

	// Gauges
	gauges := registry.GetGauges()
	if queueDepth, exists := gauges[QueueDepthKey]; exists {
		fmt.Printf("Queue Depth: %.0f", queueDepth.Value())
		if queueDepth.Value() > 50 {
			fmt.Print(" ⚠️  HIGH")
		}
		fmt.Println()
	}
	if activeWorkers, exists := gauges[ActiveWorkersKey]; exists {
		fmt.Printf("Active Workers: %.0f", activeWorkers.Value())
		if pool.config.ZombieMode && activeWorkers.Value() < float64(pool.config.Workers/2) {
			fmt.Print(" ⚠️  WORKERS DYING")
		}
		fmt.Println()
	}
	if zombieWorkers, exists := gauges[ZombieWorkersKey]; exists {
		zombies := zombieWorkers.Value()
		if zombies > 0 {
			fmt.Printf("Zombie Workers: %.0f 💀 CRITICAL\n", zombies)
		}
	}

	// Memory monitoring
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryMB := float64(m.Alloc) / 1024 / 1024
	fmt.Printf("Memory Usage: %.0f MB", memoryMB)
	if memoryMB > 500 {
		fmt.Print(" ⚠️  EXCESSIVE")
	}
	fmt.Println()

	// Show hung jobs if any
	if hungJobs, exists := gauges[HungJobsKey]; exists && hungJobs.Value() > 0 {
		fmt.Printf("\n🔴 ALERT: %.0f jobs hung for > 30s\n", hungJobs.Value())
	}

	// Counters - show key ones
	counters := registry.GetCounters()
	if total, exists := counters[JobsTotalKey]; exists {
		fmt.Printf("\nJobs Processed: %.0f", total.Value())
		if success, exists := counters[JobsSuccessKey]; exists {
			if total.Value() > 0 {
				rate := (success.Value() / total.Value()) * 100
				fmt.Printf(" (%.1f%% success)", rate)
				if rate < 90 {
					fmt.Print(" ⚠️")
				}
			}
		}
		fmt.Println()
	}

	if dropped, exists := counters[JobsDroppedKey]; exists && dropped.Value() > 0 {
		fmt.Printf("Jobs Dropped: %.0f ❌\n", dropped.Value())
	}

	if restarts, exists := counters[WorkerRestartsKey]; exists && restarts.Value() > 0 {
		fmt.Printf("Worker Restarts: %.0f\n", restarts.Value())
	}

	// Timers - show concerning patterns
	timers := registry.GetTimers()
	for name, timer := range timers {
		if timer.Count() > 0 && strings.Contains(string(name), "duration") {
			avg := timer.Sum() / float64(timer.Count())
			// Calculate p95
			buckets, counts := timer.Buckets()
			var p95 float64
			totalCount := timer.Count()
			p95Target := totalCount * 95 / 100
			var cumulative uint64
			for i, count := range counts {
				cumulative += count
				if cumulative >= p95Target {
					p95 = buckets[i]
					break
				}
			}

			if p95 > 5000 { // Over 5 seconds
				fmt.Printf("%s: avg=%.0fms p95=%.0fms ⚠️  SLOW\n",
					name, avg, p95)
			}
		}
	}

	fmt.Println("==========================")
}

// reportFinalMetrics shows comprehensive final statistics
func reportFinalMetrics(registry *metricz.Registry) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("POST-MORTEM ANALYSIS")
	fmt.Println(strings.Repeat("=", 60))

	// Calculate success rate
	counters := registry.GetCounters()
	var totalJobs, successfulJobs float64

	if total, exists := counters[JobsTotalKey]; exists {
		totalJobs = total.Value()
	}
	if success, exists := counters["jobs_success"]; exists {
		successfulJobs = success.Value()
	}

	successRate := float64(0)
	if totalJobs > 0 {
		successRate = (successfulJobs / totalJobs) * 100
	}

	fmt.Printf("Total Jobs: %.0f\n", totalJobs)
	fmt.Printf("Successful: %.0f\n", successfulJobs)
	fmt.Printf("Success Rate: %.2f%%\n", successRate)

	// Job type breakdown
	fmt.Println("\nJobs by Type:")
	for name, counter := range counters {
		nameStr := string(name)
		if len(nameStr) > 5 && nameStr[:5] == "jobs_" && name != JobsTotalKey &&
			name != JobsSuccessKey && name != JobsErrorKey {
			fmt.Printf("  %s: %.0f\n", nameStr[5:], counter.Value())
		}
	}

	// Processing time statistics
	fmt.Println("\nProcessing Times:")
	timers := registry.GetTimers()
	for name, timer := range timers {
		if timer.Count() > 0 {
			avg := timer.Sum() / float64(timer.Count())
			buckets, counts := timer.Buckets()

			// Find 50th and 95th percentiles
			var p50, p95 float64
			totalCount := timer.Count()
			p50Target := totalCount / 2
			p95Target := totalCount * 95 / 100

			var cumulative uint64
			for i, count := range counts {
				cumulative += count
				if p50 == 0 && cumulative >= p50Target {
					p50 = buckets[i]
				}
				if cumulative >= p95Target {
					p95 = buckets[i]
					break
				}
			}

			fmt.Printf("  %s: avg=%.2fms p50=%.2fms p95=%.2fms (n=%d)\n",
				name, avg, p50, p95, timer.Count())
		}
	}
}
