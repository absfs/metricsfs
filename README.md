# metricsfs

Filesystem operation metrics for Prometheus and OpenTelemetry

## Overview

`metricsfs` is an AbsFS wrapper that provides comprehensive observability and monitoring for filesystem operations. It collects detailed metrics about filesystem usage, performance, and errors, exposing them through industry-standard observability platforms like Prometheus and OpenTelemetry.

By wrapping any `absfs.FileSystem` implementation, metricsfs automatically instruments all filesystem operations, enabling:

- **Performance monitoring**: Track latencies, throughput, and bottlenecks
- **Usage analytics**: Understand I/O patterns and resource utilization
- **Error tracking**: Identify and diagnose filesystem errors
- **Capacity planning**: Monitor bandwidth and operation rates
- **SLA compliance**: Measure and alert on performance targets

## Metrics to Collect

### Operation Metrics

- **Operation Counts** (Counter)
  - `fs_operations_total{operation, status}` - Total filesystem operations by type and status
  - `fs_file_opens_total{mode}` - File opens by mode (read/write/append)
  - `fs_file_creates_total` - File creation count
  - `fs_dir_operations_total{operation}` - Directory operations (mkdir, readdir, remove)

- **Operation Latencies** (Histogram)
  - `fs_operation_duration_seconds{operation}` - Operation duration distribution
  - `fs_read_duration_seconds` - Read operation latency
  - `fs_write_duration_seconds` - Write operation latency
  - `fs_stat_duration_seconds` - Stat operation latency
  - `fs_open_duration_seconds` - Open operation latency

### Data Transfer Metrics

- **Bandwidth** (Counter + Histogram)
  - `fs_bytes_read_total` - Total bytes read
  - `fs_bytes_written_total` - Total bytes written
  - `fs_read_size_bytes{operation}` - Distribution of read sizes
  - `fs_write_size_bytes{operation}` - Distribution of write sizes

- **Throughput** (Gauge)
  - `fs_read_throughput_bytes_per_second` - Current read throughput
  - `fs_write_throughput_bytes_per_second` - Current write throughput

### Error Metrics

- **Error Counts** (Counter)
  - `fs_errors_total{operation, error_type}` - Errors by operation and type
  - `fs_permission_errors_total{operation}` - Permission denied errors
  - `fs_not_found_errors_total{operation}` - File/directory not found errors
  - `fs_timeout_errors_total{operation}` - Timeout errors

### File Descriptor Metrics

- **File Handle Usage** (Gauge)
  - `fs_open_files` - Currently open files
  - `fs_open_files_max` - Maximum concurrent open files observed

### Path-Level Metrics (Optional, with cardinality limits)

- **Hot Paths** (Counter)
  - `fs_path_access_total{path, operation}` - Access counts for specific paths (top N only)

## Architecture

### Wrapper Pattern

```
Application
    |
    v
MetricsFS (instrumentation layer)
    |
    v
Underlying FileSystem (any absfs.FileSystem)
```

### Components

1. **MetricsFS**: Main wrapper implementing `absfs.FileSystem`
2. **MetricsFile**: Instrumented file handle implementing `absfs.File`
3. **Collector**: Metrics collection and aggregation
4. **Exporter**: Prometheus/OpenTelemetry integration
5. **Config**: Configurable metrics collection options

## Implementation Phases

### Phase 1: Core Infrastructure
- Basic wrapper structure implementing absfs.FileSystem
- Operation counting for all filesystem operations
- Error tracking and categorization
- In-memory metrics storage
- Basic Prometheus exporter

### Phase 2: Performance Metrics
- Operation latency histograms
- Bandwidth counters
- File descriptor tracking
- Throughput gauges
- Configurable histogram buckets

### Phase 3: Advanced Features
- OpenTelemetry integration
- Trace correlation (span context)
- Path-based metrics with cardinality protection
- Custom metric labels
- Sampling and aggregation strategies

### Phase 4: Optimization and Tooling
- Low-overhead metric collection
- Lock-free counters where possible
- Metrics dashboard templates (Grafana)
- Alerting rule examples
- Performance overhead benchmarks
- Production best practices documentation

## API Design

### Basic Usage

```go
package main

import (
    "github.com/absfs/absfs"
    "github.com/absfs/metricsfs"
    "github.com/absfs/osfs"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "net/http"
)

func main() {
    // Create base filesystem
    base := osfs.NewFS()

    // Wrap with metrics
    fs := metricsfs.New(base)

    // Register with Prometheus
    prometheus.MustRegister(fs.Collector())

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(":9090", nil)

    // Use filesystem normally - metrics collected automatically
    f, _ := fs.OpenFile("/data/file.txt", os.O_RDWR, 0644)
    defer f.Close()

    data := make([]byte, 1024)
    f.Read(data)  // Automatically tracked
}
```

