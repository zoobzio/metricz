package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zoobzio/metricz"
)

// Gateway metric keys - the RIGHT way to use Key type
const (
	GatewayActiveRequestsKey metricz.Key = "gateway_active_requests"

	// Service-specific metric keys - static, predefined
	AuthServiceRequestsKey metricz.Key = "requests_auth"
	AuthServiceLatencyKey  metricz.Key = "latency_auth"
	AuthServiceErrorsKey   metricz.Key = "errors_auth"

	UserServiceRequestsKey metricz.Key = "requests_user"
	UserServiceLatencyKey  metricz.Key = "latency_user"
	UserServiceErrorsKey   metricz.Key = "errors_user"

	OrderServiceRequestsKey metricz.Key = "requests_order"
	OrderServiceLatencyKey  metricz.Key = "latency_order"
	OrderServiceErrorsKey   metricz.Key = "errors_order"

	PaymentServiceRequestsKey metricz.Key = "requests_payment"
	PaymentServiceLatencyKey  metricz.Key = "latency_payment"
	PaymentServiceErrorsKey   metricz.Key = "errors_payment"

	// Circuit breaker metrics
	CircuitBreakerTripsKey         metricz.Key = "circuit_breaker_trips"
	CircuitBreakerRequestsSavedKey metricz.Key = "circuit_breaker_requests_saved"

	// Connection pool metrics
	ConnectionPoolExhaustedKey metricz.Key = "connection_pool_exhausted"
	ConnectionPoolActiveKey    metricz.Key = "connection_pool_active"

	// P99 histogram
	PaymentLatencyDistributionKey metricz.Key = "payment_latency_distribution"
)

// CircuitBreaker represents a simple circuit breaker
type CircuitBreaker struct {
	tripped      bool
	failureCount int
	lastFailure  time.Time
	mu           sync.RWMutex
}

// Gateway manages API routing with comprehensive metrics
type Gateway struct {
	registry *metricz.Registry

	// Static metrics - no dynamic key creation
	authRequests metricz.Counter
	authLatency  metricz.Timer
	authErrors   metricz.Counter

	userRequests metricz.Counter
	userLatency  metricz.Timer
	userErrors   metricz.Counter

	orderRequests metricz.Counter
	orderLatency  metricz.Timer
	orderErrors   metricz.Counter

	paymentRequests  metricz.Counter
	paymentLatency   metricz.Timer
	paymentErrors    metricz.Counter
	paymentHistogram metricz.Histogram

	activeRequests metricz.Gauge

	// Circuit breaker metrics
	circuitBreakerTrips         metricz.Counter
	circuitBreakerRequestsSaved metricz.Counter

	// Connection pool metrics
	connectionPoolExhausted metricz.Counter
	connectionPoolActive    metricz.Gauge

	// Health tracking
	totalRequests atomic.Uint64
	totalErrors   atomic.Uint64

	// Circuit breakers per service
	circuitBreakers map[string]*CircuitBreaker
	cbMutex         sync.RWMutex
	cbEnabled       bool
}

