package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	gotesting "testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/metricz/testing"
)

// Common types for export tests.
type TimerStats struct {
	Count   uint64  `json:"count"`
	Sum     float64 `json:"sum_ms"`
	Average float64 `json:"avg_ms"`
}

type ServiceMetrics struct {
	Counters map[string]float64 `json:"counters"`
	Gauges   map[string]float64 `json:"gauges"`
}

// Test metric keys are defined in keys.go

// TestPrometheusExport demonstrates Prometheus-style metric export.
func TestPrometheusExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Populate metrics
	registry.Counter(HTTPRequestsTotalKey).Add(1234)
	registry.Counter(HTTPErrorsTotalKey).Add(56)
	registry.Gauge(HTTPConnectionsActiveKey).Set(42)
	registry.Histogram(HTTPRequestDurationKey, metricz.DefaultLatencyBuckets)

	// Add histogram observations
	hist := registry.Histogram(HTTPRequestDurationKey, metricz.DefaultLatencyBuckets)
	observations := []float64{5, 10, 15, 25, 50, 75, 100, 250, 500, 1000, 2500}
	for _, obs := range observations {
		hist.Observe(obs)
	}

	// Export in Prometheus format
	var buf bytes.Buffer

	// Export counters
	for name, counter := range registry.GetCounters() {
		fmt.Fprintf(&buf, "# TYPE %s counter\n", string(name))
		fmt.Fprintf(&buf, "%s %.0f\n", string(name), counter.Value())
	}

	// Export gauges
	for name, gauge := range registry.GetGauges() {
		fmt.Fprintf(&buf, "# TYPE %s gauge\n", string(name))
		fmt.Fprintf(&buf, "%s %.2f\n", string(name), gauge.Value())
	}

	// Export histograms
	for name, histogram := range registry.GetHistograms() {
		fmt.Fprintf(&buf, "# TYPE %s histogram\n", string(name))

		bounds, counts := histogram.Buckets()
		cumulative := uint64(0)

		for i, bound := range bounds {
			cumulative += counts[i]
			fmt.Fprintf(&buf, "%s_bucket{le=\"%.0f\"} %d\n", string(name), bound, cumulative)
		}

		// Add +Inf bucket
		fmt.Fprintf(&buf, "%s_bucket{le=\"+Inf\"} %d\n", string(name), histogram.Count())
		fmt.Fprintf(&buf, "%s_sum %.2f\n", string(name), histogram.Sum())
		fmt.Fprintf(&buf, "%s_count %d\n", string(name), histogram.Count())
	}

	output := buf.String()

	// Verify Prometheus format
	if !strings.Contains(output, "# TYPE http_requests_total counter") {
		t.Error("Missing counter type declaration")
	}

	if !strings.Contains(output, "http_requests_total 1234") {
		t.Error("Counter value not exported correctly")
	}

	if !strings.Contains(output, "# TYPE http_connections_active gauge") {
		t.Error("Missing gauge type declaration")
	}

	if !strings.Contains(output, "http_request_duration_ms_bucket") {
		t.Error("Histogram buckets not exported")
	}

	t.Logf("Prometheus export:\n%s", output)
}

