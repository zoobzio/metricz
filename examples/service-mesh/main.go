package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zoobzio/metricz"
)

func main() {
	// Create metrics registry for mesh
	registry := metricz.New()

	// Create service mesh
	mesh := NewServiceMesh(registry)

	// Register services
	log.Println("Starting services...")
	services := startServices(registry)

	for name, instances := range services {
		for i, instance := range instances {
			mesh.RegisterService(name, fmt.Sprintf("%s-%d", name, i), instance)
		}
	}

	// Start mesh proxy
	mesh.Start()

	// Start metrics reporter
	stopReporter := startMetricsReporter(registry, 5*time.Second)
	defer close(stopReporter)

	// Generate traffic
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go generateTraffic(ctx, mesh)

	// Simulate failure scenarios
	go simulateFailures(ctx, mesh, services, registry)

	// Handle shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	<-shutdown
	log.Println("Shutting down service mesh...")

	mesh.Stop()
	stopServices(services)

	// Final metrics
	fmt.Println("\n=== Final Service Mesh Metrics ===")
	reportFinalMetrics(registry)
}

// startServices creates mock service instances
func startServices(registry *metricz.Registry) map[string][]ServiceInstance {
	services := make(map[string][]ServiceInstance)

	// Auth service - 3 instances (potential leak source)
	services["auth-service"] = []ServiceInstance{
		NewAuthService(registry),
		NewAuthService(registry),
		NewAuthService(registry),
	}

	// User service - 3 instances
	services["user-service"] = []ServiceInstance{
		NewMockService("user-service", 10*time.Millisecond, 0.02),
		NewMockService("user-service", 12*time.Millisecond, 0.03),
		NewMockService("user-service", 15*time.Millisecond, 0.02),
	}

	// Order service - 2 instances
	services["order-service"] = []ServiceInstance{
		NewMockService("order-service", 50*time.Millisecond, 0.05),
		NewMockService("order-service", 45*time.Millisecond, 0.04),
	}

	// Inventory service - 2 instances
	services["inventory-service"] = []ServiceInstance{
		NewMockService("inventory-service", 20*time.Millisecond, 0.03),
		NewMockService("inventory-service", 25*time.Millisecond, 0.03),
	}

	// Payment service - 1 instance (critical service)
	services["payment-service"] = []ServiceInstance{
		NewMockService("payment-service", 100*time.Millisecond, 0.01),
	}

	return services
}

// stopServices shuts down all services
func stopServices(services map[string][]ServiceInstance) {
	for _, instances := range services {
		for _, instance := range instances {
			if mock, ok := instance.(*MockService); ok {
				mock.Stop()
			} else if auth, ok := instance.(*AuthService); ok {
				auth.Stop()
			}
		}
	}
}

// generateTraffic simulates service-to-service calls
func generateTraffic(ctx context.Context, mesh *ServiceMesh) {
	// Simulate different call patterns
	patterns := []struct {
		from    string
		to      string
		rate    time.Duration
		traceID string
	}{
		{"api-gateway", "auth-service", 80 * time.Millisecond, ""},
		{"api-gateway", "user-service", 100 * time.Millisecond, ""},
		{"api-gateway", "order-service", 150 * time.Millisecond, ""},
		{"order-service", "inventory-service", 200 * time.Millisecond, ""},
		{"order-service", "payment-service", 300 * time.Millisecond, ""},
		{"user-service", "order-service", 250 * time.Millisecond, ""},
		{"user-service", "auth-service", 120 * time.Millisecond, ""},
	}

	for _, pattern := range patterns {
		go func(p struct {
			from    string
			to      string
			rate    time.Duration
			traceID string
		}) {
			ticker := time.NewTicker(p.rate)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Generate trace ID for distributed tracing
					traceID := fmt.Sprintf("trace-%d", rand.Int63())

					req := ServiceRequest{
						From:    p.from,
						To:      p.to,
						Method:  "GET",
						Path:    fmt.Sprintf("/api/%s", p.to),
						TraceID: traceID,
					}

					resp := mesh.Route(req)

					if !resp.Success {
						log.Printf("[%s] Call from %s to %s failed: %v",
							traceID, p.from, p.to, resp.Error)
					}
				}
			}
		}(pattern)
	}

	// Simulate burst traffic periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Println("Generating burst traffic...")
				for i := 0; i < 50; i++ {
					go func() {
						req := ServiceRequest{
							From:    "burst-client",
							To:      "user-service",
							Method:  "GET",
							Path:    "/api/burst",
							TraceID: fmt.Sprintf("burst-%d", rand.Int63()),
						}
						mesh.Route(req)
					}()
				}
			}
		}
	}()
}