// NewGateway creates a gateway with metrics
func NewGateway(registry *metricz.Registry) *Gateway {
	return &Gateway{
		registry: registry,

		// Initialize static metrics
		authRequests: registry.Counter(AuthServiceRequestsKey),
		authLatency:  registry.Timer(AuthServiceLatencyKey),
		authErrors:   registry.Counter(AuthServiceErrorsKey),

		userRequests: registry.Counter(UserServiceRequestsKey),
		userLatency:  registry.Timer(UserServiceLatencyKey),
		userErrors:   registry.Counter(UserServiceErrorsKey),

		orderRequests: registry.Counter(OrderServiceRequestsKey),
		orderLatency:  registry.Timer(OrderServiceLatencyKey),
		orderErrors:   registry.Counter(OrderServiceErrorsKey),

		paymentRequests:  registry.Counter(PaymentServiceRequestsKey),
		paymentLatency:   registry.Timer(PaymentServiceLatencyKey),
		paymentErrors:    registry.Counter(PaymentServiceErrorsKey),
		paymentHistogram: registry.Histogram(PaymentLatencyDistributionKey, []float64{100, 250, 500, 1000, 2500, 5000, 10000}),

		activeRequests: registry.Gauge(GatewayActiveRequestsKey),

		// Circuit breaker metrics
		circuitBreakerTrips:         registry.Counter(CircuitBreakerTripsKey),
		circuitBreakerRequestsSaved: registry.Counter(CircuitBreakerRequestsSavedKey),

		// Connection pool metrics
		connectionPoolExhausted: registry.Counter(ConnectionPoolExhaustedKey),
		connectionPoolActive:    registry.Gauge(ConnectionPoolActiveKey),

		// Initialize circuit breakers
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// getServiceMetrics returns the correct metrics for a known service
func (g *Gateway) getServiceMetrics(serviceName string) (metricz.Counter, metricz.Timer, metricz.Counter) {
	switch serviceName {
	case "auth":
		return g.authRequests, g.authLatency, g.authErrors
	case "user":
		return g.userRequests, g.userLatency, g.userErrors
	case "order":
		return g.orderRequests, g.orderLatency, g.orderErrors
	case "payment":
		return g.paymentRequests, g.paymentLatency, g.paymentErrors
	default:
		// For unknown services, use auth metrics as fallback
		// In production, you'd reject unknown services
		return g.authRequests, g.authLatency, g.authErrors
	}
}

// EnableCircuitBreaker enables circuit breaker functionality
func (g *Gateway) EnableCircuitBreaker() {
	g.cbMutex.Lock()
	defer g.cbMutex.Unlock()
	g.cbEnabled = true
}

// TripCircuitBreaker manually trips the circuit breaker for a service
func (g *Gateway) TripCircuitBreaker(serviceName string) {
	g.cbMutex.Lock()
	defer g.cbMutex.Unlock()

	if g.circuitBreakers[serviceName] == nil {
		g.circuitBreakers[serviceName] = &CircuitBreaker{}
	}

	g.circuitBreakers[serviceName].tripped = true
	g.circuitBreakerTrips.Inc()
}

// ResetCircuitBreaker resets the circuit breaker for a service
func (g *Gateway) ResetCircuitBreaker(serviceName string) {
	g.cbMutex.Lock()
	defer g.cbMutex.Unlock()

	if cb, ok := g.circuitBreakers[serviceName]; ok {
		cb.tripped = false
		cb.failureCount = 0
	}
}

// isCircuitOpen checks if the circuit breaker is open for a service
func (g *Gateway) isCircuitOpen(serviceName string) bool {
	g.cbMutex.RLock()
	defer g.cbMutex.RUnlock()

	if !g.cbEnabled {
		return false
	}

	if cb, ok := g.circuitBreakers[serviceName]; ok {
		return cb.tripped
	}

	return false
}

// ProxyHandler creates a handler that proxies to a backend with metrics
func (g *Gateway) ProxyHandler(serviceName, backendURL string) http.HandlerFunc {
	// Get static metrics for this service
	requestsCounter, latencyTimer, errorsCounter := g.getServiceMetrics(serviceName)

	return func(w http.ResponseWriter, r *http.Request) {
		// Check circuit breaker
		if g.isCircuitOpen(serviceName) {
			g.circuitBreakerRequestsSaved.Inc()
			http.Error(w, "Service temporarily unavailable - circuit breaker open", http.StatusServiceUnavailable)
			return
		}

		// Track active requests and connection pool
		g.activeRequests.Inc()
		g.connectionPoolActive.Inc()
		defer func() {
			g.activeRequests.Dec()
			g.connectionPoolActive.Dec()
		}()

		// Simulate connection pool limits
		if g.connectionPoolActive.Value() > 100 {
			g.connectionPoolExhausted.Inc()
		}

		// Track total requests
		g.totalRequests.Add(1)

		// Count request
		requestsCounter.Inc()

		// Start timing
		startTime := time.Now()
		stopwatch := latencyTimer.Start()
		defer func() {
			stopwatch.Stop()
			// Record to histogram for payment service
			if serviceName == "payment" {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				g.paymentHistogram.Observe(latencyMs)
			}
		}()

		// Create backend request
		proxyReq, err := http.NewRequest(r.Method, backendURL+r.URL.Path, r.Body)
		if err != nil {
			g.recordError(errorsCounter, http.StatusInternalServerError)
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for name, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(name, value)
			}
		}

		// Add tracing headers
		proxyReq.Header.Set("X-Gateway-Service", serviceName)
		proxyReq.Header.Set("X-Gateway-Time", time.Now().Format(time.RFC3339))

		// Execute request with timeout
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Do(proxyReq)
		if err != nil {
			// Backend failure

			// Check if timeout
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
				g.recordError(errorsCounter, http.StatusGatewayTimeout)
				http.Error(w, "Backend timeout", http.StatusGatewayTimeout)
			} else {
				g.recordError(errorsCounter, http.StatusBadGateway)
				http.Error(w, "Backend unavailable", http.StatusBadGateway)
			}
			return
		}
		defer resp.Body.Close()

		// Track response status
		if resp.StatusCode >= 400 {
			g.recordError(errorsCounter, resp.StatusCode)
		}

		// Copy response headers
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}

		// Add gateway headers
		w.Header().Set("X-Gateway-Response-Time", fmt.Sprintf("%.2fms",
			float64(time.Since(startTime).Microseconds())/1000))

		// Write status
		w.WriteHeader(resp.StatusCode)

		// Copy body
		if _, err := io.Copy(w, resp.Body); err != nil {
			// Log but don't error - response already started
			g.recordError(errorsCounter, 0) // Generic error
		}
	}
}

