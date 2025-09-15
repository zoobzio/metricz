package integration

import (
	"context"
	"fmt"
	"sync"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// TestServiceWithMetrics demonstrates a service with comprehensive metrics.
// This shows the pattern AEGIS services should follow.
func TestServiceWithMetrics(t *gotesting.T) {
	// Each service gets its own registry - complete isolation
	registry := testing.NewTestRegistry(t)

	// Simulate HTTP server with metrics
	type request struct {
		path   string
		cached bool
		dbTime time.Duration
	}

	// Process request with full metric tracking
	processRequest := func(req request) {
		// Track inflight requests
		registry.Gauge(metricz.Key("http.requests.inflight")).Inc()
		defer registry.Gauge(metricz.Key("http.requests.inflight")).Dec()

		// Start timing the request
		stopwatch := registry.Timer(metricz.Key("http.requests.duration")).Start()
		defer stopwatch.Stop()

		// Count the request
		registry.Counter(metricz.Key("http.requests.total")).Inc()

		// Check cache
		if req.cached {
			registry.Counter(metricz.Key("cache.hits")).Inc()
		} else {
			registry.Counter(metricz.Key("cache.misses")).Inc()

			// Simulate database query
			registry.Counter(metricz.Key("db.queries.total")).Inc()
			dbTimer := registry.Timer(metricz.Key("db.queries.duration")).Start()
			time.Sleep(req.dbTime)
			dbTimer.Stop()
		}

		// Simulate processing
		time.Sleep(5 * time.Millisecond)

		// Random error simulation (10% error rate)
		if time.Now().UnixNano()%10 == 0 {
			registry.Counter(metricz.Key("http.errors.total")).Inc()
		}
	}

	// Simulate concurrent requests
	const numRequests = 100

	testing.GenerateLoad(t, testing.LoadConfig{
		Workers:    numRequests,
		Operations: 1,
		Operation: func(workerID, _ int) { // opID unused, workerID used for request variation
			req := request{
				path:   fmt.Sprintf("/api/v1/resource/%d", workerID%10),
				cached: workerID%3 == 0, // 33% cache hit rate
				dbTime: time.Duration(workerID%5+1) * time.Millisecond,
			}
			processRequest(req)
		},
	})

	// Verify metrics captured the workload
	totalRequests := registry.Counter(metricz.Key("http.requests.total")).Value()
	if totalRequests != numRequests {
		t.Errorf("Expected %d requests, got %f", numRequests, totalRequests)
	}

	// Inflight should be back to zero
	inflight := registry.Gauge(metricz.Key("http.requests.inflight")).Value()
	if inflight != 0 {
		t.Errorf("Expected 0 inflight requests after completion, got %f", inflight)
	}

	// Cache stats should add up
	cacheHits := registry.Counter(metricz.Key("cache.hits")).Value()
	cacheMisses := registry.Counter(metricz.Key("cache.misses")).Value()
	if cacheHits+cacheMisses != totalRequests {
		t.Errorf("Cache hits (%f) + misses (%f) should equal total requests (%f)",
			cacheHits, cacheMisses, totalRequests)
	}

	// DB queries should match cache misses
	dbQueries := registry.Counter(metricz.Key("db.queries.total")).Value()
	if dbQueries != cacheMisses {
		t.Errorf("DB queries (%f) should equal cache misses (%f)", dbQueries, cacheMisses)
	}

	// Timing metrics should have data
	if registry.Timer(metricz.Key("http.requests.duration")).Count() != uint64(numRequests) {
		t.Errorf("Request timer should have %d observations", numRequests)
	}

	if registry.Timer(metricz.Key("db.queries.duration")).Count() != uint64(cacheMisses) {
		t.Errorf("DB timer should have %d observations", int(cacheMisses))
	}
}

// TestServiceShutdown demonstrates graceful shutdown with metrics export.
func TestServiceShutdown(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Background service that generates metrics
	serviceRunning := true
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				serviceRunning = false
				return
			case <-ticker.C:
				registry.Counter(metricz.Key("background.ticks")).Inc()
				registry.Gauge(metricz.Key("background.active")).Set(1)
			}
		}
	}()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Export metrics before shutdown (simulating prometheus scrape)
	preShutdownMetrics := exportMetrics(registry)

	// Verify service is actively generating metrics
	if preShutdownMetrics.counters["background.ticks"] < 5 {
		t.Error("Background service should have generated ticks")
	}

	// Initiate graceful shutdown
	registry.Gauge(metricz.Key("background.active")).Set(0) // Mark as shutting down
	cancel()
	wg.Wait()

	// Final metric export after shutdown
	postShutdownMetrics := exportMetrics(registry)

	// Service should be marked as inactive
	if postShutdownMetrics.gauges["background.active"] != 0 {
		t.Error("Service should be marked as inactive after shutdown")
	}

	// Counter should have stopped increasing
	if !serviceRunning {
		t.Log("Service successfully shut down")
	}
}

