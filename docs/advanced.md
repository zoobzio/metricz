# Advanced Usage

## Performance Optimization

### Zero-Allocation Patterns

**Avoid string concatenation in hot paths**:
```go
// SLOW: Creates garbage
func badHandler(registry *metricz.Registry) {
    counter := registry.Counter("requests_" + endpoint + "_total")
    counter.Inc()
}

// FAST: Pre-create metrics
var (
    usersCounter  = registry.Counter("requests_users_total")
    ordersCounter = registry.Counter("requests_orders_total")
)

func goodHandler(endpoint string) {
    switch endpoint {
    case "users":
        usersCounter.Inc()
    case "orders": 
        ordersCounter.Inc()
    }
}
```

**Reuse buffers for exports**:
```go
type MetricsExporter struct {
    registry *metricz.Registry
    buffer   *bytes.Buffer
    mutex    sync.Mutex
}

func (e *MetricsExporter) Export(w http.ResponseWriter) error {
    e.mutex.Lock()
    defer e.mutex.Unlock()
    
    e.buffer.Reset() // Reuse buffer, avoid allocation
    if err := e.registry.Export(e.buffer); err != nil {
        return err
    }
    
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    _, err := e.buffer.WriteTo(w)
    return err
}
```

### High-Throughput Scenarios

**Batch operations where possible**:
```go
// Process metrics in batches to reduce contention
type BatchProcessor struct {
    counter *metricz.Counter
    batch   chan float64
    done    chan struct{}
}

func NewBatchProcessor(counter *metricz.Counter) *BatchProcessor {
    bp := &BatchProcessor{
        counter: counter,
        batch:   make(chan float64, 1000),
        done:    make(chan struct{}),
    }
    go bp.processBatches()
    return bp
}

func (bp *BatchProcessor) processBatches() {
    ticker := time.NewTicker(100 * time.Millisecond)
    var sum float64
    
    for {
        select {
        case value := <-bp.batch:
            sum += value
        case <-ticker.C:
            if sum > 0 {
                bp.counter.Add(sum)
                sum = 0
            }
        case <-bp.done:
            return
        }
    }
}

func (bp *BatchProcessor) Record(value float64) {
    select {
    case bp.batch <- value:
    default:
        // Channel full, apply directly
        bp.counter.Add(value)
    }
}
```

## Memory Management

### Bounded Metric Sets

**Implement metric limits**:
```go
type BoundedRegistry struct {
    *metricz.Registry
    maxMetrics int
    metrics    map[string]bool
    mutex      sync.RWMutex
}

func NewBoundedRegistry(maxMetrics int) *BoundedRegistry {
    return &BoundedRegistry{
        Registry:   metricz.New(),
        maxMetrics: maxMetrics,
        metrics:    make(map[string]bool),
    }
}

func (br *BoundedRegistry) Counter(key string) metricz.Counter {
    br.mutex.Lock()
    defer br.mutex.Unlock()
    
    if !br.metrics[key] {
        if len(br.metrics) >= br.maxMetrics {
            log.Printf("Metric limit reached, ignoring: %s", key)
            return &noopCounter{}
        }
        br.metrics[key] = true
    }
    
    return br.Registry.Counter(key)
}

type noopCounter struct{}
func (c *noopCounter) Inc()              {}
func (c *noopCounter) Add(float64)       {}
func (c *noopCounter) Value() float64    { return 0 }
```

### Memory Profiling

**Profile memory usage**:
```go
import (
    _ "net/http/pprof"
    "runtime"
)

func profileMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    log.Printf("Alloc: %d KB", m.Alloc/1024)
    log.Printf("TotalAlloc: %d KB", m.TotalAlloc/1024)  
    log.Printf("Sys: %d KB", m.Sys/1024)
    log.Printf("NumGC: %d", m.NumGC)
}

// Run memory profiler
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

**Track metric memory usage**:
```go
func estimateMetricMemory(registry *metricz.Registry) {
    values := registry.Values()
    
    var totalSize int64
    for name, value := range values {
        // Estimate size: metric name + value storage
        totalSize += int64(len(name)) + 64 // Rough estimate
        
        if hist, ok := value.(map[string]interface{}); ok {
            // Histograms use more memory for buckets
            if buckets, ok := hist["buckets"].(map[string]interface{}); ok {
                totalSize += int64(len(buckets)) * 64
            }
        }
    }
    
    log.Printf("Estimated metric memory usage: %d KB", totalSize/1024)
}
```

## Concurrent Access Patterns

### Lock-Free Counters

**High-contention scenarios**:
```go
import "sync/atomic"

type ShardedCounter struct {
    shards []int64
    mask   int64
}

func NewShardedCounter(shards int) *ShardedCounter {
    // Ensure power of 2 for efficient masking
    if shards&(shards-1) != 0 {
        shards = 1 << uint(64-bits.LeadingZeros64(uint64(shards-1)))
    }
    
    return &ShardedCounter{
        shards: make([]int64, shards),
        mask:   int64(shards - 1),
    }
}

