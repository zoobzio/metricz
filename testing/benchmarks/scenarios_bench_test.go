package benchmarks

import (
	"math/rand"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

// Real-world scenario benchmark keys - mirroring the examples.
const (
	// HTTP request tracking (from api-gateway example).
	HTTPRequestsKey metricz.Key = "http_requests_total"
	HTTPLatencyKey  metricz.Key = "http_request_duration"
	HTTPErrorsKey   metricz.Key = "http_errors_total"
	HTTPActiveKey   metricz.Key = "http_active_requests"

	// Batch processing (from batch-processor example).
	BatchItemsProcessedKey metricz.Key = "batch_items_processed"
	BatchProcessingTimeKey metricz.Key = "batch_processing_duration"
	BatchErrorsKey         metricz.Key = "batch_errors"
	BatchQueueSizeKey      metricz.Key = "batch_queue_size"

	// Worker pool (from worker-pool example).
	WorkerJobsProcessedKey metricz.Key = "worker_jobs_processed"
	WorkerJobDurationKey   metricz.Key = "worker_job_duration"
	WorkerUtilizationKey   metricz.Key = "worker_utilization"
	WorkerQueueDepthKey    metricz.Key = "worker_queue_depth"

	// Service mesh (from service-mesh example).
	ServiceRequestsKey metricz.Key = "service_requests"
	ServiceLatencyKey  metricz.Key = "service_latency"
	ServiceCircuitKey  metricz.Key = "service_circuit_breaker"
	ServiceHealthKey   metricz.Key = "service_health_checks"
)

// BenchmarkHTTPRequestTracking simulates HTTP gateway request tracking.
func BenchmarkHTTPRequestTracking(b *testing.B) {
	registry := metricz.New()

	requests := registry.Counter(HTTPRequestsKey)
	latency := registry.Timer(HTTPLatencyKey)
	errors := registry.Counter(HTTPErrorsKey)
	active := registry.Gauge(HTTPActiveKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Track request start.
			requests.Inc()
			active.Inc()

			// Simulate request processing with timing.
			stopwatch := latency.Start()

			// Simulate work (don't actually sleep in benchmark).
			_ = rand.Float64()

			stopwatch.Stop()

			// Track errors (5% error rate).
			if rand.Float64() < 0.05 {
				errors.Inc()
			}

			// Request complete.
			active.Dec()
		}
	})
}

// BenchmarkBatchProcessing simulates batch job processing metrics.
func BenchmarkBatchProcessing(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0}

	itemsProcessed := registry.Counter(BatchItemsProcessedKey)
	processingTime := registry.Histogram(BatchProcessingTimeKey, buckets)
	batchErrors := registry.Counter(BatchErrorsKey)
	queueSize := registry.Gauge(BatchQueueSizeKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate batch processing.
			batchSize := rand.Intn(50) + 10 // 10-60 items per batch.

			// Update queue size.
			queueSize.Add(float64(batchSize))

			// Process items in batch.
			for i := 0; i < batchSize; i++ {
				itemsProcessed.Inc()

				// Simulate processing time.
				duration := rand.Float64() * 5.0 // 0-5 seconds.
				processingTime.Observe(duration)

				// Random errors (2% failure rate).
				if rand.Float64() < 0.02 {
					batchErrors.Inc()
				}
			}

			// Batch complete, reduce queue size.
			queueSize.Add(-float64(batchSize))
		}
	})
}

// BenchmarkWorkerPoolMetrics simulates worker pool job processing.
func BenchmarkWorkerPoolMetrics(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0}

	jobsProcessed := registry.Counter(WorkerJobsProcessedKey)
	jobDuration := registry.Histogram(WorkerJobDurationKey, buckets)
	utilization := registry.Gauge(WorkerUtilizationKey)
	queueDepth := registry.Gauge(WorkerQueueDepthKey)

	utilization.Set(0) // Start with 0% utilization.

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate job processing.
			queueDepth.Inc()  // Job enters queue.
			utilization.Inc() // Worker starts processing.

			// Process job with timing.
			duration := rand.Float64() * 2.0 // 0-2 seconds.
			jobDuration.Observe(duration)
			jobsProcessed.Inc()

			// Job complete.
			queueDepth.Dec()  // Job leaves queue.
			utilization.Dec() // Worker becomes available.
		}
	})
}