### Configuration Options

```go
// Create metrics filesystem with custom configuration
fs := metricsfs.New(base, metricsfs.Config{
    // Namespace for Prometheus metrics
    Namespace: "myapp",

    // Subsystem name
    Subsystem: "storage",

    // Enable/disable specific metric groups
    EnableLatencyMetrics: true,
    EnableBandwidthMetrics: true,
    EnablePathMetrics: false,  // High cardinality - disabled by default

    // Histogram buckets for latency (seconds)
    LatencyBuckets: []float64{0.001, 0.01, 0.1, 1.0, 10.0},

    // Histogram buckets for data size (bytes)
    SizeBuckets: prometheus.ExponentialBuckets(1024, 2, 10),

    // Maximum unique paths to track (cardinality limit)
    MaxTrackedPaths: 100,

    // Sample rate for path metrics (0.0 to 1.0)
    PathSampleRate: 0.01,
})
```

### OpenTelemetry Integration

```go
import (
    "github.com/absfs/metricsfs"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

func main() {
    // Get OpenTelemetry meter provider
    provider := otel.GetMeterProvider()

    // Create metrics filesystem with OpenTelemetry
    fs := metricsfs.NewWithOTel(base, metricsfs.OTelConfig{
        MeterProvider: provider,
        MeterName: "github.com/myapp/storage",

        // Enable trace correlation
        EnableTracing: true,
    })

    // Metrics automatically exported through OTLP
    // Operations correlated with distributed traces
}
```

### Custom Labels

```go
// Add custom labels to all metrics
fs := metricsfs.New(base, metricsfs.Config{
    ConstLabels: prometheus.Labels{
        "environment": "production",
        "datacenter": "us-west-2",
        "service": "storage-api",
    },
})
```

### Metric Callbacks

```go
// Hook into metrics collection for custom logic
fs := metricsfs.New(base, metricsfs.Config{
    OnOperation: func(op metricsfs.Operation) {
        if op.Duration > time.Second {
            log.Printf("Slow operation: %s took %v", op.Name, op.Duration)
        }
    },
    OnError: func(op string, err error) {
        errorTracker.Record(op, err)
    },
})
```

## Usage Examples

### Example 1: HTTP File Server Monitoring

```go
func NewMonitoredFileServer(dir string) http.Handler {
    base := osfs.NewFS()
    fs := metricsfs.New(base, metricsfs.Config{
        Namespace: "fileserver",
        EnableLatencyMetrics: true,
        EnableBandwidthMetrics: true,
    })

    // Register metrics
    prometheus.MustRegister(fs.Collector())

    // Create file server using instrumented filesystem
    return http.FileServer(absfs.HTTPFileSystem(fs, dir))
}
```

### Example 2: S3 Backend Monitoring

```go
// Monitor S3 filesystem performance
s3base := s3fs.New(s3Config)
s3monitored := metricsfs.New(s3base, metricsfs.Config{
    Namespace: "app",
    Subsystem: "s3",
    ConstLabels: prometheus.Labels{
        "bucket": bucketName,
    },
})

// Track S3 operation latencies and bandwidth
```

### Example 3: Cache Hit Rate Tracking

```go
// Layer metrics on cache filesystem
cache := cachefs.New(backend, cacheDir)
monitored := metricsfs.New(cache, metricsfs.Config{
    Namespace: "cache",
    EnablePathMetrics: true,
    MaxTrackedPaths: 1000,
})

// Query cache hit rates from metrics:
// rate(fs_operations_total{operation="read",status="success"}[5m])
```

### Example 4: Multi-Tenant Monitoring

```go
// Create separate metrics per tenant
func NewTenantFS(tenantID string, base absfs.FileSystem) absfs.FileSystem {
    return metricsfs.New(base, metricsfs.Config{
        Namespace: "app",
        Subsystem: "tenant",
        ConstLabels: prometheus.Labels{
            "tenant_id": tenantID,
        },
    })
}
```

## Dashboard Examples

### Grafana Dashboard - Filesystem Overview

```
Panel 1: Operation Rate
Query: rate(fs_operations_total[5m])
Visualization: Graph (by operation type)

Panel 2: Error Rate
Query: rate(fs_errors_total[5m])
Visualization: Graph (by error type)

Panel 3: P95 Latency
Query: histogram_quantile(0.95, rate(fs_operation_duration_seconds_bucket[5m]))
Visualization: Graph (by operation)

Panel 4: Throughput
Query: rate(fs_bytes_read_total[5m]) + rate(fs_bytes_written_total[5m])
Visualization: Graph (stacked by read/write)

Panel 5: Open Files
Query: fs_open_files
Visualization: Gauge (current vs max)

Panel 6: Success Rate
Query: sum(rate(fs_operations_total{status="success"}[5m])) / sum(rate(fs_operations_total[5m]))
Visualization: Stat (percentage)
```