// TestJSONExport demonstrates JSON metric export.
func TestJSONExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Setup metrics
	registry.Counter(metricz.Key("api.requests")).Add(100)
	registry.Counter(metricz.Key("api.errors")).Add(5)
	registry.Gauge(metricz.Key("api.latency.p99")).Set(123.45)
	registry.Gauge(metricz.Key("api.concurrent")).Set(10)

	timer := registry.Timer(metricz.Key("api.duration"))
	for i := 0; i < 50; i++ {
		timer.Record(time.Duration(i*10) * time.Millisecond)
	}

	// Create JSON export structure
	type MetricExport struct { //nolint:govet // fieldalignment - JSON struct field order more important than memory layout
		Timestamp string                 `json:"timestamp"`
		Counters  map[string]float64     `json:"counters"`
		Gauges    map[string]float64     `json:"gauges"`
		Timers    map[string]TimerStats  `json:"timers"`
		Metadata  map[string]interface{} `json:"metadata"`
	}

	export := MetricExport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Counters:  make(map[string]float64),
		Gauges:    make(map[string]float64),
		Timers:    make(map[string]TimerStats),
		Metadata: map[string]interface{}{
			"service": "api",
			"version": "1.0.0",
			"host":    "test-host",
		},
	}

	// Populate counters
	for name, counter := range registry.GetCounters() {
		export.Counters[string(name)] = counter.Value()
	}

	// Populate gauges
	for name, gauge := range registry.GetGauges() {
		export.Gauges[string(name)] = gauge.Value()
	}

	// Populate timers
	for name, timer := range registry.GetTimers() {
		stats := TimerStats{
			Count: timer.Count(),
			Sum:   timer.Sum(),
		}
		if stats.Count > 0 {
			stats.Average = stats.Sum / float64(stats.Count)
		}
		export.Timers[string(name)] = stats
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal metrics: %v", err)
	}

	// Verify JSON structure
	var imported MetricExport
	if err := json.Unmarshal(jsonData, &imported); err != nil {
		t.Fatalf("Failed to unmarshal metrics: %v", err)
	}

	if imported.Counters["api.requests"] != 100 {
		t.Errorf("Counter not exported correctly: got %f", imported.Counters["api.requests"])
	}

	if imported.Gauges["api.latency.p99"] != 123.45 {
		t.Errorf("Gauge not exported correctly: got %f", imported.Gauges["api.latency.p99"])
	}

	if imported.Timers["api.duration"].Count != 50 {
		t.Errorf("Timer count not exported correctly: got %d", imported.Timers["api.duration"].Count)
	}

	t.Logf("JSON export:\n%s", string(jsonData))
}

// TestStreamingExport demonstrates streaming metric updates.
func TestStreamingExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Channel for metric updates
	updates := make(chan string, 100)

	// Function to format metric update
	formatUpdate := func(metricType, name string, value float64) string {
		return fmt.Sprintf("%s|%s|%s|%.2f",
			time.Now().Format("15:04:05.000"),
			metricType,
			name,
			value)
	}

	// Simulate streaming updates
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 10; i++ {
			<-ticker.C

			// Update metrics
			registry.Counter(metricz.Key("stream.events")).Inc()
			registry.Gauge(metricz.Key("stream.rate")).Set(float64(i * 10))

			// Stream updates
			updates <- formatUpdate("counter", "stream.events",
				registry.Counter(metricz.Key("stream.events")).Value())
			updates <- formatUpdate("gauge", "stream.rate",
				registry.Gauge(metricz.Key("stream.rate")).Value())
		}
		close(updates)
	}()

	// Collect streamed updates
	var streamedLines []string //nolint:prealloc // Unknown number of updates at compile time
	for update := range updates {
		streamedLines = append(streamedLines, update)
	}

	// Verify streaming format
	if len(streamedLines) < 10 {
		t.Error("Not enough streamed updates")
	}

	for _, line := range streamedLines {
		parts := strings.Split(line, "|")
		if len(parts) != 4 {
			t.Errorf("Invalid stream format: %s", line)
		}
	}

	t.Logf("Streamed %d metric updates", len(streamedLines))
}

// Custom exporter interface for testing.
type Exporter interface {
	Export(registry *metricz.Registry) ([]byte, error)
}

// StatsD format exporter.
type StatsDExporter struct {
	prefix string
}