// simulateFailures introduces various failure scenarios
func simulateFailures(ctx context.Context, mesh *ServiceMesh, services map[string][]ServiceInstance, registry *metricz.Registry) {
	time.Sleep(15 * time.Second) // Let system stabilize

	scenarios := []func(){
		// Scenario 1: Service instance failure
		func() {
			log.Println("Scenario: User service instance failure")
			if instances, ok := services["user-service"]; ok && len(instances) > 0 {
				if mock, ok := instances[0].(*MockService); ok {
					mock.SetErrorRate(0.8) // 80% error rate
					time.Sleep(10 * time.Second)
					mock.SetErrorRate(0.02) // Restore
				}
			}
		},

		// Scenario 2: Service slowdown
		func() {
			log.Println("Scenario: Payment service slowdown")
			if instances, ok := services["payment-service"]; ok && len(instances) > 0 {
				if mock, ok := instances[0].(*MockService); ok {
					mock.SetLatency(500 * time.Millisecond) // 5x slower
					time.Sleep(10 * time.Second)
					mock.SetLatency(100 * time.Millisecond) // Restore
				}
			}
		},

		// Scenario 3: Complete service outage
		func() {
			log.Println("Scenario: Inventory service outage")
			if instances, ok := services["inventory-service"]; ok {
				for _, instance := range instances {
					if mock, ok := instance.(*MockService); ok {
						mock.SetErrorRate(1.0) // 100% failure
					}
				}
				time.Sleep(8 * time.Second)
				// Restore
				for _, instance := range instances {
					if mock, ok := instance.(*MockService); ok {
						mock.SetErrorRate(0.03)
					}
				}
			}
		},

		// Scenario 4: The Cascade That Wasn't the Database
		func() {
			cascadeScenario(ctx, mesh, services, registry)
		},
	}

	for i, scenario := range scenarios {
		select {
		case <-ctx.Done():
			return
		default:
			if i < len(scenarios)-1 {
				scenario()
				time.Sleep(5 * time.Second) // Gap between scenarios
			} else {
				// Run cascade scenario last
				time.Sleep(10 * time.Second) // Extra pause before the big one
				scenario()
			}
		}
	}
}

// startMetricsReporter periodically displays metrics
func startMetricsReporter(registry *metricz.Registry, interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				reportMetrics(registry)
			case <-stop:
				return
			}
		}
	}()

	return stop
}