func (sc *ShardedCounter) Inc() {
    shard := runtime.Gosched() & sc.mask // Pseudo-random shard
    atomic.AddInt64(&sc.shards[shard], 1)
}

func (sc *ShardedCounter) Value() int64 {
    var total int64
    for i := range sc.shards {
        total += atomic.LoadInt64(&sc.shards[i])
    }
    return total
}
```

### Reader-Writer Optimization

**Separate read and write operations**:
```go
type OptimizedRegistry struct {
    writeRegistry *metricz.Registry
    readRegistry  *metricz.Registry
    writeBuffer   *bytes.Buffer
    mutex         sync.RWMutex
}

func (or *OptimizedRegistry) Counter(key string) metricz.Counter {
    return or.writeRegistry.Counter(key)
}

func (or *OptimizedRegistry) Export(w io.Writer) error {
    or.mutex.RLock()
    defer or.mutex.RUnlock()
    return or.readRegistry.Export(w)
}

func (or *OptimizedRegistry) snapshot() {
    or.mutex.Lock()
    defer or.mutex.Unlock()
    
    // Copy current metrics to read registry
    values := or.writeRegistry.Values()
    or.readRegistry = metricz.New()
    
    for name, value := range values {
        // Recreate metrics in read registry
        switch v := value.(type) {
        case float64:
            counter := or.readRegistry.Counter(name)
            counter.Add(v)
        }
    }
}
```

## Custom Export Formats

### StatsD Integration

**Export to StatsD format**:
```go
func exportToStatsD(registry *metricz.Registry, addr string) error {
    conn, err := net.Dial("udp", addr)
    if err != nil {
        return err
    }
    defer conn.Close()
    
    values := registry.Values()
    
    for name, value := range values {
        switch v := value.(type) {
        case float64:
            // Counter format: metric_name:value|c
            fmt.Fprintf(conn, "%s:%.0f|c\n", name, v)
        case map[string]interface{}:
            // Histogram format: metric_name:value|h
            if count, ok := v["count"].(uint64); ok {
                fmt.Fprintf(conn, "%s_count:%d|g\n", name, count)
            }
            if sum, ok := v["sum"].(float64); ok {
                fmt.Fprintf(conn, "%s_sum:%.2f|g\n", name, sum)
            }
        }
    }
    
    return nil
}
```

### InfluxDB Line Protocol

**Export to InfluxDB format**:
```go
func exportToInfluxDB(registry *metricz.Registry, w io.Writer) error {
    values := registry.Values()
    timestamp := time.Now().UnixNano()
    
    for name, value := range values {
        switch v := value.(type) {
        case float64:
            fmt.Fprintf(w, "%s value=%.2f %d\n", name, v, timestamp)
        case map[string]interface{}:
            if count, ok := v["count"].(uint64); ok {
                fmt.Fprintf(w, "%s,type=count value=%d %d\n", name, count, timestamp)
            }
            if sum, ok := v["sum"].(float64); ok {
                fmt.Fprintf(w, "%s,type=sum value=%.2f %d\n", name, sum, timestamp)
            }
        }
    }
    
    return nil
}
```

## Debugging and Profiling

### Performance Benchmarking

**Benchmark metric operations**:
```go
func BenchmarkCounterInc(b *testing.B) {
    registry := metricz.New()
    counter := registry.Counter("benchmark_counter")
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            counter.Inc()
        }
    })
}

func BenchmarkExport(b *testing.B) {
    registry := metricz.New()
    
    // Create many metrics
    for i := 0; i < 1000; i++ {
        counter := registry.Counter(fmt.Sprintf("metric_%d", i))
        counter.Add(float64(i))
    }
    
    buf := &bytes.Buffer{}
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        buf.Reset()
        registry.Export(buf)
    }
}
```

### Lock Contention Analysis

**Detect contention hotspots**:
```go
import (
    "runtime"
    "sync"
    "time"
)

func analyzeContention() {
    var wg sync.WaitGroup
    counter := registry.Counter("contention_test")
    
    // Create contention
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 10000; j++ {
                counter.Inc()
            }
        }()
    }
    
    // Monitor goroutine blocking
    go func() {
        ticker := time.NewTicker(100 * time.Millisecond)
        for range ticker.C {
            log.Printf("Goroutines: %d", runtime.NumGoroutine())
        }
    }()
    
    wg.Wait()
}
```

### Memory Leak Detection

**Automated leak detection**:
```go
func detectMemoryLeaks(registry *metricz.Registry) {
    ticker := time.NewTicker(30 * time.Second)
    var baseMemory uint64
    
    go func() {
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            if baseMemory == 0 {
                baseMemory = m.Alloc
                continue
            }
            
            growth := float64(m.Alloc-baseMemory) / float64(baseMemory)
            if growth > 0.5 { // 50% growth
                values := registry.Values()
                log.Printf("Memory growth detected: %.1f%%, metrics: %d", 
                    growth*100, len(values))
                
                // Log largest metrics
                var largest []string
                for name := range values {
                    if len(name) > 100 {
                        largest = append(largest, name)
                    }
                }
                
                if len(largest) > 0 {
                    log.Printf("Large metric names detected: %v", largest)
                }
            }
        }
    }()
}
```

## Production Patterns

### Circuit Breaker for Exports

**Prevent export failures from cascading**:
```go
type CircuitBreakerExporter struct {
    registry     *metricz.Registry
    failures     int64
    lastFailure  time.Time
    threshold    int
    timeout      time.Duration
    mutex        sync.RWMutex
}