func (s *StatsDExporter) Export(registry *metricz.Registry) ([]byte, error) {
	var buf bytes.Buffer

	// Export counters as StatsD counters
	for name, counter := range registry.GetCounters() {
		// Use integer format for counters
		fmt.Fprintf(&buf, "%s%s:%d|c\n", s.prefix, string(name), int64(counter.Value()))
	}

	// Export gauges as StatsD gauges
	for name, gauge := range registry.GetGauges() {
		// Check if value is a whole number
		val := gauge.Value()
		if val == float64(int64(val)) {
			fmt.Fprintf(&buf, "%s%s:%d|g\n", s.prefix, string(name), int64(val))
		} else {
			fmt.Fprintf(&buf, "%s%s:%.1f|g\n", s.prefix, string(name), val)
		}
	}

	// Export timers as StatsD timers
	for name, timer := range registry.GetTimers() {
		if timer.Count() > 0 {
			avg := timer.Sum() / float64(timer.Count())
			// Use integer format for milliseconds
			fmt.Fprintf(&buf, "%s%s:%d|ms\n", s.prefix, string(name), int64(avg))
		}
	}

	// Export histograms as StatsD histograms
	for name, hist := range registry.GetHistograms() {
		if hist.Count() > 0 {
			avg := hist.Sum() / float64(hist.Count())
			fmt.Fprintf(&buf, "%s%s:%d|h\n", s.prefix, string(name), int64(avg))
		}
	}

	return buf.Bytes(), nil
}

// TestCustomExporter demonstrates implementing a custom exporter.
func TestCustomExporter(t *gotesting.T) {

	// Use the custom exporter
	registry := testing.NewTestRegistry(t)
	registry.Counter(RequestsKey).Add(42)
	registry.Gauge(TemperatureKey).Set(23.5)
	registry.Timer(LatencyKey).Record(100 * time.Millisecond)

	exporter := &StatsDExporter{prefix: "myapp."}
	data, err := exporter.Export(registry)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	output := string(data)

	// Verify StatsD format
	if !strings.Contains(output, "myapp.requests:42|c") {
		t.Error("Counter not in StatsD format")
	}

	if !strings.Contains(output, "myapp.temperature:23.5|g") {
		t.Error("Gauge not in StatsD format")
	}

	if !strings.Contains(output, "|ms") {
		t.Error("Timer not in StatsD format")
	}

	t.Logf("StatsD export:\n%s", output)
}

// TestDeltaExport demonstrates exporting metric deltas.
func TestDeltaExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Snapshot for delta calculation
	type Snapshot struct {
		counters map[string]float64
		taken    time.Time
	}

	takeSnapshot := func(reg *metricz.Registry) Snapshot {
		snap := Snapshot{
			counters: make(map[string]float64),
			taken:    time.Now(),
		}

		for name, counter := range reg.GetCounters() {
			snap.counters[string(name)] = counter.Value()
		}

		return snap
	}

	// Initial snapshot
	registry.Counter(metricz.Key("processed")).Add(100)
	registry.Counter(metricz.Key("errors")).Add(5)
	snap1 := takeSnapshot(registry)

	// Some time passes, more metrics
	time.Sleep(50 * time.Millisecond)
	registry.Counter(metricz.Key("processed")).Add(50)
	registry.Counter(metricz.Key("errors")).Add(2)
	snap2 := takeSnapshot(registry)

	// Calculate deltas
	duration := snap2.taken.Sub(snap1.taken)
	deltas := make(map[string]float64)
	rates := make(map[string]float64)

	for name, value2 := range snap2.counters {
		if value1, exists := snap1.counters[name]; exists {
			delta := value2 - value1
			deltas[name] = delta
			rates[name] = delta / duration.Seconds()
		}
	}

	// Verify deltas
	if deltas["processed"] != 50 {
		t.Errorf("Expected delta of 50 for processed, got %f", deltas["processed"])
	}

	if deltas["errors"] != 2 {
		t.Errorf("Expected delta of 2 for errors, got %f", deltas["errors"])
	}

	t.Logf("Delta export - Duration: %v", duration)
	t.Logf("Deltas: %+v", deltas)
	t.Logf("Rates (per second): %+v", rates)
}

