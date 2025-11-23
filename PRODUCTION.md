# Production Best Practices

This document outlines best practices for deploying and operating metricsfs in production environments.

## Table of Contents

1. [Configuration](#configuration)
2. [Performance Considerations](#performance-considerations)
3. [Cardinality Management](#cardinality-management)
4. [Monitoring the Monitor](#monitoring-the-monitor)
5. [Resource Limits](#resource-limits)
6. [Security](#security)
7. [Troubleshooting](#troubleshooting)

## Configuration

### Recommended Production Configuration

```go
config := metricsfs.Config{
    Namespace:              "myapp",
    Subsystem:              "storage",

    // Enable core metrics
    EnableLatencyMetrics:   true,
    EnableBandwidthMetrics: true,

    // Disable high-cardinality metrics by default
    EnablePathMetrics:      false,

    // Set appropriate histogram buckets for your use case
    LatencyBuckets: []float64{
        0.001,  // 1ms
        0.005,  // 5ms
        0.01,   // 10ms
        0.05,   // 50ms
        0.1,    // 100ms
        0.5,    // 500ms
        1.0,    // 1s
        5.0,    // 5s
    },

    // Size buckets for your typical file sizes
    SizeBuckets: prometheus.ExponentialBuckets(1024, 2, 15), // 1KB to 16MB

    // Add environment labels
    ConstLabels: prometheus.Labels{
        "environment": os.Getenv("ENVIRONMENT"),
        "datacenter":  os.Getenv("DATACENTER"),
        "instance":    hostname,
    },
}
```

### Environment-Specific Configurations

#### Development
```go
config := metricsfs.DefaultConfig()
config.EnablePathMetrics = true  // OK for dev
config.MaxTrackedPaths = 1000
```

#### Staging
```go
config := metricsfs.DefaultConfig()
config.EnablePathMetrics = false
// Use production-like configuration
```

#### Production
```go
config := metricsfs.DefaultConfig()
config.EnablePathMetrics = false  // Disable high-cardinality metrics
// Tune buckets based on actual workload patterns
```

## Performance Considerations

### Expected Overhead

Based on benchmarks, metricsfs adds approximately:
- **CPU**: < 5% overhead on typical workloads
- **Memory**: ~10MB for 1M operations/hour
- **Latency**: < 100µs per operation

### Optimizing for Performance

1. **Disable Unused Metrics**
   ```go
   config.EnableLatencyMetrics = false    // If you don't need latency histograms
   config.EnableBandwidthMetrics = false  // If you don't need bandwidth metrics
   ```

2. **Reduce Histogram Buckets**
   ```go
   // Fewer buckets = less memory and CPU
   config.LatencyBuckets = []float64{0.01, 0.1, 1.0, 10.0}
   ```

3. **Avoid Callbacks in Hot Paths**
   ```go
   // Callbacks add overhead - use sparingly
   config.OnOperation = nil
   config.OnError = nil
   ```

4. **Use Sampling for High-Traffic Paths**
   ```go
   if config.EnablePathMetrics {
       config.PathSampleRate = 0.01  // Sample 1% of operations
   }
   ```

### Benchmarking Your Workload

Always benchmark with your specific workload:

```bash
go test -bench=. -benchmem -benchtime=10s
```

Compare baseline vs instrumented performance:
```bash
go test -bench=BenchmarkMetricsOverhead -benchtime=30s
```

## Cardinality Management

### Understanding Cardinality

Cardinality is the number of unique label combinations in your metrics. High cardinality can cause:
- Increased memory usage in Prometheus
- Slower query performance
- Higher storage costs

### Cardinality Limits

**Safe Cardinality Levels:**
- **Low**: < 100 unique series per metric
- **Medium**: 100-1,000 unique series
- **High**: 1,000-10,000 unique series
- **Very High**: > 10,000 unique series (⚠️ use with caution)

### Managing Path Metrics Cardinality

```go
// Option 1: Disable path metrics entirely (recommended for production)
config.EnablePathMetrics = false

// Option 2: Use strict limits
config.EnablePathMetrics = true
config.MaxTrackedPaths = 100      // Only track top 100 hottest paths
config.PathSampleRate = 0.001     // Sample 0.1% of operations

// Option 3: Use path normalization (implement custom logic)
config.OnOperation = func(op Operation) {
    // Normalize paths to reduce cardinality
    // e.g., /users/123/profile -> /users/:id/profile
    normalizedPath := normalizePath(op.Path)
    // Record to separate system
}
```

### Monitoring Cardinality

Query Prometheus to check cardinality:

```promql
# Count unique label combinations
count(count by(__name__, operation, status) (fs_operations_total))

# Find high-cardinality metrics
topk(10, count by(__name__) ({__name__=~"fs_.*"}))
```

## Monitoring the Monitor

### Meta-Metrics

Monitor the metrics collection system itself:

```promql
# Prometheus metrics about metricsfs
prometheus_tsdb_head_series{job="myapp"}
prometheus_tsdb_head_chunks{job="myapp"}
rate(prometheus_tsdb_head_samples_appended_total{job="myapp"}[5m])
```

### Health Checks

Implement health checks:

```go
func healthCheck(fs *metricsfs.MetricsFS) error {
    // Verify metrics are being collected
    collector := fs.Collector()

    // Check that metrics are reasonable
    openFiles := collector.openFiles.Load()
    if openFiles < 0 {
        return fmt.Errorf("invalid open files count: %d", openFiles)
    }

    return nil
}
```

### Alerting on Metrics Collection

```yaml
- alert: MetricsCollectionFailed
  expr: absent(fs_operations_total) == 1
  for: 5m
  annotations:
    summary: "Filesystem metrics are not being collected"
```

## Resource Limits

### Memory Limits

**Factors affecting memory usage:**
1. Number of histogram buckets
2. Number of unique label combinations (cardinality)
3. Number of tracked paths

**Example memory calculation:**
```
Base overhead: ~2MB
+ (histogram_buckets × operations × 8 bytes)
+ (tracked_paths × 100 bytes)
+ (label_combinations × 50 bytes)
```

**Recommendations:**
- Set `GOMEMLIMIT` for the application
- Monitor with `go_memstats_*` metrics
- Use `runtime.ReadMemStats()` for programmatic checks

### CPU Limits

**CPU usage factors:**
1. Operation frequency
2. Enabled metric types
3. Number of histogram buckets
4. Callback complexity

**Recommendations:**
- Profile with `pprof` to identify bottlenecks
- Use CPU quotas in container environments
- Monitor with `process_cpu_seconds_total`

### File Descriptor Limits

```go
// Check file descriptor limits
var rLimit syscall.Rlimit
syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
log.Printf("File descriptor limit: %d", rLimit.Cur)

// Alert when approaching limit
if float64(openFiles) > float64(rLimit.Cur)*0.8 {
    log.Warn("Approaching file descriptor limit")
}
```

## Security

### Sensitive Path Information

Avoid exposing sensitive information in metrics:

```go
// BAD: Exposes user IDs
config.EnablePathMetrics = true

// GOOD: Disable or normalize paths
config.EnablePathMetrics = false

// ACCEPTABLE: Normalize paths before recording
config.OnOperation = func(op Operation) {
    safePath := sanitizePath(op.Path)
    // Record with safe path
}

func sanitizePath(path string) string {
    // Remove sensitive information
    // /users/12345/documents -> /users/:id/documents
    return pathRegex.ReplaceAllString(path, "/:id/")
}
```

### Metrics Endpoint Security

Protect the `/metrics` endpoint:

```go
// Add authentication
http.Handle("/metrics", authMiddleware(promhttp.Handler()))

// Or bind to internal interface only
http.ListenAndServe("127.0.0.1:9090", nil)

// Use network policies in Kubernetes
// Allow only prometheus scraper to access :9090
```

### Error Message Sanitization

```go
config.OnError = func(operation string, err error) {
    // Don't log full paths or sensitive error details
    sanitizedErr := sanitizeError(err)
    log.Error(sanitizedErr)
}
```

## Troubleshooting

### Common Issues

#### 1. High Memory Usage

**Symptoms:**
- Prometheus OOM kills
- Increasing memory usage over time

**Solutions:**
```go
// Reduce histogram buckets
config.LatencyBuckets = []float64{0.01, 0.1, 1.0}

// Disable path metrics
config.EnablePathMetrics = false

// Reduce label cardinality
config.ConstLabels = nil  // Remove unnecessary labels
```

#### 2. Slow Queries

**Symptoms:**
- Grafana dashboards timing out
- Prometheus query timeouts

**Solutions:**
```promql
# Use recording rules for common queries
- record: fs:operations:rate5m
  expr: rate(fs_operations_total[5m])

# Use recording rules in dashboard queries
{__name__="fs:operations:rate5m"}
```

#### 3. Missing Metrics

**Symptoms:**
- Metrics not appearing in Prometheus
- Gaps in dashboards

**Diagnostics:**
```bash
# Check metrics endpoint
curl http://localhost:9090/metrics | grep fs_

# Check Prometheus targets
# Navigate to Prometheus UI > Status > Targets

# Check for errors in logs
grep -i "error\|warn" app.log | grep metric
```

**Solutions:**
```go
// Ensure collector is registered
prometheus.MustRegister(fs.Collector())

// Check for registration conflicts
registry := prometheus.NewRegistry()
registry.MustRegister(fs.Collector())
```

#### 4. File Descriptor Leaks

**Symptoms:**
- `fs_open_files` continuously increasing
- "too many open files" errors

**Diagnostics:**
```bash
# Check open files
lsof -p <pid> | wc -l

# Monitor with metrics
fs_open_files
deriv(fs_open_files[10m])
```

**Solutions:**
```go
// Add defer to ensure cleanup
f, err := fs.Open(path)
if err != nil {
    return err
}
defer f.Close()  // Always close!

// Monitor for leaks
if deriv(fs_open_files[10m]) > 0.1 {
    // Alert: possible file descriptor leak
}
```

### Debug Mode

Enable debug logging:

```go
config.OnOperation = func(op Operation) {
    if op.Duration > time.Second {
        log.Printf("Slow operation: %s took %v", op.Name, op.Duration)
    }
}

config.OnError = func(operation string, err error) {
    log.Printf("Error in %s: %v", operation, err)
}
```

### Performance Profiling

Use Go's pprof for profiling:

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Analyze profiles:
```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Look for metricsfs in the output
```

## Checklist for Production Deployment

- [ ] Disable path metrics (`EnablePathMetrics = false`)
- [ ] Set appropriate histogram buckets for your workload
- [ ] Configure resource limits (memory, CPU, file descriptors)
- [ ] Add environment-specific labels
- [ ] Protect metrics endpoint with authentication
- [ ] Set up alerting rules
- [ ] Import Grafana dashboards
- [ ] Configure Prometheus scraping
- [ ] Set up recording rules for common queries
- [ ] Test failover and recovery scenarios
- [ ] Document runbooks for alerts
- [ ] Benchmark with production-like workload
- [ ] Monitor cardinality in Prometheus
- [ ] Set up log aggregation for errors
- [ ] Plan for metrics retention and storage

## Support and Resources

- **Issues**: https://github.com/absfs/metricsfs/issues
- **Discussions**: https://github.com/absfs/metricsfs/discussions
- **Examples**: See `examples/` directory
- **Benchmarks**: Run `go test -bench=.`