// reportMetrics displays current mesh metrics
func reportMetrics(registry *metricz.Registry) {
	fmt.Println("\n=== Service Mesh Metrics ===")
	fmt.Println(time.Now().Format("15:04:05"))

	// System metrics for auth service
	gauges := registry.GetGauges()
	if authGoroutines, exists := gauges[metricz.Key("auth_goroutines")]; exists {
		fmt.Printf("\nAuth Service Health:\n")
		fmt.Printf("  Goroutines: %.0f\n", authGoroutines.Value())
		if authMemory, exists := gauges[metricz.Key("auth_memory_mb")]; exists {
			fmt.Printf("  Memory: %.1f MB\n", authMemory.Value())
		}
	}

	// Circuit breaker states
	fmt.Println("\nCircuit Breakers:")
	for name, gauge := range gauges {
		nameStr := string(name)
		if len(nameStr) > 3 && nameStr[:3] == "cb_" {
			state := "CLOSED"
			if gauge.Value() == 1 {
				state = "OPEN"
			} else if gauge.Value() == 0.5 {
				state = "HALF_OPEN"
			}
			fmt.Printf("  %s: %s\n", nameStr[3:], state)
		}
	}

	// Request counts by service
	counters := registry.GetCounters()
	fmt.Println("\nRequests by Service:")
	serviceRequests := make(map[string]float64)
	for name, counter := range counters {
		nameStr := string(name)
		if len(nameStr) > 9 && nameStr[:9] == "requests_" {
			service := nameStr[9:]
			serviceRequests[service] = counter.Value()
		}
	}
	for service, count := range serviceRequests {
		fmt.Printf("  %s: %.0f\n", service, count)
	}

	// Error rates
	fmt.Println("\nError Rates:")
	for service, requests := range serviceRequests {
		errorKey := metricz.Key(fmt.Sprintf("errors_%s", service))
		if errorCounter, exists := counters[errorKey]; exists && requests > 0 {
			errorRate := (errorCounter.Value() / requests) * 100
			fmt.Printf("  %s: %.2f%%\n", service, errorRate)
		}
	}

	// Load balancing distribution
	fmt.Println("\nLoad Distribution:")
	instanceCounts := make(map[string]map[string]float64)
	for name, counter := range counters {
		nameStr := string(name)
		if len(nameStr) > 9 && nameStr[:9] == "instance_" {
			parts := extractServiceInstance(nameStr[9:])
			if len(parts) == 2 {
				service := parts[0]
				instance := parts[1]
				if instanceCounts[service] == nil {
					instanceCounts[service] = make(map[string]float64)
				}
				instanceCounts[service][instance] = counter.Value()
			}
		}
	}

	for service, instances := range instanceCounts {
		fmt.Printf("  %s:\n", service)
		var total float64
		for _, count := range instances {
			total += count
		}
		for instance, count := range instances {
			percentage := float64(0)
			if total > 0 {
				percentage = (count / total) * 100
			}
			fmt.Printf("    %s: %.0f (%.1f%%)\n", instance, count, percentage)
		}
	}

	fmt.Println("============================")
}

