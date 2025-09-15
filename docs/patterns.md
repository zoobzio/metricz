# Common Patterns

## HTTP Server Middleware

### Basic Middleware
```go
func metricsMiddleware(registry *metricz.Registry) func(http.Handler) http.Handler {
    requests := registry.Counter("http_requests_total")
    duration := registry.Timer("http_request_duration", []float64{
        0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
    })
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            requests.Inc()
            stop := duration.Start()
            next.ServeHTTP(w, r)
            stop()
        })
    }
}

// Usage
func main() {
    registry := metricz.New()
    
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", usersHandler)
    mux.HandleFunc("/api/orders", ordersHandler)
    
    // Wrap with metrics
    handler := metricsMiddleware(registry)(mux)
    
    // Metrics endpoint
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        registry.Export(w)
    })
    
    http.ListenAndServe(":8080", handler)
}
```

### Per-Route Metrics
```go
type routeMetrics struct {
    requests *metricz.Counter
    duration *metricz.Timer
    errors   *metricz.Counter
}

func (rm *routeMetrics) middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rm.requests.Inc()
        stop := rm.duration.Start()
        
        wrapped := &statusWriter{ResponseWriter: w, status: 200}
        next.ServeHTTP(wrapped, r)
        
        if wrapped.status >= 400 {
            rm.errors.Inc()
        }
        
        stop()
    })
}

type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}

// Setup per route
func setupRoutes(registry *metricz.Registry) {
    users := &routeMetrics{
        requests: registry.Counter("users_requests_total"),
        duration: registry.Timer("users_duration", defaultBuckets),
        errors:   registry.Counter("users_errors_total"),
    }
    
    orders := &routeMetrics{
        requests: registry.Counter("orders_requests_total"),
        duration: registry.Timer("orders_duration", defaultBuckets), 
        errors:   registry.Counter("orders_errors_total"),
    }
    
    http.Handle("/api/users", users.middleware(http.HandlerFunc(usersHandler)))
    http.Handle("/api/orders", orders.middleware(http.HandlerFunc(ordersHandler)))
}
```

## Background Workers

### Job Processing
```go
func worker(registry *metricz.Registry, jobs <-chan Job) {
    processed := registry.Counter("jobs_processed_total")
    failed := registry.Counter("jobs_failed_total")
    duration := registry.Timer("job_duration", []float64{0.1, 0.5, 1.0, 5.0, 10.0})
    queueSize := registry.Gauge("job_queue_size")
    
    for job := range jobs {
        queueSize.Set(float64(len(jobs)))
        
        stop := duration.Start()
        
        if err := processJob(job); err != nil {
            failed.Inc()
            log.Printf("Job failed: %v", err)
        } else {
            processed.Inc()
        }
        
        stop()
    }
}

func processJob(job Job) error {
    // Your job processing logic
    time.Sleep(100 * time.Millisecond) // Simulate work
    return nil
}
```

### Pool Monitoring
```go
type WorkerPool struct {
    registry    *metricz.Registry
    activeJobs  *metricz.Gauge
    totalJobs   *metricz.Counter
    failedJobs  *metricz.Counter
    jobDuration *metricz.Timer
}

func NewWorkerPool(registry *metricz.Registry) *WorkerPool {
    return &WorkerPool{
        registry:    registry,
        activeJobs:  registry.Gauge("worker_active_jobs"),
        totalJobs:   registry.Counter("worker_total_jobs"),
        failedJobs:  registry.Counter("worker_failed_jobs"),
        jobDuration: registry.Timer("worker_job_duration", 
            []float64{0.01, 0.1, 0.5, 1.0, 5.0}),
    }
}

func (wp *WorkerPool) ProcessJob(job Job) error {
    wp.totalJobs.Inc()
    wp.activeJobs.Inc()
    defer wp.activeJobs.Dec()
    
    stop := wp.jobDuration.Start()
    defer stop()
    
    if err := job.Process(); err != nil {
        wp.failedJobs.Inc()
        return err
    }
    
    return nil
}
```

## Export Patterns

### Prometheus Integration
```go
func prometheusHandler(registry *metricz.Registry) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        registry.Export(w)
    }
}

// Mount at /metrics
http.HandleFunc("/metrics", prometheusHandler(registry))
```

### JSON API Endpoint  
```go
func metricsAPIHandler(registry *metricz.Registry) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        
        if err := registry.ExportJSON(w); err != nil {
            http.Error(w, "Failed to export metrics", http.StatusInternalServerError)
            return
        }
    }
}

// Mount at /api/metrics
http.HandleFunc("/api/metrics", metricsAPIHandler(registry))
```

