package integration

import "github.com/zoobzio/metricz"

// Shared metric keys for all integration tests - consistent Key type usage.
const (
	// Common service metrics.
	RequestsKey metricz.Key = "requests"
	ErrorsKey   metricz.Key = "errors"
	LatencyKey  metricz.Key = "latency"

	// Test metrics.
	TestCounterKey metricz.Key = "test_counter"
	TestGaugeKey   metricz.Key = "test_gauge"
	NewCounterKey  metricz.Key = "new_counter"
	CounterKey     metricz.Key = "counter"
	GaugeKey       metricz.Key = "gauge"
	HistKey        metricz.Key = "hist"
	TimerKey       metricz.Key = "timer"
	FinalKey       metricz.Key = "final"

	// Race test specific keys.
	TestHistogramKey   metricz.Key = "test_histogram"
	TestTimerKey       metricz.Key = "test_timer"
	HistogramKey       metricz.Key = "histogram"
	SharedHistogramKey metricz.Key = "shared_histogram"
	HighContentionKey  metricz.Key = "high_contention"

	// Shared test keys.
	SharedCounterKey metricz.Key = "shared_counter"
	SharedGaugeKey   metricz.Key = "shared_gauge"
	SharedHistKey    metricz.Key = "shared_hist"
	SharedTimerKey   metricz.Key = "shared_timer"

	// Numbered keys for isolation tests.
	Counter1Key metricz.Key = "counter1"
	Counter2Key metricz.Key = "counter2"
	Gauge1Key   metricz.Key = "gauge1"
	Gauge2Key   metricz.Key = "gauge2"
	Timer1Key   metricz.Key = "timer1"
	Timer2Key   metricz.Key = "timer2"
	Hist1Key    metricz.Key = "hist1"
	Hist2Key    metricz.Key = "hist2"

	// Service pattern keys.
	ServiceOperationsKey metricz.Key = "service.operations"
	ServiceErrorsKey     metricz.Key = "service.errors"
	ServiceLatencyKey    metricz.Key = "service.latency"

	// Helper metrics.
	HelperUsersCreatedKey metricz.Key = "helper.users.created"
	HelperUsersDeletedKey metricz.Key = "helper.users.deleted"
	HelperUserCreationKey metricz.Key = "helper.user.creation"

	// Parallel test metrics.
	ParallelOperationsKey metricz.Key = "parallel.operations"
	ParallelErrorsKey     metricz.Key = "parallel.errors"

	// Additional specific test keys.
	SharedNameKey    metricz.Key = "shared_name"
	OpsKey           metricz.Key = "ops"
	StatusKey        metricz.Key = "status"
	Test1Key         metricz.Key = "test1"
	Test2Key         metricz.Key = "test2"
	Test3Key         metricz.Key = "test3"
	TestKey          metricz.Key = "test"
	InstanceIDKey    metricz.Key = "instance_id"
	ContaminationKey metricz.Key = "contamination_test"
	TemperatureKey   metricz.Key = "temperature"

	// Aggregation test keys.
	TaskCompletedKey   metricz.Key = "task.completed"
	TasksProcessedKey  metricz.Key = "tasks.processed"
	TaskDurationKey    metricz.Key = "task.duration"
	TasksErrorsKey     metricz.Key = "tasks.errors"
	WorkerStatusKey    metricz.Key = "worker.status"
	ResponseTimeKey    metricz.Key = "response.time"
	ResponseTimeP50Key metricz.Key = "response.time.p50"
	ResponseTimeP95Key metricz.Key = "response.time.p95"
	ResponseTimeP99Key metricz.Key = "response.time.p99"
	RequestRateKey     metricz.Key = "request.rate"
	RequestRateMaxKey  metricz.Key = "request.rate.max"

	// Export test keys.
	HTTPRequestsTotalKey     metricz.Key = "http_requests_total"
	HTTPErrorsTotalKey       metricz.Key = "http_errors_total"
	HTTPConnectionsActiveKey metricz.Key = "http_connections_active"
	HTTPRequestDurationKey   metricz.Key = "http_request_duration_ms"

	// Memory pattern test keys - static replacements for dynamic keys.
	MemoryTestCounterKey metricz.Key = "memory_test_counter"
	MemoryTestGaugeKey   metricz.Key = "memory_test_gauge"
	MemoryTestHistKey    metricz.Key = "memory_test_hist"

	// Service lifecycle test keys - static replacements for dynamic service metrics.
	ExternalAuthCallsKey        metricz.Key = "external.auth.calls"
	ExternalAuthLatencyKey      metricz.Key = "external.auth.latency"
	ExternalAuthSuccessKey      metricz.Key = "external.auth.success"
	ExternalAuthErrorsKey       metricz.Key = "external.auth.errors"
	ExternalAuthAvailabilityKey metricz.Key = "external.auth.availability"

	ExternalDatabaseCallsKey        metricz.Key = "external.database.calls"
	ExternalDatabaseLatencyKey      metricz.Key = "external.database.latency"
	ExternalDatabaseSuccessKey      metricz.Key = "external.database.success"
	ExternalDatabaseErrorsKey       metricz.Key = "external.database.errors"
	ExternalDatabaseAvailabilityKey metricz.Key = "external.database.availability"

	ExternalCacheCallsKey        metricz.Key = "external.cache.calls"
	ExternalCacheLatencyKey      metricz.Key = "external.cache.latency"
	ExternalCacheSuccessKey      metricz.Key = "external.cache.success"
	ExternalCacheErrorsKey       metricz.Key = "external.cache.errors"
	ExternalCacheAvailabilityKey metricz.Key = "external.cache.availability"

	ExternalStorageCallsKey        metricz.Key = "external.storage.calls"
	ExternalStorageLatencyKey      metricz.Key = "external.storage.latency"
	ExternalStorageSuccessKey      metricz.Key = "external.storage.success"
	ExternalStorageErrorsKey       metricz.Key = "external.storage.errors"
	ExternalStorageAvailabilityKey metricz.Key = "external.storage.availability"

	// Test pattern keys for isolation testing.
	PatternOperationsKey metricz.Key = "pattern.operations"
	PatternLatencyKey    metricz.Key = "pattern.latency"

	// Mock service keys.
	MockCallsKey   metricz.Key = "mock.calls"
	MockLatencyKey metricz.Key = "mock.latency"
	MockErrorsKey  metricz.Key = "mock.errors"

	// Golden test keys.
	GoldenThroughputKey metricz.Key = "golden.throughput"
	GoldenLatencyKey    metricz.Key = "golden.latency"
	GoldenErrorRateKey  metricz.Key = "golden.error_rate"
	GoldenDiffKey       metricz.Key = "golden.diff"
)