// cascadeScenario implements "The Cascade That Wasn't the Database"
func cascadeScenario(ctx context.Context, mesh *ServiceMesh, services map[string][]ServiceInstance, registry *metricz.Registry) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("PRODUCTION WAR STORY: The Cascade That Wasn't the Database")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\n🕐 14:23 UTC - Black Friday. Traffic surge beginning...")
	fmt.Println("All systems green. Database performing well.")
	fmt.Println("Auth service handling authentication requests normally.")

	// Enable leak mode on auth services
	if authInstances, exists := services["auth-service"]; exists {
		for _, instance := range authInstances {
			if authService, ok := instance.(*AuthService); ok {
				authService.EnableLeakMode()
			}
		}
	}

	fmt.Println("\n🕐 14:25 UTC - Auth service goroutine leak begins...")
	fmt.Println("Token validation calls creating stuck goroutines.")
	fmt.Println("Metrics collection goroutines accumulating.")
	time.Sleep(3 * time.Second)

	// Simulate increasing traffic load
	fmt.Println("\n🕐 14:27 UTC - Traffic increasing. 15,000 RPS.")
	go intensiveTraffic(ctx, mesh, "normal")
	time.Sleep(4 * time.Second)

	fmt.Println("\n🕐 14:29 UTC - First auth instance OOM. Load shifts.")
	// Kill first auth instance
	if authInstances, exists := services["auth-service"]; exists && len(authInstances) > 0 {
		if authService, ok := authInstances[0].(*AuthService); ok {
			authService.SetHealthy(false)
		}
	}
	time.Sleep(2 * time.Second)

	fmt.Println("\n🕐 14:30 UTC - Auth traffic concentrated on 2 instances.")
	fmt.Println("Load balancer routing all auth requests to remaining instances.")
	go intensiveTraffic(ctx, mesh, "concentrated")
	time.Sleep(3 * time.Second)

	fmt.Println("\n🕐 14:31 UTC - Second auth instance failing. Retry storm begins.")
	// Simulate second instance degradation
	if authInstances, exists := services["auth-service"]; exists && len(authInstances) > 1 {
		if authService, ok := authInstances[1].(*AuthService); ok {
			authService.SetErrorRate(0.6) // 60% failure rate
		}
	}
	time.Sleep(2 * time.Second)

	fmt.Println("\n🕐 14:32 UTC - Circuit breakers opening. Retry amplification.")
	fmt.Println("Client retries creating 51,000 RPS to single healthy auth instance.")
	go intensiveTraffic(ctx, mesh, "storm")
	time.Sleep(3 * time.Second)

	fmt.Println("\n🕐 14:33 UTC - Last auth instance overwhelmed. Total cascade.")
	// Final auth instance fails
	if authInstances, exists := services["auth-service"]; exists && len(authInstances) > 2 {
		if authService, ok := authInstances[2].(*AuthService); ok {
			authService.SetErrorRate(0.9) // 90% failure
		}
	}
	time.Sleep(2 * time.Second)

	fmt.Println("\n🚨 CRITICAL: Authentication system down. All user flows failing.")
	fmt.Println("Database still healthy. Confused debugging begins...")
	time.Sleep(3 * time.Second)

	fmt.Println("\n🕐 14:35 UTC - SRE team investigates database first.")
	fmt.Println("Database queries: normal. Connection pools: healthy.")
	fmt.Println("Database metrics show no stress indicators.")
	time.Sleep(2 * time.Second)

	fmt.Println("\n🕐 14:37 UTC - Network team checks connectivity.")
	fmt.Println("Network latency: normal. Packet loss: none.")
	fmt.Println("Load balancer health checks: failing on auth only.")
	time.Sleep(2 * time.Second)

	fmt.Println("\n💡 14:39 UTC - Metrics reveal the truth:")

	// Show the shocking metrics
	if authInstances, exists := services["auth-service"]; exists {
		var totalLeaked int64
		for _, instance := range authInstances {
			if authService, ok := instance.(*AuthService); ok {
				totalLeaked += authService.GetLeakedRoutines()
			}
		}

		gauges := registry.GetGauges()
		if authGoroutines, exists := gauges[metricz.Key("auth_goroutines")]; exists {
			currentGoroutines := authGoroutines.Value()
			fmt.Printf("   🔍 Auth service goroutines: %.0f (normal: ~200)\n", currentGoroutines)
			fmt.Printf("   🔍 Leaked goroutines created: %d\n", totalLeaked)
			fmt.Printf("   🔍 Memory pressure from goroutine stacks\n")
		}
	}

	fmt.Println("   🔍 Auth instances failing due to memory exhaustion")
	fmt.Println("   🔍 NOT database. NOT network. Goroutine leak.")

	time.Sleep(3 * time.Second)

	fmt.Println("\n🔧 14:42 UTC - Emergency response: Restart auth instances.")
	fmt.Println("Disabling leak mode and restoring service health...")

	// Restore auth services
	if authInstances, exists := services["auth-service"]; exists {
		for _, instance := range authInstances {
			if authService, ok := instance.(*AuthService); ok {
				authService.DisableLeakMode()
				authService.SetErrorRate(0.02) // Normal error rate
				authService.SetHealthy(true)
			}
		}
	}

	// Stop intensive traffic
	time.Sleep(3 * time.Second)

	fmt.Println("\n✅ 14:45 UTC - Service restored. Cascade stops.")
	fmt.Println("Auth services healthy. Traffic distributed normally.")
	fmt.Println("Database never was the problem.")

	time.Sleep(2 * time.Second)

	fmt.Println("\n📊 POST-MORTEM FINDINGS:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("• Root cause: Goroutine leak in token validation")
	fmt.Println("• Trigger: JWT validation timeouts not cleaned up")
	fmt.Println("• Amplifier: Metrics collection goroutines never stopped")
	fmt.Println("• Result: Memory exhaustion → instance failures → cascade")
	fmt.Println("• Database: Completely unrelated, investigated first")
	fmt.Println("• Solution: Proper goroutine lifecycle management")

	fmt.Println("\n🎯 KEY LESSON:")
	fmt.Println("Metrics revealed the truth when traditional debugging failed.")
	fmt.Println("Goroutine count 18,000 vs normal 200 was the smoking gun.")
	fmt.Println("Always check system metrics before complex investigations.")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("End of war story. Normal operations resumed.")
	fmt.Println(strings.Repeat("=", 60))
}