// recordError tracks errors using static counters
func (g *Gateway) recordError(errorsCounter metricz.Counter, statusCode int) {
	// Track total errors
	g.totalErrors.Add(1)

	// Service-level error using the passed counter
	errorsCounter.Inc()

	// Note: In this teaching example, we don't track per-status-code errors
	// to avoid dynamic key creation. Production systems should define
	// specific error counters for critical status codes (500, 503, etc.)
	_ = statusCode // Acknowledge parameter but don't use dynamically
}

// Health represents gateway health status
type Health struct {
	Healthy        bool
	ActiveRequests int
	ErrorRate      float64
	Services       map[string]ServiceHealth
}

// ServiceHealth represents individual service health
type ServiceHealth struct {
	RequestRate float64
	ErrorRate   float64
	AvgLatency  float64
}

// GetHealth returns current gateway health
func (g *Gateway) GetHealth() Health {
	totalReqs := g.totalRequests.Load()
	totalErrs := g.totalErrors.Load()

	errorRate := float64(0)
	if totalReqs > 0 {
		errorRate = float64(totalErrs) / float64(totalReqs) * 100
	}

	health := Health{
		Healthy:        errorRate < 5.0, // Less than 5% errors
		ActiveRequests: int(g.activeRequests.Value()),
		ErrorRate:      errorRate,
		Services:       make(map[string]ServiceHealth),
	}

	// Calculate per-service health using static metrics
	services := []struct {
		name     string
		requests metricz.Counter
		errors   metricz.Counter
		latency  metricz.Timer
	}{
		{"auth", g.authRequests, g.authErrors, g.authLatency},
		{"user", g.userRequests, g.userErrors, g.userLatency},
		{"order", g.orderRequests, g.orderErrors, g.orderLatency},
		{"payment", g.paymentRequests, g.paymentErrors, g.paymentLatency},
	}

	for _, svc := range services {
		serviceHealth := ServiceHealth{
			RequestRate: svc.requests.Value(),
		}

		// Error rate
		if svc.requests.Value() > 0 {
			serviceHealth.ErrorRate = svc.errors.Value() / svc.requests.Value() * 100
		}

		// Average latency
		if svc.latency.Count() > 0 {
			serviceHealth.AvgLatency = svc.latency.Sum() / float64(svc.latency.Count())
		}

		health.Services[svc.name] = serviceHealth
	}

	return health
}
