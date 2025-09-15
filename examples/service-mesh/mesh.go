package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/metricz"
)

// Metric keys for service mesh - use Key type for all metrics
const (
	// Mesh-level metrics
	RetriesTotal   metricz.Key = "retries_total"
	RetriesSuccess metricz.Key = "retries_success"

	// Service-specific metric prefixes - will be combined with service names
	RequestsPrefix = "requests_"
	ErrorsPrefix   = "errors_"
	LatencyPrefix  = "latency_"
	InstancePrefix = "instance_"
)

// ServiceRequest represents an inter-service request
type ServiceRequest struct {
	From    string
	To      string
	Method  string
	Path    string
	TraceID string
	Timeout time.Duration
}

// ServiceResponse represents a service response
type ServiceResponse struct {
	Success  bool
	Data     interface{}
	Error    error
	Latency  time.Duration
	Instance string
}

// ServiceInstance represents a service endpoint
type ServiceInstance interface {
	Call(req ServiceRequest) ServiceResponse
	Health() bool
}

// ServiceMesh manages service routing and metrics
type ServiceMesh struct {
	registry *metricz.Registry

	// Service registry
	services map[string][]*ServiceEndpoint
	mu       sync.RWMutex

	// Components
	loadBalancer   *LoadBalancer
	circuitBreaker map[string]*CircuitBreaker

	// Metrics
	requestCounters  map[string]metricz.Counter
	errorCounters    map[string]metricz.Counter
	latencyTimers    map[string]metricz.Timer
	instanceCounters map[string]metricz.Counter
	retriesTotal     metricz.Counter
	retriesSuccess   metricz.Counter
}

// ServiceEndpoint wraps a service instance with metadata
type ServiceEndpoint struct {
	Name     string
	Instance ServiceInstance
	ID       string
	Healthy  bool
}

// NewServiceMesh creates a new service mesh
func NewServiceMesh(registry *metricz.Registry) *ServiceMesh {
	return &ServiceMesh{
		registry:         registry,
		services:         make(map[string][]*ServiceEndpoint),
		loadBalancer:     NewLoadBalancer(registry),
		circuitBreaker:   make(map[string]*CircuitBreaker),
		requestCounters:  make(map[string]metricz.Counter),
		errorCounters:    make(map[string]metricz.Counter),
		latencyTimers:    make(map[string]metricz.Timer),
		instanceCounters: make(map[string]metricz.Counter),
		retriesTotal:     registry.Counter(RetriesTotal),
		retriesSuccess:   registry.Counter(RetriesSuccess),
	}
}

// RegisterService adds a service instance to the mesh
func (m *ServiceMesh) RegisterService(serviceName, instanceID string, instance ServiceInstance) {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoint := &ServiceEndpoint{
		Name:     serviceName,
		Instance: instance,
		ID:       instanceID,
		Healthy:  true,
	}

	m.services[serviceName] = append(m.services[serviceName], endpoint)

	// Create circuit breaker for this service if not exists
	if _, exists := m.circuitBreaker[serviceName]; !exists {
		m.circuitBreaker[serviceName] = NewCircuitBreaker(
			serviceName,
			m.registry,
			CircuitBreakerConfig{
				Threshold:   5,
				Timeout:     10 * time.Second,
				HalfOpenMax: 3,
			},
		)
	}

	// Initialize metrics for this service
	if _, exists := m.requestCounters[serviceName]; !exists {
		m.requestCounters[serviceName] = m.registry.Counter(metricz.Key(fmt.Sprintf("%s%s", RequestsPrefix, serviceName)))
		m.errorCounters[serviceName] = m.registry.Counter(metricz.Key(fmt.Sprintf("%s%s", ErrorsPrefix, serviceName)))
		m.latencyTimers[serviceName] = m.registry.Timer(metricz.Key(fmt.Sprintf("%s%s", LatencyPrefix, serviceName)))
	}
}