// intensiveTraffic generates various traffic patterns
func intensiveTraffic(ctx context.Context, mesh *ServiceMesh, pattern string) {

	switch pattern {
	case "normal":
		// Normal Black Friday traffic
		for i := 0; i < 50; i++ {
			go func() {
				req := ServiceRequest{
					From:    "api-gateway",
					To:      "auth-service",
					Method:  "POST",
					Path:    "/auth/validate",
					TraceID: fmt.Sprintf("bf-%d", rand.Int63()),
				}
				mesh.Route(req)
			}()
			time.Sleep(20 * time.Millisecond)
		}

	case "concentrated":
		// Traffic concentrated on fewer instances
		for i := 0; i < 80; i++ {
			go func() {
				req := ServiceRequest{
					From:    "api-gateway",
					To:      "auth-service",
					Method:  "POST",
					Path:    "/auth/validate",
					TraceID: fmt.Sprintf("conc-%d", rand.Int63()),
				}
				mesh.Route(req)
			}()
			time.Sleep(10 * time.Millisecond)
		}

	case "storm":
		// Retry storm amplification
		for i := 0; i < 150; i++ {
			go func() {
				// Simulate client retries
				for retry := 0; retry < 3; retry++ {
					req := ServiceRequest{
						From:    "api-gateway",
						To:      "auth-service",
						Method:  "POST",
						Path:    "/auth/validate",
						TraceID: fmt.Sprintf("storm-%d-r%d", rand.Int63(), retry),
					}
					mesh.Route(req)
					time.Sleep(5 * time.Millisecond)
				}
			}()
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// extractServiceInstance parses service and instance from metric name
func extractServiceInstance(name string) []string {
	// Simple parsing - in production use proper parsing
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '_' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{}
}

// reportFinalMetrics shows comprehensive statistics
func reportFinalMetrics(registry *metricz.Registry) {
	counters := registry.GetCounters()

	// Total requests
	var totalRequests float64
	for name, counter := range counters {
		nameStr := string(name)
		if len(nameStr) > 9 && nameStr[:9] == "requests_" {
			totalRequests += counter.Value()
		}
	}

	fmt.Printf("Total Requests: %.0f\n", totalRequests)

	// Circuit breaker trips
	if trips, exists := counters[metricz.Key("circuit_breaker_trips")]; exists {
		fmt.Printf("Circuit Breaker Trips: %.0f\n", trips.Value())
	}

	// Retry statistics
	if retries, exists := counters[metricz.Key("retries_total")]; exists {
		fmt.Printf("Total Retries: %.0f\n", retries.Value())
	}
	if retrySuccess, exists := counters[metricz.Key("retries_success")]; exists {
		fmt.Printf("Successful Retries: %.0f\n", retrySuccess.Value())
	}

	// Latency percentiles
	timers := registry.GetTimers()
	fmt.Println("\nService Latencies (P50/P95/P99):")

	for name, timer := range timers {
		if timer.Count() == 0 {
			continue
		}

		buckets, counts := timer.Buckets()
		totalCount := timer.Count()

		var p50, p95, p99 float64
		var cumulative uint64

		for i, count := range counts {
			cumulative += count
			if p50 == 0 && cumulative >= totalCount/2 {
				p50 = buckets[i]
			}
			if p95 == 0 && cumulative >= totalCount*95/100 {
				p95 = buckets[i]
			}
			if cumulative >= totalCount*99/100 {
				p99 = buckets[i]
				break
			}
		}

		fmt.Printf("  %s: %.1f/%.1f/%.1f ms\n", name, p50, p95, p99)
	}
}