// TestMultiServiceIsolation demonstrates multiple services with isolated metrics.
func TestMultiServiceIsolation(t *gotesting.T) {
	// Three microservices, each with its own registry
	apiRegistry := testing.NewTestRegistry(t)
	authRegistry := testing.NewTestRegistry(t)
	dbRegistry := testing.NewTestRegistry(t)

	// Initialize all metrics upfront (services would typically do this at startup)
	// API service metrics
	apiRegistry.Counter(metricz.Key("requests"))
	apiRegistry.Counter(metricz.Key("errors"))

	// Auth service metrics
	authRegistry.Counter(metricz.Key("logins"))
	authRegistry.Counter(metricz.Key("validations"))

	// DB service metrics
	dbRegistry.Counter(metricz.Key("queries"))
	dbRegistry.Counter(metricz.Key("connections"))

	// Simulate inter-service communication
	handleAPIRequest := func() {
		apiRegistry.Counter(metricz.Key("requests")).Inc()

		// API calls auth service
		authRegistry.Counter(metricz.Key("validations")).Inc()

		// API queries database
		dbRegistry.Counter(metricz.Key("queries")).Inc()
	}

	// Process some requests
	for i := 0; i < 10; i++ {
		handleAPIRequest()
	}

	// Each service maintains its own metrics
	if apiRegistry.Counter(metricz.Key("requests")).Value() != 10 {
		t.Error("API metrics incorrect")
	}

	if authRegistry.Counter(metricz.Key("validations")).Value() != 10 {
		t.Error("Auth metrics incorrect")
	}

	if dbRegistry.Counter(metricz.Key("queries")).Value() != 10 {
		t.Error("DB metrics incorrect")
	}

	// Services don't see each other's metrics
	if len(apiRegistry.GetCounters()) != 2 { // requests, errors
		t.Error("API registry should only have its own metrics")
	}

	if len(authRegistry.GetCounters()) != 2 { // logins, validations
		t.Error("Auth registry should only have its own metrics")
	}

	if len(dbRegistry.GetCounters()) != 2 { // queries, connections
		t.Error("DB registry should only have its own metrics")
	}
}

// TestServiceRecoveryMetrics demonstrates tracking service recovery.
func TestServiceRecoveryMetrics(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Track service health and recovery
	type serviceHealth struct { //nolint:govet // fieldalignment - logical field order more important in test structs
		healthy          bool
		lastHealthy      time.Time
		consecutiveFails int
	}

	health := &serviceHealth{healthy: true, lastHealthy: time.Now()}

	// Simulate health checks with metrics
	performHealthCheck := func(shouldFail bool) {
		registry.Counter(metricz.Key("health.checks.total")).Inc()

		if shouldFail {
			registry.Counter(metricz.Key("health.checks.failed")).Inc()
			health.consecutiveFails++
			health.healthy = false

			// Track time since last healthy
			downtime := time.Since(health.lastHealthy).Seconds()
			registry.Gauge(metricz.Key("health.downtime.seconds")).Set(downtime)
		} else {
			registry.Counter(metricz.Key("health.checks.passed")).Inc()

			// Recovery detected
			if !health.healthy && health.consecutiveFails > 0 {
				registry.Counter(metricz.Key("health.recoveries")).Inc()
				// Record recovery time
				recoveryTime := time.Since(health.lastHealthy).Seconds()
				registry.Histogram(metricz.Key("health.recovery.duration"), metricz.DefaultDurationBuckets).Observe(recoveryTime)
			}

			health.healthy = true
			health.lastHealthy = time.Now()
			health.consecutiveFails = 0
			registry.Gauge(metricz.Key("health.downtime.seconds")).Set(0)
		}

		// Update consecutive fails gauge
		registry.Gauge(metricz.Key("health.consecutive.fails")).Set(float64(health.consecutiveFails))
	}

	// Simulate failure and recovery cycle
	for i := 0; i < 5; i++ {
		performHealthCheck(false) // healthy
	}

	// Service goes down
	for i := 0; i < 3; i++ {
		performHealthCheck(true) // unhealthy
		time.Sleep(10 * time.Millisecond)
	}

	// Service recovers
	for i := 0; i < 5; i++ {
		performHealthCheck(false) // healthy again
	}

	// Verify metrics tell the story
	totalChecks := registry.Counter(metricz.Key("health.checks.total")).Value()
	if totalChecks != 13 {
		t.Errorf("Expected 13 health checks, got %f", totalChecks)
	}

	failures := registry.Counter(metricz.Key("health.checks.failed")).Value()
	if failures != 3 {
		t.Errorf("Expected 3 failures, got %f", failures)
	}

	recoveries := registry.Counter(metricz.Key("health.recoveries")).Value()
	if recoveries != 1 {
		t.Errorf("Expected 1 recovery, got %f", recoveries)
	}

	// Should be healthy now
	if registry.Gauge(metricz.Key("health.consecutive.fails")).Value() != 0 {
		t.Error("Should have zero consecutive failures after recovery")
	}

	if registry.Gauge(metricz.Key("health.downtime.seconds")).Value() != 0 {
		t.Error("Downtime should be zero after recovery")
	}
}