func (cbe *CircuitBreakerExporter) Export(w io.Writer) error {
    cbe.mutex.RLock()
    failures := cbe.failures
    lastFailure := cbe.lastFailure
    cbe.mutex.RUnlock()
    
    // Circuit open - reject immediately
    if failures >= int64(cbe.threshold) {
        if time.Since(lastFailure) < cbe.timeout {
            return errors.New("circuit breaker open")
        }
        // Try to close circuit
        cbe.mutex.Lock()
        cbe.failures = 0
        cbe.mutex.Unlock()
    }
    
    err := cbe.registry.Export(w)
    if err != nil {
        cbe.mutex.Lock()
        cbe.failures++
        cbe.lastFailure = time.Now()
        cbe.mutex.Unlock()
    }
    
    return err
}
```

### Graceful Degradation

**Continue serving when metrics fail**:
```go
type RobustMetrics struct {
    primary   *metricz.Registry
    fallback  *metricz.Registry
    healthy   int64 // atomic
}

func (rm *RobustMetrics) Counter(key string) metricz.Counter {
    if atomic.LoadInt64(&rm.healthy) == 1 {
        return &robustCounter{
            primary:  rm.primary.Counter(key),
            fallback: rm.fallback.Counter(key),
            robust:   rm,
        }
    }
    return rm.fallback.Counter(key)
}

type robustCounter struct {
    primary  metricz.Counter
    fallback metricz.Counter
    robust   *RobustMetrics
}

func (rc *robustCounter) Inc() {
    defer func() {
        if r := recover(); r != nil {
            // Primary failed, switch to fallback
            atomic.StoreInt64(&rc.robust.healthy, 0)
            rc.fallback.Inc()
        }
    }()
    
    rc.primary.Inc()
}
```

## Custom Metric Types

### Rate Metric

**Track rates over time windows**:
```go
type RateMetric struct {
    samples []sample
    window  time.Duration
    mutex   sync.RWMutex
}

type sample struct {
    timestamp time.Time
    value     float64
}

func NewRateMetric(window time.Duration) *RateMetric {
    return &RateMetric{
        window: window,
    }
}

func (rm *RateMetric) Record(value float64) {
    rm.mutex.Lock()
    defer rm.mutex.Unlock()
    
    now := time.Now()
    rm.samples = append(rm.samples, sample{now, value})
    
    // Remove old samples
    cutoff := now.Add(-rm.window)
    var keep []sample
    for _, s := range rm.samples {
        if s.timestamp.After(cutoff) {
            keep = append(keep, s)
        }
    }
    rm.samples = keep
}

func (rm *RateMetric) Rate() float64 {
    rm.mutex.RLock()
    defer rm.mutex.RUnlock()
    
    if len(rm.samples) < 2 {
        return 0
    }
    
    var sum float64
    for _, s := range rm.samples {
        sum += s.value
    }
    
    elapsed := rm.samples[len(rm.samples)-1].timestamp.Sub(rm.samples[0].timestamp)
    return sum / elapsed.Seconds()
}
```

### Percentile Metric

**Track percentiles efficiently**:
```go
import "sort"

type PercentileMetric struct {
    values []float64
    mutex  sync.RWMutex
    maxSize int
}

func NewPercentileMetric(maxSize int) *PercentileMetric {
    return &PercentileMetric{
        maxSize: maxSize,
    }
}

func (pm *PercentileMetric) Observe(value float64) {
    pm.mutex.Lock()
    defer pm.mutex.Unlock()
    
    pm.values = append(pm.values, value)
    
    // Keep only recent values
    if len(pm.values) > pm.maxSize {
        copy(pm.values, pm.values[len(pm.values)-pm.maxSize:])
        pm.values = pm.values[:pm.maxSize]
    }
}

func (pm *PercentileMetric) Percentile(p float64) float64 {
    pm.mutex.RLock()
    defer pm.mutex.RUnlock()
    
    if len(pm.values) == 0 {
        return 0
    }
    
    sorted := make([]float64, len(pm.values))
    copy(sorted, pm.values)
    sort.Float64s(sorted)
    
    index := int(p * float64(len(sorted)))
    if index >= len(sorted) {
        index = len(sorted) - 1
    }
    
    return sorted[index]
}
```