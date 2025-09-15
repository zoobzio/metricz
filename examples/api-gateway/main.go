package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zoobzio/metricz"
)

// MockBackend simulates a backend service
type MockBackend struct {
	name      string
	port      int
	latency   time.Duration
	errorRate float64
	server    *http.Server

	// Black Friday simulation - payment service degradation
	degrading     bool
	degradedSince time.Time
	mu            sync.RWMutex
}

// NewMockBackend creates a mock backend service
func NewMockBackend(name string, port int, latency time.Duration, errorRate float64) *MockBackend {
	return &MockBackend{
		name:      name,
		port:      port,
		latency:   latency,
		errorRate: errorRate,
	}
}

// Start begins serving the mock backend
func (b *MockBackend) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Black Friday scenario: payment service degradation
		b.mu.RLock()
		degraded := b.degrading
		b.mu.RUnlock()

		var actualLatency time.Duration
		var actualErrorRate float64

		if degraded && b.name == "payment" {
			// Payment service under extreme load
			// Latency increases exponentially during Black Friday
			actualLatency = b.latency * time.Duration(2+rand.Intn(8)) // 2x-10x latency
			actualErrorRate = 0.30                                    // 30% error rate under load
		} else {
			actualLatency = b.latency
			actualErrorRate = b.errorRate
		}

		// Simulate processing time
		time.Sleep(actualLatency)

		// Simulate errors
		if rand.Float64() < actualErrorRate {
			if b.name == "payment" && degraded {
				// Payment service returns specific errors during degradation
				http.Error(w, "Payment gateway timeout - provider overwhelmed", http.StatusGatewayTimeout)
			} else {
				http.Error(w, fmt.Sprintf("%s service error", b.name), http.StatusInternalServerError)
			}
			return
		}

		// Success response
		response := map[string]interface{}{
			"service":   b.name,
			"message":   "success",
			"timestamp": time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	b.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", b.port),
		Handler: mux,
	}

	log.Printf("Starting %s backend on port %d", b.name, b.port)
	return b.server.ListenAndServe()
}

// Shutdown stops the mock backend
func (b *MockBackend) Shutdown(ctx context.Context) error {
	if b.server != nil {
		return b.server.Shutdown(ctx)
	}
	return nil
}

// StartDegradation simulates service degradation
func (b *MockBackend) StartDegradation() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.degrading = true
	b.degradedSince = time.Now()
}

// StopDegradation stops service degradation
func (b *MockBackend) StopDegradation() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.degrading = false
}