// TestBatchExport demonstrates batching metrics for export.
func TestBatchExport(t *gotesting.T) {
	// Multiple registries (simulating multiple services)
	registries := map[string]*metricz.Registry{
		"api":   testing.NewTestRegistry(t),
		"auth":  testing.NewTestRegistry(t),
		"cache": testing.NewTestRegistry(t),
	}

	// Populate metrics - use static keys
	registries["api"].Counter(RequestsKey).Add(1000)
	registries["api"].Gauge(LatencyKey).Set(45.2)

	registries["auth"].Counter(ExternalAuthCallsKey).Add(50)
	registries["auth"].Counter(ExternalAuthErrorsKey).Add(3)

	registries["cache"].Counter(ExternalCacheCallsKey).Add(800)
	registries["cache"].Counter(ExternalCacheErrorsKey).Add(200)
	registries["cache"].Gauge(MemoryTestGaugeKey).Set(128.5)

	// Batch export structure
	type BatchExport struct { //nolint:govet // fieldalignment - JSON struct field order more important than memory layout
		Timestamp string                    `json:"timestamp"`
		Services  map[string]ServiceMetrics `json:"services"`
	}

	// Create batch export
	batch := BatchExport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Services:  make(map[string]ServiceMetrics),
	}

	for service, registry := range registries {
		metrics := ServiceMetrics{
			Counters: make(map[string]float64),
			Gauges:   make(map[string]float64),
		}

		for name, counter := range registry.GetCounters() {
			metrics.Counters[string(name)] = counter.Value()
		}

		for name, gauge := range registry.GetGauges() {
			metrics.Gauges[string(name)] = gauge.Value()
		}

		batch.Services[service] = metrics
	}

	// Export to JSON
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		t.Fatalf("Failed to create batch export: %v", err)
	}

	// Verify batch structure
	var imported BatchExport
	if err := json.Unmarshal(data, &imported); err != nil {
		t.Fatalf("Failed to parse batch export: %v", err)
	}

	// Check all services are present
	if len(imported.Services) != 3 {
		t.Errorf("Expected 3 services in batch, got %d", len(imported.Services))
	}

	// Verify specific metrics
	if imported.Services["api"].Counters["requests"] != 1000 {
		t.Error("API requests not exported correctly")
	}

	if imported.Services["cache"].Gauges["memory_test_gauge"] != 128.5 {
		t.Error("Cache size not exported correctly")
	}

	t.Logf("Batch export size: %d bytes", len(data))
}

// TestFilteredExport demonstrates selective metric export.
func TestFilteredExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Populate various metrics
	registry.Counter(metricz.Key("public.api.requests")).Add(100)
	registry.Counter(metricz.Key("internal.debug.calls")).Add(50)
	registry.Counter(metricz.Key("private.admin.actions")).Add(10)

	registry.Gauge(metricz.Key("public.api.latency")).Set(45.0)
	registry.Gauge(metricz.Key("internal.memory.usage")).Set(1024.0)
	registry.Gauge(metricz.Key("private.secret.value")).Set(42.0)

	// Filter function for public metrics only
	isPublic := func(name string) bool {
		return strings.HasPrefix(name, "public.")
	}

	// Export only public metrics
	publicMetrics := make(map[string]float64)

	for name, counter := range registry.GetCounters() {
		if isPublic(string(name)) {
			publicMetrics[string(name)] = counter.Value()
		}
	}

	for name, gauge := range registry.GetGauges() {
		if isPublic(string(name)) {
			publicMetrics[string(name)] = gauge.Value()
		}
	}

	// Verify filtering
	if len(publicMetrics) != 2 {
		t.Errorf("Expected 2 public metrics, got %d", len(publicMetrics))
	}

	if _, exists := publicMetrics["public.api.requests"]; !exists {
		t.Error("Public API requests not exported")
	}

	if _, exists := publicMetrics["internal.debug.calls"]; exists {
		t.Error("Internal metrics should not be exported")
	}

	if _, exists := publicMetrics["private.admin.actions"]; exists {
		t.Error("Private metrics should not be exported")
	}

	t.Logf("Exported %d public metrics", len(publicMetrics))
}

