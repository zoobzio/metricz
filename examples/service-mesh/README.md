# Service Mesh Metrics Example

## The Problem

You're running a microservices architecture. Services call each other. Some calls fail. Some timeout. Circuit breakers trip. You need to track:
- Inter-service latencies
- Circuit breaker states
- Request routing decisions
- Service discovery health
- Distributed tracing correlations

Without proper metrics at the mesh layer, you can't see the actual service communication patterns.

## The Discovery Journey

### Phase 1: Service-Level Metrics

```go
// Each service tracks its own metrics
serviceA.requestCount++
serviceB.errorCount++
```

Problems:
- No view of service interactions
- Can't see call chains
- No circuit breaker visibility
- Missing retry patterns

### Phase 2: Centralized Metrics

```go
// Central metrics service
metricsService.Record("serviceA", "serviceB", latency)
```

New problems:
- Single point of failure
- Network overhead for every metric
- Lost metrics during network issues
- Complex aggregation logic

### Phase 3: Discovery of metricz with Mesh Pattern

The mesh pattern with metricz provides:
- **Per-service registries** with aggregation
- **Circuit breaker metrics** at the mesh layer
- **Request routing metrics** for load balancing
- **Service discovery metrics** for health

## The Solution

This example demonstrates a service mesh with comprehensive observability:

1. **Sidecar proxy pattern** with metrics
2. **Circuit breaker** with state tracking
3. **Load balancing** with algorithm metrics
4. **Service discovery** with health checks
5. **Distributed tracing** context propagation

## Running the Example

```bash
# Run the service mesh demo
go run .

# The demo will:
# 1. Start multiple service instances
# 2. Configure mesh routing
# 3. Simulate various failure scenarios
# 4. Show circuit breaker behavior
# 5. Display mesh-level metrics
```

## What You'll Learn

1. **Sidecar Pattern**: Intercepting and measuring all service calls
2. **Circuit Breaker Metrics**: State transitions and trip reasons
3. **Load Balancer Metrics**: Distribution across instances
4. **Service Discovery**: Health check patterns
5. **Aggregation Patterns**: Combining metrics from multiple services

## Implementation Phases

### Phase 1: Basic Mesh
- Simple proxy between services
- Request/response counting
- Basic latency tracking

### Phase 2: Resilience Patterns
- Circuit breakers with metrics
- Retry logic with backoff
- Timeout tracking

### Phase 3: Advanced Routing
- Load balancing algorithms
- Service discovery integration
- Traffic shaping metrics
- Canary deployment tracking

## Key Insights

1. **Mesh vs Service Metrics**: Services track business logic, mesh tracks infrastructure.

2. **Circuit Breaker States**: Track open, half-open, closed transitions with reasons.

3. **Load Distribution**: Measure actual vs expected distribution across instances.

4. **Discovery Lag**: Track time between service registration and first successful route.

5. **Context Propagation**: Trace IDs enable correlation across service boundaries.

## Failure Patterns Demonstrated

1. **Cascading Failures**: One service failure affecting others
2. **Circuit Breaker Storms**: Multiple breakers opening simultaneously  
3. **Retry Amplification**: Retries causing more load
4. **Discovery Delays**: New instances not receiving traffic
5. **Partial Degradation**: Some endpoints working, others failing
6. **The Cascade That Wasn't the Database**: A complete production war story

### The Cascade That Wasn't the Database

The final scenario demonstrates a real production incident where:

- **14:23 UTC**: Black Friday traffic surge begins
- **14:25 UTC**: Auth service goroutine leak starts (JWT validation timeouts)
- **14:29 UTC**: First auth instance OOMs from 18,000+ goroutines
- **14:31 UTC**: Second instance fails, retry storm creates 51,000 RPS
- **14:33 UTC**: Total cascade, all authentication down
- **14:35-39 UTC**: Teams investigate database (healthy) and network (fine)
- **14:39 UTC**: **Metrics reveal truth**: Goroutine count 18,000 vs normal 200
- **14:42 UTC**: Emergency auth restart, cascade stops

**Key Lesson**: Database appeared fine throughout. Goroutine metrics were the smoking gun that revealed memory exhaustion as root cause. Traditional debugging failed; system metrics saved the day.

## Files in This Example

- `main.go` - Service mesh demonstration
- `mesh.go` - Mesh proxy implementation
- `circuit_breaker.go` - Circuit breaker with metrics
- `load_balancer.go` - Load balancing algorithms
- `service.go` - Mock service implementations
- `mesh_test.go` - Mesh behavior tests