// main demonstrates the corrected gateway with static keys
func main() {
	var scenario string
	flag.StringVar(&scenario, "scenario", "normal", "Run scenario: normal or blackfriday")
	flag.Parse()

	if scenario == "blackfriday" {
		fmt.Println("=== BLACK FRIDAY MELTDOWN SCENARIO ===")
		fmt.Println("")
		fmt.Println("📅 November 24, 2023 - 09:00 AM PST")
		fmt.Println("The biggest shopping day of the year just began...")
		fmt.Println("")
		time.Sleep(2 * time.Second)
	} else {
		fmt.Println("=== API Gateway with Static Keys Example ===")
	}

	fmt.Println()

	// Create metrics registry
	registry := metricz.New()

	// Create gateway with static metrics
	gateway := NewGateway(registry)

	// Black Friday scenario: Initialize circuit breaker if needed
	if scenario == "blackfriday" {
		gateway.EnableCircuitBreaker()
	}

	// Start mock backends
	if scenario == "blackfriday" {
		fmt.Println("🚀 Starting services for Black Friday traffic...")
	} else {
		fmt.Println("Starting mock backends...")
	}

	authBackend := NewMockBackend("auth", 8001, 30*time.Millisecond, 0.02)
	userBackend := NewMockBackend("user", 8002, 50*time.Millisecond, 0.05)
	orderBackend := NewMockBackend("order", 8003, 100*time.Millisecond, 0.10)
	paymentBackend := NewMockBackend("payment", 8004, 150*time.Millisecond, 0.01) // Usually reliable

	// Start backends in goroutines
	go func() {
		if err := authBackend.Start(); err != http.ErrServerClosed {
			log.Printf("Auth backend error: %v", err)
		}
	}()

	go func() {
		if err := userBackend.Start(); err != http.ErrServerClosed {
			log.Printf("User backend error: %v", err)
		}
	}()

	go func() {
		if err := orderBackend.Start(); err != http.ErrServerClosed {
			log.Printf("Order backend error: %v", err)
		}
	}()

	go func() {
		if err := paymentBackend.Start(); err != http.ErrServerClosed {
			log.Printf("Payment backend error: %v", err)
		}
	}()

	// Wait for backends to start
	time.Sleep(1 * time.Second)

	// Create HTTP server with gateway
	mux := http.NewServeMux()

	// Route to specific services with static metrics
	mux.HandleFunc("/api/auth/", gateway.ProxyHandler("auth", "http://localhost:8001"))
	mux.HandleFunc("/api/user/", gateway.ProxyHandler("user", "http://localhost:8002"))
	mux.HandleFunc("/api/order/", gateway.ProxyHandler("order", "http://localhost:8003"))
	mux.HandleFunc("/api/payment/", gateway.ProxyHandler("payment", "http://localhost:8004"))

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := gateway.GetHealth()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := map[string]interface{}{
			"counters":   registry.GetCounters(),
			"gauges":     registry.GetGauges(),
			"timers":     registry.GetTimers(),
			"histograms": registry.GetHistograms(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	if scenario == "blackfriday" {
		fmt.Println("\n💳 Gateway ready for Black Friday traffic on port 8080")
		fmt.Println("📊 Monitoring dashboard: http://localhost:8080/metrics")
		fmt.Println("")
	} else {
		fmt.Println("Gateway starting on port 8080...")
		fmt.Println("Available endpoints:")
		fmt.Println("  - GET /api/auth/...")
		fmt.Println("  - GET /api/user/...")
		fmt.Println("  - GET /api/order/...")
		fmt.Println("  - GET /api/payment/...")
		fmt.Println("  - GET /health")
		fmt.Println("  - GET /metrics")
		fmt.Println()
	}

	// Start traffic generator
	if scenario == "blackfriday" {
		go blackFridayTraffic(gateway, paymentBackend)
	} else {
		go generateTraffic()
	}

	// Graceful shutdown
	go func() {
		if scenario == "blackfriday" {
			time.Sleep(45 * time.Second) // Run Black Friday scenario longer
		} else {
			time.Sleep(30 * time.Second) // Run demo for 30 seconds
		}

		fmt.Println("\nShutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Shutdown gateway
		server.Shutdown(ctx)

		// Shutdown backends
		authBackend.Shutdown(ctx)
		userBackend.Shutdown(ctx)
		orderBackend.Shutdown(ctx)
		paymentBackend.Shutdown(ctx)

		// Show final metrics
		if scenario == "blackfriday" {
			showBlackFridayAnalysis(registry)
		} else {
			showFinalMetrics(registry)
		}
	}()

	// Start server
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// generateTraffic simulates realistic API traffic
func generateTraffic() {
	time.Sleep(2 * time.Second) // Wait for server to start

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	endpoints := []string{
		"http://localhost:8080/api/auth/login",
		"http://localhost:8080/api/user/profile",
		"http://localhost:8080/api/order/list",
		"http://localhost:8080/api/user/settings",
		"http://localhost:8080/api/order/create",
		"http://localhost:8080/api/auth/refresh",
	}

	fmt.Println("Generating traffic for 25 seconds...")

	for i := 0; i < 200; i++ {
		for _, endpoint := range endpoints {
			go func(url string) {
				resp, err := client.Get(url)
				if err == nil {
					resp.Body.Close()
				}
			}(endpoint)

			// Vary request rate
			if i < 50 {
				time.Sleep(100 * time.Millisecond) // Slow start
			} else if i < 150 {
				time.Sleep(50 * time.Millisecond) // Peak traffic
			} else {
				time.Sleep(200 * time.Millisecond) // Wind down
			}
		}
	}
}

// showFinalMetrics displays the collected metrics
func showFinalMetrics(registry *metricz.Registry) {
	fmt.Println("\n=== Final Metrics Report ===")

	// Get all metrics
	counters := registry.GetCounters()
	gauges := registry.GetGauges()
	timers := registry.GetTimers()

	fmt.Println("\nService Request Counts:")
	for name, counter := range counters {
		if name == "requests_auth" || name == "requests_user" || name == "requests_order" || name == "requests_payment" {
			service := name[9:] // Remove "requests_" prefix
			fmt.Printf("  %s: %.0f requests\n", service, counter.Value())
		}
	}

	fmt.Println("\nService Error Counts:")
	for name, counter := range counters {
		if name == "errors_auth" || name == "errors_user" || name == "errors_order" || name == "errors_payment" {
			service := name[7:] // Remove "errors_" prefix
			fmt.Printf("  %s: %.0f errors\n", service, counter.Value())
		}
	}

	fmt.Println("\nService Latencies:")
	for name, timer := range timers {
		if name == "latency_auth" || name == "latency_user" || name == "latency_order" || name == "latency_payment" {
			service := name[8:] // Remove "latency_" prefix
			if timer.Count() > 0 {
				avg := timer.Sum() / float64(timer.Count())
				fmt.Printf("  %s: %.1fms avg (%.0f requests)\n", service, avg, float64(timer.Count()))
			}
		}
	}

	fmt.Printf("\nActive Requests: %.0f\n", gauges["gateway_active_requests"].Value())

	fmt.Println("\n=== Static Keys Success! ===")
	fmt.Println("This example shows how to use static, predefined metric keys")
	fmt.Println("instead of dynamically generated ones. Key benefits:")
	fmt.Println("  - Predictable memory usage")
	fmt.Println("  - No metric explosion")
	fmt.Println("  - Better performance")
	fmt.Println("  - Easier debugging")
}

// blackFridayTraffic simulates the Black Friday meltdown scenario
func blackFridayTraffic(gateway *Gateway, paymentBackend *MockBackend) {
	time.Sleep(2 * time.Second) // Wait for server to start

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Track failed payments for narrative
	var failedPayments atomic.Int64
	var successfulPayments atomic.Int64

	fmt.Println("\n🛍️  09:15 AM - Traffic starts ramping up...")
	fmt.Println("   Early bird shoppers hitting the payment gateway")

	// Phase 1: Normal traffic (5 seconds)
	for i := 0; i < 50; i++ {
		go func() {
			resp, err := client.Get("http://localhost:8080/api/payment/checkout")
			if err == nil {
				if resp.StatusCode == 200 {
					successfulPayments.Add(1)
				} else {
					failedPayments.Add(1)
				}
				resp.Body.Close()
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(3 * time.Second)

	fmt.Println("\n⚠️  09:45 AM - ALERT: Payment latency climbing!")
	fmt.Println("   P99 latency: 500ms → 1,200ms")
	fmt.Println("   Support tickets starting to come in...")

	// Phase 2: Traffic surge begins, payment service starts degrading
	paymentBackend.StartDegradation()

	// Aggressive retry storm without circuit breaker
	for i := 0; i < 100; i++ {
		for j := 0; j < 5; j++ { // Each client retries 5 times
			go func() {
				resp, err := client.Get("http://localhost:8080/api/payment/checkout")
				if err == nil {
					if resp.StatusCode == 200 {
						successfulPayments.Add(1)
					} else {
						failedPayments.Add(1)
					}
					resp.Body.Close()
				} else {
					failedPayments.Add(1)
				}
			}()
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(5 * time.Second)

	fmt.Println("\n🔥 10:15 AM - CRITICAL: Payment gateway in meltdown!")
	fmt.Println("   P99 latency: 4,800ms!")
	fmt.Println("   Connection pool exhausted")
	fmt.Println("   Failed transactions:", failedPayments.Load())
	fmt.Println("   Revenue impact: $", failedPayments.Load()*127) // Average order value

	time.Sleep(3 * time.Second)

	fmt.Println("\n🔧 10:30 AM - INTERVENTION: Engineering enables circuit breaker")
	fmt.Println("   Circuit breaker pattern detecting failures...")
	fmt.Println("   Stopping retry storm to let payment service recover")

	// Circuit breaker engages - traffic normalizes
	gateway.TripCircuitBreaker("payment")

	time.Sleep(5 * time.Second)

	fmt.Println("\n✅ 10:45 AM - RECOVERY: Payment service stabilizing")
	fmt.Println("   Circuit breaker preventing cascade failures")
	fmt.Println("   P99 latency dropping back to normal")

	// Payment service recovers
	paymentBackend.StopDegradation()
	gateway.ResetCircuitBreaker("payment")

	// Normal traffic resumes
	for i := 0; i < 50; i++ {
		go func() {
			resp, err := client.Get("http://localhost:8080/api/payment/checkout")
			if err == nil {
				if resp.StatusCode == 200 {
					successfulPayments.Add(1)
				}
				resp.Body.Close()
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(3 * time.Second)

	fmt.Println("\n💰 11:00 AM - Back to normal operations")
	fmt.Println("   Revenue saved by circuit breaker: $", successfulPayments.Load()*127)
}

// showBlackFridayAnalysis shows the post-mortem analysis
func showBlackFridayAnalysis(registry *metricz.Registry) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 BLACK FRIDAY POST-MORTEM ANALYSIS")
	fmt.Println(strings.Repeat("=", 60))

	// Get all metrics
	timers := registry.GetTimers()
	counters := registry.GetCounters()
	histograms := registry.GetHistograms()

	fmt.Println("\n🔍 WHAT THE METRICS REVEALED:")
	fmt.Println("\n1. PAYMENT SERVICE DEGRADATION:")

	if paymentTimer, ok := timers["latency_payment"]; ok && paymentTimer.Count() > 0 {
		avg := paymentTimer.Sum() / float64(paymentTimer.Count())
		fmt.Printf("   • Average latency during incident: %.1fms\n", avg)
		fmt.Printf("   • Total payment requests: %.0f\n", float64(paymentTimer.Count()))
	}

	if paymentErrors, ok := counters["errors_payment"]; ok {
		if paymentRequests, ok := counters["requests_payment"]; ok {
			errorRate := (paymentErrors.Value() / paymentRequests.Value()) * 100
			fmt.Printf("   • Error rate: %.1f%%\n", errorRate)
			fmt.Printf("   • Failed transactions: %.0f\n", paymentErrors.Value())
		}
	}

	fmt.Println("\n2. CONNECTION POOL EXHAUSTION:")
	if poolStats, ok := counters["connection_pool_exhausted"]; ok {
		fmt.Printf("   • Pool exhaustion events: %.0f\n", poolStats.Value())
	}
	if activeConns, ok := counters["connection_pool_active"]; ok {
		fmt.Printf("   • Peak active connections: %.0f\n", activeConns.Value())
	}

	fmt.Println("\n3. CIRCUIT BREAKER IMPACT:")
	if cbTrips, ok := counters["circuit_breaker_trips"]; ok {
		fmt.Printf("   • Circuit breaker engagements: %.0f\n", cbTrips.Value())
	}
	if cbSaves, ok := counters["circuit_breaker_requests_saved"]; ok {
		fmt.Printf("   • Requests saved from retry storm: %.0f\n", cbSaves.Value())
	}

	fmt.Println("\n4. LATENCY DISTRIBUTION:")
	if p99Hist, ok := histograms["payment_latency_distribution"]; ok {
		buckets, counts := p99Hist.Buckets()
		total := p99Hist.Count()
		fmt.Printf("   • Total requests: %d\n", total)
		fmt.Printf("   • Average: %.1fms\n", p99Hist.Sum()/float64(total))

		// Show distribution
		var cumulative uint64
		for i, bucket := range buckets {
			cumulative += counts[i]
			percentage := float64(cumulative) / float64(total) * 100
			if counts[i] > 0 {
				fmt.Printf("   • ≤%.0fms: %d requests (%.1f%%)\n", bucket, counts[i], percentage)
			}
		}
	}

	fmt.Println("\n📝 KEY LESSONS LEARNED:")
	fmt.Println("\n1. BEFORE (Aggressive Retries):")
	fmt.Println("   • Retry storms amplified the problem 5x")
	fmt.Println("   • Connection pool exhausted in 90 seconds")
	fmt.Println("   • P99 latency spiked to 4,800ms")
	fmt.Println("   • Cascade failure affected all services")

	fmt.Println("\n2. AFTER (Circuit Breaker):")
	fmt.Println("   • Failed fast instead of timing out")
	fmt.Println("   • Payment service recovered in 2 minutes")
	fmt.Println("   • P99 latency returned to <500ms")
	fmt.Println("   • Other services remained healthy")

	fmt.Println("\n💡 METRICS THAT SAVED BLACK FRIDAY:")
	fmt.Println("   • Real-time P99 latency monitoring")
	fmt.Println("   • Connection pool saturation alerts")
	fmt.Println("   • Circuit breaker state tracking")
	fmt.Println("   • Per-service error rate dashboards")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎯 CONCLUSION: Metrics aren't just numbers.")
	fmt.Println("   They're your early warning system for production disasters.")
	fmt.Println(strings.Repeat("=", 60))
}
