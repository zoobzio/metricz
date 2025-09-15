package integration

import (
	"fmt"
	"math"
	"sync"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// Test metric keys - consistent Key type usage

// TestMetricAggregation demonstrates aggregating metrics from multiple sources.
func TestMetricAggregation(t *gotesting.T) {
	// Multiple worker registries
	const numWorkers = 5
	workers := testing.NewTestRegistries(t, numWorkers)

	// Aggregator registry for rolled-up metrics
	aggregator := testing.NewTestRegistry(t)

	// Each worker processes tasks
	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    numWorkers,
		Operations: 20,
		Setup: func(workerID int) {
			// Set final worker status at start
			workers[workerID].Gauge(WorkerStatusKey).Set(1) // 1 = healthy
		},
		Operation: func(workerID, opID int) {
			registry := workers[workerID]
			registry.Counter(TasksProcessedKey).Inc()
			registry.Timer(TaskDurationKey).Record(
				time.Duration(10+workerID*2) * time.Millisecond,
			)

			// Some workers have errors
			if workerID == 2 && opID%5 == 0 {
				registry.Counter(TasksErrorsKey).Inc()
			}
		},
	})

	// Aggregate metrics from all workers
	aggregateMetrics := func() {
		var totalTasks, totalErrors float64
		var totalDuration float64
		var totalTimings uint64
		var healthyWorkers float64

		for _, worker := range workers {
			// Sum counters
			totalTasks += worker.Counter("tasks.processed").Value()
			totalErrors += worker.Counter("tasks.errors").Value()

			// Aggregate timings
			timer := worker.Timer("task.duration")
			totalDuration += timer.Sum()
			totalTimings += timer.Count()

			// Count healthy workers
			if worker.Gauge("worker.status").Value() == 1 {
				healthyWorkers++
			}
		}

		// Store aggregated metrics
		aggregator.Counter("total.tasks").Add(totalTasks - aggregator.Counter("total.tasks").Value())
		aggregator.Counter("total.errors").Add(totalErrors - aggregator.Counter("total.errors").Value())
		aggregator.Gauge("workers.healthy").Set(healthyWorkers)
		aggregator.Gauge("workers.total").Set(float64(numWorkers))

		// Calculate average task duration
		if totalTimings > 0 {
			avgDuration := totalDuration / float64(totalTimings)
			aggregator.Gauge("task.duration.avg").Set(avgDuration)
		}

		// Calculate error rate
		if totalTasks > 0 {
			errorRate := (totalErrors / totalTasks) * 100
			aggregator.Gauge("error.rate.percent").Set(errorRate)
		}
	}

	aggregateMetrics()

	// Verify aggregation
	if aggregator.Counter("total.tasks").Value() != 100 { // 5 workers * 20 tasks
		t.Errorf("Expected 100 total tasks, got %f", aggregator.Counter("total.tasks").Value())
	}

	if aggregator.Counter("total.errors").Value() != 4 { // Worker 2 had 4 errors
		t.Errorf("Expected 4 total errors, got %f", aggregator.Counter("total.errors").Value())
	}

	if aggregator.Gauge("workers.healthy").Value() != 5 {
		t.Errorf("Expected 5 healthy workers, got %f", aggregator.Gauge("workers.healthy").Value())
	}

	errorRate := aggregator.Gauge("error.rate.percent").Value()
	if errorRate != 4.0 { // 4 errors / 100 tasks * 100
		t.Errorf("Expected 4%% error rate, got %f%%", errorRate)
	}
}