// TestLongRunningServiceMetrics demonstrates metrics for long-running processes.
func TestLongRunningServiceMetrics(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Batch processing service
	processBatch := func(_ int, size int) { // batchID unused, size used for metrics
		// Track batch processing
		registry.Counter(metricz.Key("batch.started")).Inc()
		registry.Gauge(metricz.Key("batch.current.size")).Set(float64(size))

		timer := registry.Timer(metricz.Key("batch.duration")).Start()
		defer func() {
			timer.Stop()
			registry.Counter(metricz.Key("batch.completed")).Inc()
			registry.Gauge(metricz.Key("batch.current.size")).Set(0)
		}()

		// Process items
		for i := 0; i < size; i++ {
			registry.Counter(metricz.Key("batch.items.processed")).Inc()

			// Simulate work
			time.Sleep(1 * time.Millisecond)

			// Track progress
			progress := float64(i+1) / float64(size) * 100
			registry.Gauge(metricz.Key("batch.progress.percent")).Set(progress)
		}
	}

	// Process batches until context expires
	batchID := 0
	for {
		select {
		case <-ctx.Done():
			// Final metrics check
			started := registry.Counter(metricz.Key("batch.started")).Value()
			completed := registry.Counter(metricz.Key("batch.completed")).Value()

			t.Logf("Processed %f batches, completed %f", started, completed)

			// There might be one in-progress batch
			if started-completed > 1 {
				t.Error("Too many incomplete batches")
			}

			// Items processed should be reasonable
			items := registry.Counter(metricz.Key("batch.items.processed")).Value()
			if items == 0 {
				t.Error("No items were processed")
			}

			return

		default:
			batchID++
			batchSize := 10 + (batchID%5)*2 // Variable batch sizes
			processBatch(batchID, batchSize)
		}
	}
}