// TestSortedExport demonstrates exporting metrics in sorted order.
func TestSortedExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Add metrics in random order
	metrics := []string{"zebra", "alpha", "charlie", "bravo", "delta"}
	for i, name := range metrics {
		registry.Counter(metricz.Key(name)).Add(float64(i))
	}

	// Export in sorted order
	counters := registry.GetCounters()
	gauges := registry.GetGauges()
	output := make([]string, 0, len(counters)+len(gauges))

	// Get all counter names and sort
	names := make([]string, 0, len(counters))
	for name := range counters {
		names = append(names, string(name))
	}
	sort.Strings(names)

	// Export in sorted order
	for _, name := range names {
		counter := registry.Counter(metricz.Key(name))
		output = append(output, fmt.Sprintf("%s: %.0f", name, counter.Value()))
	}

	// Verify order
	if len(output) != 5 {
		t.Errorf("Expected 5 metrics, got %d", len(output))
	}

	expectedOrder := []string{"alpha", "bravo", "charlie", "delta", "zebra"}
	for i, name := range expectedOrder {
		if !strings.HasPrefix(output[i], name) {
			t.Errorf("Position %d: expected %s, got %s", i, name, output[i])
		}
	}

	t.Logf("Sorted export:\n%s", strings.Join(output, "\n"))
}

// TestCompressedExport demonstrates compressing metric exports.
func TestCompressedExport(t *gotesting.T) {
	registry := testing.NewTestRegistry(t)

	// Generate many metric operations with static keys
	for i := 0; i < 100; i++ {
		registry.Counter(MemoryTestCounterKey).Add(float64(i))
		registry.Gauge(MemoryTestGaugeKey).Set(float64(i * 2))
	}

	// Create verbose export
	verbose := make(map[string]interface{})

	// Convert Key type to string for JSON marshaling
	counterMap := make(map[string]float64)
	for k, v := range registry.GetCounters() {
		counterMap[string(k)] = v.Value()
	}
	verbose["counters"] = counterMap

	gaugeMap := make(map[string]float64)
	for k, v := range registry.GetGauges() {
		gaugeMap[string(k)] = v.Value()
	}
	verbose["gauges"] = gaugeMap

	verboseJSON, err := json.Marshal(verbose)
	if err != nil {
		t.Fatalf("Failed to marshal verbose export: %v", err)
	}

	// Create compressed format (values only, with index mapping)
	type CompressedExport struct {
		Keys   []string  `json:"k"`
		Values []float64 `json:"v"`
	}

	compressed := CompressedExport{
		Keys:   make([]string, 0),
		Values: make([]float64, 0),
	}

	// Add counters
	for name, counter := range registry.GetCounters() {
		compressed.Keys = append(compressed.Keys, "c:"+string(name))
		compressed.Values = append(compressed.Values, counter.Value())
	}

	// Add gauges
	for name, gauge := range registry.GetGauges() {
		compressed.Keys = append(compressed.Keys, "g:"+string(name))
		compressed.Values = append(compressed.Values, gauge.Value())
	}

	compressedJSON, err := json.Marshal(compressed)
	if err != nil {
		t.Fatalf("Failed to marshal compressed export: %v", err)
	}

	// Compare sizes
	verboseSize := len(verboseJSON)
	compressedSize := len(compressedJSON)
	savings := float64(verboseSize-compressedSize) / float64(verboseSize) * 100

	t.Logf("Verbose export: %d bytes", verboseSize)
	t.Logf("Compressed export: %d bytes", compressedSize)
	t.Logf("Space saved: %.1f%%", savings)

	// Verify we can reconstruct from compressed format
	var reconstructed CompressedExport
	if err := json.Unmarshal(compressedJSON, &reconstructed); err != nil {
		t.Fatalf("Failed to unmarshal compressed export: %v", err)
	}

	if len(reconstructed.Keys) != 2 { // 1 counter + 1 gauge
		t.Errorf("Expected 2 metrics in compressed export, got %d", len(reconstructed.Keys))
	}
}