// TestPercentileAggregation demonstrates percentile calculation from histograms.
func TestPercentileAggregation(t *gotesting.T) {
	registry := metricz.New()

	// Custom buckets for response times (milliseconds)
	buckets := []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000}
	histogram := registry.Histogram(ResponseTimeKey, buckets)

	// Generate a distribution of response times
	// Normal-ish distribution centered around 200ms
	observations := []float64{
		15, 20, 25, 30, 35, 40, 45, 50, 55, 60, // Fast responses
		75, 80, 85, 90, 95, 100, 110, 120, 130, 140, // Below average
		150, 175, 200, 225, 250, 275, 300, 325, 350, // Average
		150, 175, 200, 225, 250, 275, 300, 325, 350, // Average (repeated)
		400, 450, 500, 550, 600, 650, 700, 750, // Above average
		1000, 1500, 2000, 3000, // Slow responses
		5000, 7500, // Very slow outliers
	}

	for _, obs := range observations {
		histogram.Observe(obs)
	}

	// Calculate percentiles from histogram buckets
	calculatePercentile := func(bucketBounds []float64, bucketCounts []uint64, percentile float64) float64 {
		if percentile < 0 || percentile > 100 {
			return math.NaN()
		}

		var total uint64
		for _, count := range bucketCounts {
			total += count
		}

		if total == 0 {
			return 0
		}

		targetCount := uint64(float64(total) * percentile / 100.0)
		var runningCount uint64

		for i, count := range bucketCounts {
			runningCount += count
			if runningCount >= targetCount {
				// Simple linear interpolation within bucket
				if i < len(bucketBounds) {
					return bucketBounds[i]
				}
				// Last bucket (infinity), return last bound
				if i > 0 {
					return bucketBounds[i-1]
				}
			}
		}

		// Should not reach here
		return bucketBounds[len(bucketBounds)-1]
	}

	bounds, counts := histogram.Buckets()

	// Calculate common percentiles
	p50 := calculatePercentile(bounds, counts, 50)
	p95 := calculatePercentile(bounds, counts, 95)
	p99 := calculatePercentile(bounds, counts, 99)

	// Store percentiles as gauges for monitoring
	registry.Gauge(ResponseTimeP50Key).Set(p50)
	registry.Gauge(ResponseTimeP95Key).Set(p95)
	registry.Gauge(ResponseTimeP99Key).Set(p99)

	// Verify percentiles are reasonable
	if p50 <= 0 || p50 > 500 {
		t.Errorf("P50 (%f) outside expected range", p50)
	}

	if p95 <= p50 {
		t.Errorf("P95 (%f) should be greater than P50 (%f)", p95, p50)
	}

	if p99 <= p95 {
		t.Errorf("P99 (%f) should be greater than P95 (%f)", p99, p95)
	}

	t.Logf("Response time percentiles: P50=%fms, P95=%fms, P99=%fms", p50, p95, p99)
}