// TestServiceDependencyMetrics demonstrates tracking service dependencies.
func TestServiceDependencyMetrics(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Track calls to external services
	type externalCall struct {
		service  string
		endpoint string
		latency  time.Duration
		success  bool
	}

	trackExternalCall := func(call externalCall) {
		// Use static keys instead of dynamic construction
		switch call.service {
		case "auth":
			registry.Counter(ExternalAuthCallsKey).Inc()
			registry.Timer(ExternalAuthLatencyKey).Record(call.latency)
			if call.success {
				registry.Counter(ExternalAuthSuccessKey).Inc()
			} else {
				registry.Counter(ExternalAuthErrorsKey).Inc()
			}
			total := registry.Counter(ExternalAuthCallsKey).Value()
			successes := registry.Counter(ExternalAuthSuccessKey).Value()
			availability := (successes / total) * 100
			registry.Gauge(ExternalAuthAvailabilityKey).Set(availability)

		case "database":
			registry.Counter(ExternalDatabaseCallsKey).Inc()
			registry.Timer(ExternalDatabaseLatencyKey).Record(call.latency)
			if call.success {
				registry.Counter(ExternalDatabaseSuccessKey).Inc()
			} else {
				registry.Counter(ExternalDatabaseErrorsKey).Inc()
			}
			total := registry.Counter(ExternalDatabaseCallsKey).Value()
			successes := registry.Counter(ExternalDatabaseSuccessKey).Value()
			availability := (successes / total) * 100
			registry.Gauge(ExternalDatabaseAvailabilityKey).Set(availability)

		case "cache":
			registry.Counter(ExternalCacheCallsKey).Inc()
			registry.Timer(ExternalCacheLatencyKey).Record(call.latency)
			if call.success {
				registry.Counter(ExternalCacheSuccessKey).Inc()
			} else {
				registry.Counter(ExternalCacheErrorsKey).Inc()
			}
			total := registry.Counter(ExternalCacheCallsKey).Value()
			successes := registry.Counter(ExternalCacheSuccessKey).Value()
			availability := (successes / total) * 100
			registry.Gauge(ExternalCacheAvailabilityKey).Set(availability)

		case "storage":
			registry.Counter(ExternalStorageCallsKey).Inc()
			registry.Timer(ExternalStorageLatencyKey).Record(call.latency)
			if call.success {
				registry.Counter(ExternalStorageSuccessKey).Inc()
			} else {
				registry.Counter(ExternalStorageErrorsKey).Inc()
			}
			total := registry.Counter(ExternalStorageCallsKey).Value()
			successes := registry.Counter(ExternalStorageSuccessKey).Value()
			availability := (successes / total) * 100
			registry.Gauge(ExternalStorageAvailabilityKey).Set(availability)
		}
	}

	// Simulate calls to various services
	services := []struct {
		name        string
		successRate float64
		baseLatency time.Duration
	}{
		{"auth", 0.99, 5 * time.Millisecond},
		{"database", 0.95, 10 * time.Millisecond},
		{"cache", 0.999, 1 * time.Millisecond},
		{"storage", 0.9, 50 * time.Millisecond},
	}

	// Generate traffic patterns
	for _, svc := range services {
		for i := 0; i < 100; i++ {
			call := externalCall{
				service:  svc.name,
				endpoint: fmt.Sprintf("/api/%d", i%5),
				latency:  svc.baseLatency + time.Duration(i%10)*time.Millisecond,
				success:  float64(i)/100 < svc.successRate,
			}
			trackExternalCall(call)
		}
	}

	// Verify dependency metrics using static keys
	for _, svc := range services {
		switch svc.name {
		case "auth":
			calls := registry.Counter(ExternalAuthCallsKey).Value()
			if calls != 100 {
				t.Errorf("Service %s: expected 100 calls, got %f", svc.name, calls)
			}
			availability := registry.Gauge(ExternalAuthAvailabilityKey).Value()
			expectedAvail := svc.successRate * 100
			if availability < expectedAvail-5 || availability > expectedAvail+5 {
				t.Errorf("Service %s: availability %f%% outside expected range around %f%%",
					svc.name, availability, expectedAvail)
			}
			if registry.Timer(ExternalAuthLatencyKey).Count() != 100 {
				t.Errorf("Service %s: latency timer missing observations", svc.name)
			}

		case "database":
			calls := registry.Counter(ExternalDatabaseCallsKey).Value()
			if calls != 100 {
				t.Errorf("Service %s: expected 100 calls, got %f", svc.name, calls)
			}
			availability := registry.Gauge(ExternalDatabaseAvailabilityKey).Value()
			expectedAvail := svc.successRate * 100
			if availability < expectedAvail-5 || availability > expectedAvail+5 {
				t.Errorf("Service %s: availability %f%% outside expected range around %f%%",
					svc.name, availability, expectedAvail)
			}
			if registry.Timer(ExternalDatabaseLatencyKey).Count() != 100 {
				t.Errorf("Service %s: latency timer missing observations", svc.name)
			}

		case "cache":
			calls := registry.Counter(ExternalCacheCallsKey).Value()
			if calls != 100 {
				t.Errorf("Service %s: expected 100 calls, got %f", svc.name, calls)
			}
			availability := registry.Gauge(ExternalCacheAvailabilityKey).Value()
			expectedAvail := svc.successRate * 100
			if availability < expectedAvail-5 || availability > expectedAvail+5 {
				t.Errorf("Service %s: availability %f%% outside expected range around %f%%",
					svc.name, availability, expectedAvail)
			}
			if registry.Timer(ExternalCacheLatencyKey).Count() != 100 {
				t.Errorf("Service %s: latency timer missing observations", svc.name)
			}

		case "storage":
			calls := registry.Counter(ExternalStorageCallsKey).Value()
			if calls != 100 {
				t.Errorf("Service %s: expected 100 calls, got %f", svc.name, calls)
			}
			availability := registry.Gauge(ExternalStorageAvailabilityKey).Value()
			expectedAvail := svc.successRate * 100
			if availability < expectedAvail-5 || availability > expectedAvail+5 {
				t.Errorf("Service %s: availability %f%% outside expected range around %f%%",
					svc.name, availability, expectedAvail)
			}
			if registry.Timer(ExternalStorageLatencyKey).Count() != 100 {
				t.Errorf("Service %s: latency timer missing observations", svc.name)
			}
		}
	}
}

// Helper function to export metrics for verification.
func exportMetrics(registry *metricz.Registry) struct {
	counters   map[string]float64
	gauges     map[string]float64
	histograms map[string]uint64
	timers     map[string]uint64
} {
	result := struct {
		counters   map[string]float64
		gauges     map[string]float64
		histograms map[string]uint64
		timers     map[string]uint64
	}{
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string]uint64),
		timers:     make(map[string]uint64),
	}

	for key, counter := range registry.GetCounters() {
		result.counters[string(key)] = counter.Value()
	}

	for key, gauge := range registry.GetGauges() {
		result.gauges[string(key)] = gauge.Value()
	}

	for key, histogram := range registry.GetHistograms() {
		result.histograms[string(key)] = histogram.Count()
	}

	for key, timer := range registry.GetTimers() {
		result.timers[string(key)] = timer.Count()
	}

	return result
}