### Health Check Integration
```go
func healthHandler(registry *metricz.Registry) http.HandlerFunc {
    healthChecks := registry.Counter("health_checks_total")
    
    return func(w http.ResponseWriter, r *http.Request) {
        healthChecks.Inc()
        
        values := registry.Values()
        
        response := map[string]interface{}{
            "status": "healthy",
            "timestamp": time.Now().Unix(),
            "metrics": values,
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}
```

## Resource Monitoring

### Memory Usage
```go
func monitorMemory(registry *metricz.Registry) {
    memAlloc := registry.Gauge("memory_alloc_bytes")
    memSys := registry.Gauge("memory_sys_bytes")  
    numGC := registry.Counter("gc_runs_total")
    
    ticker := time.NewTicker(10 * time.Second)
    go func() {
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            memAlloc.Set(float64(m.Alloc))
            memSys.Set(float64(m.Sys))
            numGC.Set(float64(m.NumGC))
        }
    }()
}
```

### Connection Pool
```go
type PoolMetrics struct {
    active     *metricz.Gauge
    idle       *metricz.Gauge  
    total      *metricz.Gauge
    waits      *metricz.Counter
    timeouts   *metricz.Counter
    waitTime   *metricz.Timer
}

func NewPoolMetrics(registry *metricz.Registry, name string) *PoolMetrics {
    return &PoolMetrics{
        active:   registry.Gauge(name + "_active_connections"),
        idle:     registry.Gauge(name + "_idle_connections"),
        total:    registry.Gauge(name + "_total_connections"),
        waits:    registry.Counter(name + "_connection_waits_total"),
        timeouts: registry.Counter(name + "_connection_timeouts_total"),
        waitTime: registry.Timer(name + "_connection_wait_duration",
            []float64{0.001, 0.01, 0.1, 0.5, 1.0}),
    }
}

func (pm *PoolMetrics) RecordAcquire(duration time.Duration, timeout bool) {
    pm.waits.Inc()
    pm.waitTime.Observe(duration)
    
    if timeout {
        pm.timeouts.Inc()
    }
}

func (pm *PoolMetrics) UpdateCounts(active, idle, total int) {
    pm.active.Set(float64(active))
    pm.idle.Set(float64(idle))  
    pm.total.Set(float64(total))
}
```

## Custom Bucket Configurations

### HTTP Response Times
```go
// Web application response time buckets (seconds)
httpBuckets := []float64{
    0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
}

latency := registry.Timer("http_request_duration", httpBuckets)
```

### Database Query Times
```go
// Database query duration buckets (seconds)
dbBuckets := []float64{
    0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0,
}

dbTimer := registry.Timer("db_query_duration", dbBuckets)
```

### Request/Response Sizes
```go
// Size buckets (bytes)  
sizeBuckets := []float64{
    100, 1000, 10000, 100000, 1000000, 10000000,
}

requestSize := registry.Histogram("http_request_size_bytes", sizeBuckets) 
responseSize := registry.Histogram("http_response_size_bytes", sizeBuckets)
```

### Queue Depths  
```go
// Queue size buckets
queueBuckets := []float64{
    1, 5, 10, 25, 50, 100, 250, 500, 1000,
}

queueDepth := registry.Histogram("queue_depth", queueBuckets)
```

## Error Patterns

### Graceful Degradation
```go
func resilientHandler(registry *metricz.Registry) http.HandlerFunc {
    requests := registry.Counter("requests_total")
    errors := registry.Counter("request_errors_total") 
    
    return func(w http.ResponseWriter, r *http.Request) {
        requests.Inc()
        
        // Metrics collection shouldn't break request handling
        defer func() {
            if r := recover(); r != nil {
                // Log metric collection panic but continue serving
                log.Printf("Metrics panic: %v", r)
            }
        }()
        
        if err := handleRequest(w, r); err != nil {
            errors.Inc()
            http.Error(w, "Internal Server Error", 500)
        }
    }
}
```

### Circuit Breaker Integration
```go
type CircuitBreakerMetrics struct {
    requests *metricz.Counter
    success  *metricz.Counter  
    failures *metricz.Counter
    timeouts *metricz.Counter
    state    *metricz.Gauge // 0=closed, 1=open, 2=half-open
}

func (cbm *CircuitBreakerMetrics) RecordCall() {
    cbm.requests.Inc()
}

func (cbm *CircuitBreakerMetrics) RecordSuccess() {
    cbm.success.Inc()
}

func (cbm *CircuitBreakerMetrics) RecordFailure() {
    cbm.failures.Inc()
}

func (cbm *CircuitBreakerMetrics) RecordTimeout() {
    cbm.timeouts.Inc()
    cbm.failures.Inc()
}

func (cbm *CircuitBreakerMetrics) SetState(state int) {
    cbm.state.Set(float64(state))
}
```