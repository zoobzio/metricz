package main

import (
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/zoobzio/metricz"
)

// Load balancer metric keys
const (
	LoadBalancerSelectionsKey metricz.Key = "load_balancer_selections"
	LoadBalancerAlgorithmKey  metricz.Key = "load_balancer_algorithm"
)

// LoadBalancer distributes requests across service instances
type LoadBalancer struct {
	registry *metricz.Registry

	// Round-robin state
	roundRobinCounters map[string]*atomic.Uint64
	mu                 sync.RWMutex

	// Metrics
	selectCounter  metricz.Counter
	algorithmGauge metricz.Gauge // 0=random, 1=round-robin, 2=least-connections
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(registry *metricz.Registry) *LoadBalancer {
	return &LoadBalancer{
		registry:           registry,
		roundRobinCounters: make(map[string]*atomic.Uint64),
		selectCounter:      registry.Counter(LoadBalancerSelectionsKey),
		algorithmGauge:     registry.Gauge(LoadBalancerAlgorithmKey),
	}
}

// Select chooses an endpoint using load balancing algorithm
func (lb *LoadBalancer) Select(serviceName string, endpoints []*ServiceEndpoint) *ServiceEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	lb.selectCounter.Inc()

	if len(endpoints) == 1 {
		return endpoints[0]
	}

	// Use round-robin for better distribution
	return lb.roundRobin(serviceName, endpoints)
}

// roundRobin implements round-robin selection
func (lb *LoadBalancer) roundRobin(serviceName string, endpoints []*ServiceEndpoint) *ServiceEndpoint {
	lb.algorithmGauge.Set(1.0) // 1 = round-robin

	lb.mu.Lock()
	counter, exists := lb.roundRobinCounters[serviceName]
	if !exists {
		counter = &atomic.Uint64{}
		lb.roundRobinCounters[serviceName] = counter
	}
	lb.mu.Unlock()

	index := counter.Add(1) % uint64(len(endpoints))
	return endpoints[index]
}

// random implements random selection
func (lb *LoadBalancer) random(endpoints []*ServiceEndpoint) *ServiceEndpoint {
	lb.algorithmGauge.Set(0.0) // 0 = random
	return endpoints[rand.Intn(len(endpoints))]
}

// leastConnections would implement least-connections algorithm
// For this example, we'll keep it simple
func (lb *LoadBalancer) leastConnections(endpoints []*ServiceEndpoint) *ServiceEndpoint {
	lb.algorithmGauge.Set(2.0) // 2 = least-connections
	// Simplified - just use random for now
	return lb.random(endpoints)
}
