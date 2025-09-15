# Worker Pool Metrics Example

## The Zombie Worker Apocalypse

**Tuesday, 3:47 AM. Pager goes off.**

"Service degraded. Jobs backing up."

You check the dashboard. 10 workers running. Queue depth normal. Success rate 94%. Everything looks fine.

But customers are screaming. 30+ second delays. Orders timing out.

SSH into production. Run `ps aux | grep worker`.
**47 processes.** Should be 10.
Memory usage: **8.7GB.** Should be 400MB.

Your workers have become zombies. Still running, consuming resources, but dead inside. Processing nothing. And spawning more zombies every minute.

This example shows you how to detect and prevent the zombie apocalypse.

## The Discovery Journey

### Phase 1: Basic Counting

```go
type WorkerPool struct {
    jobCount    int64
    errorCount  int64
    busyWorkers int32
}
```

Problems:
- No visibility into job types
- No processing time tracking
- Race conditions with multiple readers
- No queue depth monitoring

### Phase 2: Channel Monitoring

```go
type WorkerPool struct {
    jobs     chan Job
    results  chan Result
    metrics  chan MetricUpdate
}

// Separate goroutine collecting metrics
go pool.metricsCollector()
```

New problems:
- Metrics channel becomes bottleneck
- Memory grows with metric history
- Complex synchronization
- Testing requires coordinating multiple goroutines

### Phase 3: Discovery of metricz

metricz solves the measurement problem:
- **Atomic gauges** for queue depth and worker counts
- **Histograms** for job duration distributions
- **Per-job-type metrics** using key patterns
- **Registry isolation** for testing

## The Solution

This example demonstrates a production worker pool with comprehensive observability:

1. **Queue metrics**: depth, arrival rate, drain rate
2. **Worker metrics**: utilization, idle time, throughput
3. **Job metrics**: duration by type, success/failure rates
4. **System metrics**: memory usage, goroutine count

## Running the Scenarios

### Scenario 1: Normal Operations
```bash
go run . normal

# Watch healthy metrics:
# - Workers: 10 (stable)
# - Memory: ~400MB (stable)
# - Success rate: 94%+
# - No zombies
```

### Scenario 2: The Zombie Apocalypse
```bash
go run . zombie

# Watch the system degrade:
# - 00:30 - First toxic job arrives
# - 01:00 - First zombie appears
# - 02:00 - Memory climbing (1.2GB)
# - 03:00 - 15 zombies, 3 active workers
# - 05:00 - System unresponsive
```

The zombie scenario simulates what happened in production: workers that hang on toxic jobs, never releasing resources, spawning replacements that also become zombies.

## What You'll Learn

1. **Zombie Detection**: How to identify workers that are dead but still consuming resources
2. **Hung Job Tracking**: Detecting jobs that run forever
3. **Memory Leaks**: How goroutines can leak memory even with proper channels
4. **Lifecycle Management**: Why workers need health checks and timeouts
5. **Toxic Payloads**: How one bad job can cascade into system failure

## Implementation Phases

### Phase 1: Basic Pool
- Simple job queue
- Fixed worker count
- Basic success/error counting

### Phase 2: Dynamic Scaling
- Auto-scaling workers based on queue depth
- Idle worker detection
- Resource limit enforcement

### Phase 3: Advanced Monitoring
- Job retry metrics
- Circuit breaker per job type
- Predictive scaling based on arrival patterns
- Dead letter queue metrics

## The Critical Metrics That Saved Production

### Traditional Metrics (These Lied to Us)
- ✅ Worker count: 10 (looked correct, but 37 were zombies)
- ✅ Queue depth: 23 (seemed fine, but jobs were stuck)
- ✅ Success rate: 94% (high enough, but customers suffering)

### Life-Saving Metrics (The Truth)
- 💀 **Zombie workers**: Dead goroutines still consuming memory
- 🔴 **Hung jobs**: Jobs processing > 30 seconds
- ⚠️ **Worker restarts**: Excessive churn indicating problems
- 📈 **Memory per worker**: Should be constant, not growing

```go
// The metrics that revealed the apocalypse
zombieWorkers := registry.Gauge("zombie_workers")
hungJobs := registry.Gauge("hung_jobs")
workerRestarts := registry.Counter("worker_restarts")

// Alert when zombies appear
if zombieWorkers.Value() > 0 {
    panic("ZOMBIES DETECTED")
}
```

## Failure Patterns Demonstrated

1. **Worker Starvation**: All workers busy, queue growing
2. **Poison Jobs**: Jobs that always fail, affecting success rate
3. **Slow Jobs**: Jobs that block workers for extended periods
4. **Memory Pressure**: Queue growth under sustained load
5. **Thundering Herd**: Burst arrivals overwhelming the pool

## Files in This Example

- `main.go` - Worker pool with normal vs zombie scenarios
- `pool.go` - Worker pool with zombie detection and lifecycle management
- `pool_test.go` - Tests demonstrating failure modes
- `SCENARIO.md` - Full production incident timeline and analysis