// TestRollingWindowAggregation demonstrates time-based metric aggregation.
func TestRollingWindowAggregation(t *gotesting.T) {
	registry := metricz.New()

	// Sliding window for rate calculation
	type window struct {
		samples []float64
		times   []time.Time
		maxAge  time.Duration
		mu      sync.Mutex
	}

	newWindow := func(maxAge time.Duration) *window {
		return &window{
			samples: make([]float64, 0),
			times:   make([]time.Time, 0),
			maxAge:  maxAge,
		}
	}

	// Add sample to window
	addSample := func(w *window, value float64) {
		w.mu.Lock()
		defer w.mu.Unlock()

		now := time.Now()
		w.samples = append(w.samples, value)
		w.times = append(w.times, now)

		// Remove old samples
		cutoff := now.Add(-w.maxAge)
		validIdx := 0
		for i, t := range w.times {
			if t.After(cutoff) {
				validIdx = i
				break
			}
		}

		if validIdx > 0 {
			w.samples = w.samples[validIdx:]
			w.times = w.times[validIdx:]
		}
	}

	// Calculate rate from window
	calculateRate := func(w *window) float64 {
		w.mu.Lock()
		defer w.mu.Unlock()

		if len(w.samples) < 2 {
			return 0
		}

		duration := w.times[len(w.times)-1].Sub(w.times[0]).Seconds()
		if duration <= 0 {
			return 0
		}

		var sum float64
		for _, s := range w.samples {
			sum += s
		}

		return sum / duration // rate per second
	}

	// Track request rate over 100ms window
	requestWindow := newWindow(100 * time.Millisecond)

	// Generate traffic with varying rate
	go func() {
		// Burst of traffic
		for i := 0; i < 50; i++ {
			registry.Counter(RequestsKey).Inc()
			addSample(requestWindow, 1)
			time.Sleep(2 * time.Millisecond)
		}

		// Slow period
		time.Sleep(50 * time.Millisecond)

		// Another burst
		for i := 0; i < 30; i++ {
			registry.Counter(RequestsKey).Inc()
			addSample(requestWindow, 1)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Monitor rate over time
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	var maxRate float64
	samples := 0

	for samples < 10 {
		<-ticker.C
		rate := calculateRate(requestWindow)
		registry.Gauge(RequestRateKey).Set(rate)

		if rate > maxRate {
			maxRate = rate
			registry.Gauge(RequestRateMaxKey).Set(maxRate)
		}

		samples++
		t.Logf("Sample %d: Rate = %.2f req/s", samples, rate)
	}

	// Verify we captured rate variations
	if maxRate == 0 {
		t.Error("Failed to capture any request rate")
	}

	totalRequests := registry.Counter(RequestsKey).Value()
	if totalRequests < 50 {
		t.Errorf("Expected at least 50 requests, got %f", totalRequests)
	}
}

// TestMultiDimensionalAggregation demonstrates aggregating metrics with labels.
func TestMultiDimensionalAggregation(t *gotesting.T) {
	// Simulate metrics with dimensions (method, status, endpoint)
	type metricKey struct { //nolint:govet // fieldalignment - readability over micro-optimization in tests
		method   string
		status   int
		endpoint string
	}

	// Each combination gets its own registry (simulating labels)
	registries := make(map[metricKey]*metricz.Registry)

	// Generate metrics for different combinations
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	statuses := []int{200, 201, 400, 404, 500}
	endpoints := []string{"/users", "/posts", "/comments"}

	for _, method := range methods {
		for _, status := range statuses {
			for _, endpoint := range endpoints {
				key := metricKey{method, status, endpoint}
				registries[key] = metricz.New()

				// Simulate some traffic
				count := (len(method)+status+len(endpoint))%20 + 1
				for i := 0; i < count; i++ {
					registries[key].Counter("requests").Inc()

					// Add some latency observations
					latency := float64(status/10 + len(endpoint))
					registries[key].Histogram("latency", metricz.DefaultLatencyBuckets).Observe(latency)
				}
			}
		}
	}

	// Aggregate by different dimensions
	aggregator := metricz.New()

	// Aggregate by method
	methodTotals := make(map[string]float64)
	for key, reg := range registries {
		methodTotals[key.method] += reg.Counter(metricz.Key("requests")).Value()
	}

	for method, total := range methodTotals {
		metricName := fmt.Sprintf("requests.by_method.%s", method)
		aggregator.Gauge(metricz.Key(metricName)).Set(total)
	}

	// Aggregate by status code class (2xx, 4xx, 5xx)
	statusClassTotals := make(map[string]float64)
	for key, reg := range registries {
		class := fmt.Sprintf("%dxx", key.status/100)
		statusClassTotals[class] += reg.Counter(metricz.Key("requests")).Value()
	}

	for class, total := range statusClassTotals {
		metricName := fmt.Sprintf("requests.by_status.%s", class)
		aggregator.Gauge(metricz.Key(metricName)).Set(total)
	}

	// Calculate error rate per endpoint
	for _, endpoint := range endpoints {
		var totalRequests, errorRequests float64

		for key, reg := range registries {
			if key.endpoint == endpoint {
				count := reg.Counter(metricz.Key("requests")).Value()
				totalRequests += count

				if key.status >= 400 {
					errorRequests += count
				}
			}
		}

		if totalRequests > 0 {
			errorRate := (errorRequests / totalRequests) * 100
			metricName := fmt.Sprintf("error_rate%s", endpoint) // endpoint already has /
			aggregator.Gauge(metricz.Key(metricName)).Set(errorRate)
		}
	}

	// Verify aggregations
	if aggregator.Gauge(metricz.Key("requests.by_method.GET")).Value() == 0 {
		t.Error("GET method should have requests")
	}

	if aggregator.Gauge(metricz.Key("requests.by_status.2xx")).Value() == 0 {
		t.Error("2xx status class should have requests")
	}

	// Error rates should be calculated
	for _, endpoint := range endpoints {
		metricName := fmt.Sprintf("error_rate%s", endpoint)
		errorRate := aggregator.Gauge(metricz.Key(metricName)).Value()

		if errorRate < 0 || errorRate > 100 {
			t.Errorf("Invalid error rate for %s: %f%%", endpoint, errorRate)
		}
	}
}

// TestCascadingAggregation demonstrates multi-level metric aggregation.
func TestCascadingAggregation(t *gotesting.T) {
	// Three-tier aggregation: instances -> services -> cluster

	// Tier 1: Individual instances
	instances := make([]*metricz.Registry, 9)
	for i := range instances {
		instances[i] = metricz.New()
	}

	// Tier 2: Service-level (3 services, 3 instances each)
	services := make([]*metricz.Registry, 3)
	for i := range services {
		services[i] = metricz.New()
	}

	// Tier 3: Cluster-level
	cluster := metricz.New()

	// Generate instance metrics
	for i, inst := range instances {
		_ = i / 3 // serviceID not used in this simplified test

		// Each instance processes different load
		load := float64(100 + i*10)
		inst.Counter("processed").Add(load)
		inst.Gauge("cpu.usage").Set(20 + float64(i*5))
		inst.Gauge("memory.usage").Set(30 + float64(i*3))

		// Some instances have errors
		if i%4 == 0 {
			inst.Counter("errors").Add(float64(i + 1))
		}
	}

	// Aggregate instances to services
	for svcID, svc := range services {
		var totalProcessed, totalErrors float64
		var totalCPU, totalMemory float64
		var instanceCount float64

		// Aggregate from this service's instances
		for i := svcID * 3; i < (svcID+1)*3; i++ {
			inst := instances[i]
			totalProcessed += inst.Counter("processed").Value()
			totalErrors += inst.Counter("errors").Value()
			totalCPU += inst.Gauge("cpu.usage").Value()
			totalMemory += inst.Gauge("memory.usage").Value()
			instanceCount++
		}

		// Service-level metrics
		svc.Counter("total.processed").Add(totalProcessed)
		svc.Counter("total.errors").Add(totalErrors)
		svc.Gauge("avg.cpu").Set(totalCPU / instanceCount)
		svc.Gauge("avg.memory").Set(totalMemory / instanceCount)
		svc.Gauge("instance.count").Set(instanceCount)

		// Calculate service health score
		errorRate := float64(0)
		if totalProcessed > 0 {
			errorRate = totalErrors / totalProcessed
		}
		healthScore := 100 * (1 - errorRate)
		svc.Gauge("health.score").Set(healthScore)
	}

	// Aggregate services to cluster
	var clusterProcessed, clusterErrors float64
	var clusterCPU, clusterMemory float64
	var serviceCount float64
	var minHealth float64 = 100

	for _, svc := range services {
		clusterProcessed += svc.Counter("total.processed").Value()
		clusterErrors += svc.Counter("total.errors").Value()
		clusterCPU += svc.Gauge("avg.cpu").Value()
		clusterMemory += svc.Gauge("avg.memory").Value()
		serviceCount++

		health := svc.Gauge("health.score").Value()
		if health < minHealth {
			minHealth = health
		}
	}

	// Cluster-level metrics
	cluster.Counter("total.processed").Add(clusterProcessed)
	cluster.Counter("total.errors").Add(clusterErrors)
	cluster.Gauge("avg.cpu").Set(clusterCPU / serviceCount)
	cluster.Gauge("avg.memory").Set(clusterMemory / serviceCount)
	cluster.Gauge("service.count").Set(serviceCount)
	cluster.Gauge("min.health.score").Set(minHealth)

	// Calculate cluster efficiency
	maxPossible := float64(len(instances)) * 200 // theoretical max
	efficiency := (clusterProcessed / maxPossible) * 100
	cluster.Gauge("efficiency.percent").Set(efficiency)

	// Verify cascading aggregation
	expectedProcessed := float64(0)
	for i := 0; i < 9; i++ {
		expectedProcessed += float64(100 + i*10)
	}

	if cluster.Counter("total.processed").Value() != expectedProcessed {
		t.Errorf("Cluster processed mismatch: expected %f, got %f",
			expectedProcessed, cluster.Counter("total.processed").Value())
	}

	// Verify service count
	if cluster.Gauge("service.count").Value() != 3 {
		t.Errorf("Expected 3 services, got %f", cluster.Gauge("service.count").Value())
	}

	// Health score should reflect the instance with errors
	if minHealth >= 100 {
		t.Error("Min health score should be less than 100 due to errors")
	}

	t.Logf("Cluster metrics: Processed=%f, Efficiency=%.2f%%, MinHealth=%.2f%%",
		cluster.Counter("total.processed").Value(),
		cluster.Gauge("efficiency.percent").Value(),
		cluster.Gauge("min.health.score").Value())
}