### Sample Queries

```promql
# Slow operations (>100ms) rate
sum(rate(fs_operation_duration_seconds_bucket{le="0.1"}[5m]))
  / sum(rate(fs_operation_duration_seconds_count[5m]))

# Error rate by operation
sum by (operation) (rate(fs_errors_total[5m]))

# Read/Write ratio
sum(rate(fs_bytes_read_total[5m])) / sum(rate(fs_bytes_written_total[5m]))

# Average operation size
rate(fs_bytes_written_total[5m]) / rate(fs_operations_total{operation="write"}[5m])

# File descriptor leak detection
deriv(fs_open_files[10m]) > 0
```

### Alerting Rules

```yaml
groups:
  - name: filesystem_alerts
    rules:
      - alert: HighFilesystemErrorRate
        expr: rate(fs_errors_total[5m]) > 0.01
        for: 5m
        annotations:
          summary: "High filesystem error rate detected"

      - alert: FilesystemHighLatency
        expr: histogram_quantile(0.95, rate(fs_operation_duration_seconds_bucket[5m])) > 1.0
        for: 5m
        annotations:
          summary: "Filesystem operations are slow"

      - alert: OpenFileDescriptorLeak
        expr: deriv(fs_open_files[10m]) > 0.1
        for: 10m
        annotations:
          summary: "Possible file descriptor leak"
```

## Performance Overhead Analysis

### Overhead Targets

- **Metric collection**: <5% CPU overhead
- **Memory**: <10MB for typical workload (1M ops/hour)
- **Latency**: <100us per operation instrumentation

### Optimization Strategies

1. **Lock-Free Counters**
   - Use atomic operations for increment-only metrics
   - Minimize contention on hot paths

2. **Lazy Aggregation**
   - Collect raw events in lock-free buffers
   - Aggregate periodically in background

3. **Sampling**
   - Sample path-level metrics to limit cardinality
   - Configurable sample rates per metric type

4. **Histogram Pre-allocation**
   - Pre-allocate histogram buckets
   - Reuse duration measurement buffers

5. **Optional Metrics**
   - Allow disabling expensive metric groups
   - Fine-grained control over what's collected

### Benchmarking

```go
// Benchmark overhead of metrics collection
func BenchmarkMetricsOverhead(b *testing.B) {
    base := memfs.New()
    instrumented := metricsfs.New(base)

    b.Run("baseline", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            base.Stat("/test")
        }
    })

    b.Run("instrumented", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            instrumented.Stat("/test")
        }
    })
}
```

Expected results:
```
BenchmarkMetricsOverhead/baseline-8          10000000    120 ns/op
BenchmarkMetricsOverhead/instrumented-8       9500000    126 ns/op
Overhead: ~5%
```

## Integration Testing

```go
func TestMetricsCollection(t *testing.T) {
    base := memfs.New()
    fs := metricsfs.New(base)

    // Perform operations
    f, _ := fs.Create("/test.txt")
    f.Write([]byte("hello"))
    f.Close()

    // Verify metrics collected
    metrics := fs.Collector().Collect()

    assert.Contains(t, metrics, "fs_operations_total")
    assert.Equal(t, 1, metrics["fs_file_creates_total"])
    assert.Greater(t, metrics["fs_bytes_written_total"], 0)
}
```

## Production Best Practices

1. **Cardinality Management**
   - Disable path-level metrics in high-traffic environments
   - Use sampling for high-cardinality dimensions
   - Set aggressive limits on unique label values

2. **Resource Limits**
   - Configure histogram bucket counts appropriately
   - Monitor memory usage of metrics collector
   - Set max tracked paths based on available memory

3. **Performance Monitoring**
   - Benchmark metrics overhead in your environment
   - Monitor the monitoring (meta-metrics on collection time)
   - Use profiling to identify hot paths

4. **Alerting**
   - Alert on error rates, not absolute error counts
   - Set SLO-based alerts (P95 latency, success rate)
   - Include runbooks in alert annotations

5. **Dashboard Design**
   - Start with high-level overview dashboard
   - Drill-down dashboards for specific subsystems
   - Use consistent time ranges and aggregations

## Related Projects

- [absfs](https://github.com/absfs/absfs) - Core filesystem abstraction
- [cachefs](https://github.com/absfs/cachefs) - Filesystem caching layer
- [lockfs](https://github.com/absfs/lockfs) - Filesystem locking and synchronization
- [retryfs](https://github.com/absfs/retryfs) - Automatic retry for transient failures

## License

MIT License - see LICENSE file for details