// BenchmarkServiceMeshMetrics simulates service mesh monitoring.
func BenchmarkServiceMeshMetrics(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0}

	serviceRequests := registry.Counter(ServiceRequestsKey)
	serviceLatency := registry.Histogram(ServiceLatencyKey, buckets)
	circuitBreaker := registry.Gauge(ServiceCircuitKey)
	healthChecks := registry.Counter(ServiceHealthKey)

	// Initialize circuit breaker state (0 = closed, 1 = open).
	circuitBreaker.Set(0)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Service request.
			serviceRequests.Inc()

			// Measure latency.
			latency := rand.Float64() * 0.5 // 0-500ms.
			serviceLatency.Observe(latency)

			// Circuit breaker logic (open circuit on high latency).
			if latency > 0.4 {
				circuitBreaker.Set(1) // Open circuit.
			} else if rand.Float64() < 0.1 { // 10% chance to close.
				circuitBreaker.Set(0) // Close circuit.
			}

			// Health check (every ~10th operation).
			if rand.Float64() < 0.1 {
				healthChecks.Inc()
			}
		}
	})
}

// BenchmarkRealisticMixedLoad simulates real application with varied traffic patterns.
func BenchmarkRealisticMixedLoad(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.001, 0.01, 0.1, 1.0, 10.0}

	// HTTP metrics.
	httpRequests := registry.Counter(HTTPRequestsKey)
	httpLatency := registry.Timer(HTTPLatencyKey)
	httpErrors := registry.Counter(HTTPErrorsKey)

	// Worker metrics.
	jobsProcessed := registry.Counter(WorkerJobsProcessedKey)
	jobLatency := registry.Histogram(WorkerJobDurationKey, buckets)

	// System metrics.
	activeConnections := registry.Gauge(HTTPActiveKey)
	queueDepth := registry.Gauge(WorkerQueueDepthKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Realistic load distribution.
			operation := rand.Float64()

			if operation < 0.6 { //nolint:gocritic // ifElseChain: probability-based selection clearer as if-else.
				// 60% HTTP request processing.
				httpRequests.Inc()
				activeConnections.Inc()

				stopwatch := httpLatency.Start()
				// Simulate varying response times.
				if rand.Float64() < 0.1 {
					// 10% slow requests.
					time.Sleep(time.Microsecond * 10)
				}
				stopwatch.Stop()

				// 3% error rate.
				if rand.Float64() < 0.03 {
					httpErrors.Inc()
				}

				activeConnections.Dec()

			} else if operation < 0.85 {
				// 25% background job processing.
				queueDepth.Inc()

				duration := rand.Float64() * 2.0
				jobLatency.Observe(duration)
				jobsProcessed.Inc()

				queueDepth.Dec()

			} else {
				// 15% mixed operations (exports, health checks, etc.).
				switch rand.Intn(3) {
				case 0:
					_ = registry.GetCounters()
				case 1:
					_ = registry.GetGauges()
				case 2:
					_ = registry.GetHistograms()
				}
			}
		}
	})
}

// BenchmarkSpikeLoad simulates traffic spikes with 80/20 pattern.
func BenchmarkSpikeLoad(b *testing.B) {
	registry := metricz.New()

	requests := registry.Counter(HTTPRequestsKey)
	latency := registry.Timer(HTTPLatencyKey)
	active := registry.Gauge(HTTPActiveKey)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if rand.Float64() < 0.8 {
				// 80% normal load.
				requests.Inc()
				active.Inc()

				stopwatch := latency.Start()
				stopwatch.Stop()

				active.Dec()
			} else {
				// 20% spike operations (batch requests).
				batchSize := rand.Intn(10) + 5 // 5-15 requests in burst.

				for i := 0; i < batchSize; i++ {
					requests.Inc()
					active.Inc()

					stopwatch := latency.Start()
					stopwatch.Stop()

					active.Dec()
				}
			}
		}
	})
}

// BenchmarkLongRunningService simulates long-running service with periodic exports.
func BenchmarkLongRunningService(b *testing.B) {
	registry := metricz.New()
	buckets := []float64{0.01, 0.1, 1.0, 10.0}

	// Service metrics.
	requests := registry.Counter(HTTPRequestsKey)
	latency := registry.Histogram(HTTPLatencyKey, buckets)
	errors := registry.Counter(HTTPErrorsKey)
	health := registry.Counter(ServiceHealthKey)

	// Pre-populate with some data.
	for i := 0; i < 1000; i++ {
		requests.Inc()
		latency.Observe(rand.Float64())
		if i%100 == 0 {
			errors.Inc()
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	var exportCounter int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Normal request processing.
			requests.Inc()
			latency.Observe(rand.Float64() * 0.5)

			// Occasional errors.
			if rand.Float64() < 0.02 {
				errors.Inc()
			}

			// Periodic health checks and exports (simulate monitoring scrapes).
			exportCounter++
			if exportCounter%100 == 0 {
				health.Inc()

				// Export metrics (simulate Prometheus scrape).
				_ = registry.GetCounters()
				_ = registry.GetHistograms()
			}
		}
	})
}