// Route handles service-to-service routing
func (m *ServiceMesh) Route(req ServiceRequest) ServiceResponse {
	// Track request
	m.getRequestCounter(req.To).Inc()

	// Check circuit breaker
	cb := m.getCircuitBreaker(req.To)
	if !cb.Allow() {
		m.getErrorCounter(req.To).Inc()
		return ServiceResponse{
			Success: false,
			Error:   fmt.Errorf("circuit breaker open for %s", req.To),
			Latency: 0,
		}
	}

	// Get healthy endpoints
	endpoints := m.getHealthyEndpoints(req.To)
	if len(endpoints) == 0 {
		m.getErrorCounter(req.To).Inc()
		cb.RecordFailure()
		return ServiceResponse{
			Success: false,
			Error:   fmt.Errorf("no healthy instances for %s", req.To),
			Latency: 0,
		}
	}

	// Load balance
	endpoint := m.loadBalancer.Select(req.To, endpoints)
	if endpoint == nil {
		m.getErrorCounter(req.To).Inc()
		cb.RecordFailure()
		return ServiceResponse{
			Success: false,
			Error:   fmt.Errorf("load balancer failed to select instance for %s", req.To),
			Latency: 0,
		}
	}

	// Track instance selection
	m.getInstanceCounter(req.To, endpoint.ID).Inc()

	// Set default timeout
	if req.Timeout == 0 {
		req.Timeout = 5 * time.Second
	}

	// Call with retry
	resp := m.callWithRetry(endpoint, req, cb)

	// Record latency
	m.getLatencyTimer(req.To).Record(resp.Latency)

	// Record circuit breaker result
	if resp.Success {
		cb.RecordSuccess()
	} else {
		cb.RecordFailure()
		m.getErrorCounter(req.To).Inc()
	}

	return resp
}

// callWithRetry attempts to call a service with retry logic
func (m *ServiceMesh) callWithRetry(endpoint *ServiceEndpoint, req ServiceRequest, cb *CircuitBreaker) ServiceResponse {
	maxRetries := 3
	backoff := 100 * time.Millisecond

	var lastResp ServiceResponse

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			m.retriesTotal.Inc()
			time.Sleep(backoff)
			backoff *= 2

			// Check if circuit breaker still allows
			if !cb.Allow() {
				break
			}
		}

		// Make the call
		start := time.Now()
		resp := endpoint.Instance.Call(req)
		resp.Latency = time.Since(start)
		resp.Instance = endpoint.ID

		if resp.Success {
			if attempt > 0 {
				m.retriesSuccess.Inc()
			}
			return resp
		}

		lastResp = resp

		// Don't retry on client errors (4xx equivalent)
		if isClientError(resp.Error) {
			break
		}
	}

	return lastResp
}

// getHealthyEndpoints returns healthy service endpoints
func (m *ServiceMesh) getHealthyEndpoints(serviceName string) []*ServiceEndpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoints, exists := m.services[serviceName]
	if !exists {
		return nil
	}

	healthy := make([]*ServiceEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Instance.Health() {
			healthy = append(healthy, ep)
		}
	}

	return healthy
}

// Metric getters
func (m *ServiceMesh) getRequestCounter(service string) metricz.Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if counter, exists := m.requestCounters[service]; exists {
		return counter
	}
	counter := m.registry.Counter(metricz.Key(fmt.Sprintf("%s%s", RequestsPrefix, service)))
	m.requestCounters[service] = counter
	return counter
}

func (m *ServiceMesh) getErrorCounter(service string) metricz.Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if counter, exists := m.errorCounters[service]; exists {
		return counter
	}
	counter := m.registry.Counter(metricz.Key(fmt.Sprintf("%s%s", ErrorsPrefix, service)))
	m.errorCounters[service] = counter
	return counter
}

func (m *ServiceMesh) getLatencyTimer(service string) metricz.Timer {
	m.mu.Lock()
	defer m.mu.Unlock()

	if timer, exists := m.latencyTimers[service]; exists {
		return timer
	}
	timer := m.registry.Timer(metricz.Key(fmt.Sprintf("%s%s", LatencyPrefix, service)))
	m.latencyTimers[service] = timer
	return timer
}

func (m *ServiceMesh) getInstanceCounter(service, instance string) metricz.Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s%s_%s", InstancePrefix, service, instance)
	if counter, exists := m.instanceCounters[key]; exists {
		return counter
	}
	counter := m.registry.Counter(metricz.Key(key))
	m.instanceCounters[key] = counter
	return counter
}

func (m *ServiceMesh) getCircuitBreaker(service string) *CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cb, exists := m.circuitBreaker[service]; exists {
		return cb
	}

	// Create default circuit breaker
	cb := NewCircuitBreaker(
		service,
		m.registry,
		CircuitBreakerConfig{
			Threshold:   5,
			Timeout:     10 * time.Second,
			HalfOpenMax: 3,
		},
	)
	m.circuitBreaker[service] = cb
	return cb
}

// Start begins mesh operations
func (m *ServiceMesh) Start() {
	// Start health checking
	go m.healthCheckLoop()
}

// Stop halts mesh operations
func (m *ServiceMesh) Stop() {
	// Cleanup
}

// healthCheckLoop periodically checks service health
func (m *ServiceMesh) healthCheckLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		for _, endpoints := range m.services {
			for _, ep := range endpoints {
				ep.Healthy = ep.Instance.Health()
			}
		}
		m.mu.Unlock()
	}
}

// isClientError determines if error is client-side (non-retryable)
func isClientError(err error) bool {
	if err == nil {
		return false
	}
	// Simple heuristic - in production check actual error types
	return false
}
