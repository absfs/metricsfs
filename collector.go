package metricsfs

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector collects and exposes filesystem metrics.
type Collector struct {
	config Config

	// Operation counters
	operationsTotal    *prometheus.CounterVec
	fileOpensTotal     *prometheus.CounterVec
	fileCreatesTotal   prometheus.Counter
	dirOperationsTotal *prometheus.CounterVec

	// Latency histograms
	operationDuration *prometheus.HistogramVec
	readDuration      prometheus.Histogram
	writeDuration     prometheus.Histogram
	statDuration      prometheus.Histogram
	openDuration      prometheus.Histogram

	// Bandwidth counters
	bytesReadTotal    prometheus.Counter
	bytesWrittenTotal prometheus.Counter
	readSizeBytes     *prometheus.HistogramVec
	writeSizeBytes    *prometheus.HistogramVec

	// Note: Throughput (bytes/second) is calculated via Prometheus queries:
	// - Read throughput: rate(fs_bytes_read_total[5m])
	// - Write throughput: rate(fs_bytes_written_total[5m])
	// This approach is more efficient and aligns with Prometheus best practices.

	// Error counters
	errorsTotal           *prometheus.CounterVec
	permissionErrorsTotal *prometheus.CounterVec
	notFoundErrorsTotal   *prometheus.CounterVec
	timeoutErrorsTotal    *prometheus.CounterVec

	// File descriptor tracking
	openFiles         atomic.Int64
	openFilesMax      atomic.Int64
	openFilesGauge    prometheus.Gauge
	openFilesMaxGauge prometheus.Gauge

	// Path metrics (if enabled)
	pathAccessTotal *prometheus.CounterVec
	pathMutex       sync.RWMutex
	trackedPaths    map[string]bool
}

// NewCollector creates a new metrics collector with the given configuration.
func NewCollector(config Config) *Collector {
	config.applyDefaults()

	c := &Collector{
		config:       config,
		trackedPaths: make(map[string]bool),
	}

	// Initialize operation counters
	c.operationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "operations_total",
			Help:        "Total filesystem operations by type and status",
			ConstLabels: config.ConstLabels,
		},
		[]string{"operation", "status"},
	)

	c.fileOpensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "file_opens_total",
			Help:        "File opens by mode",
			ConstLabels: config.ConstLabels,
		},
		[]string{"mode"},
	)

	c.fileCreatesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "file_creates_total",
			Help:        "File creation count",
			ConstLabels: config.ConstLabels,
		},
	)

	c.dirOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "dir_operations_total",
			Help:        "Directory operations",
			ConstLabels: config.ConstLabels,
		},
		[]string{"operation"},
	)

	// Initialize latency histograms
	if config.EnableLatencyMetrics {
		c.operationDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "operation_duration_seconds",
				Help:        "Operation duration distribution",
				Buckets:     config.LatencyBuckets,
				ConstLabels: config.ConstLabels,
			},
			[]string{"operation"},
		)

		c.readDuration = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "read_duration_seconds",
				Help:        "Read operation latency",
				Buckets:     config.LatencyBuckets,
				ConstLabels: config.ConstLabels,
			},
		)

		c.writeDuration = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "write_duration_seconds",
				Help:        "Write operation latency",
				Buckets:     config.LatencyBuckets,
				ConstLabels: config.ConstLabels,
			},
		)

		c.statDuration = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "stat_duration_seconds",
				Help:        "Stat operation latency",
				Buckets:     config.LatencyBuckets,
				ConstLabels: config.ConstLabels,
			},
		)

		c.openDuration = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "open_duration_seconds",
				Help:        "Open operation latency",
				Buckets:     config.LatencyBuckets,
				ConstLabels: config.ConstLabels,
			},
		)
	}

	// Initialize bandwidth counters
	if config.EnableBandwidthMetrics {
		c.bytesReadTotal = prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "bytes_read_total",
				Help:        "Total bytes read",
				ConstLabels: config.ConstLabels,
			},
		)

		c.bytesWrittenTotal = prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "bytes_written_total",
				Help:        "Total bytes written",
				ConstLabels: config.ConstLabels,
			},
		)

		c.readSizeBytes = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "read_size_bytes",
				Help:        "Distribution of read sizes",
				Buckets:     config.SizeBuckets,
				ConstLabels: config.ConstLabels,
			},
			[]string{"operation"},
		)

		c.writeSizeBytes = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "write_size_bytes",
				Help:        "Distribution of write sizes",
				Buckets:     config.SizeBuckets,
				ConstLabels: config.ConstLabels,
			},
			[]string{"operation"},
		)
	}

	// Initialize error counters
	c.errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "errors_total",
			Help:        "Errors by operation and type",
			ConstLabels: config.ConstLabels,
		},
		[]string{"operation", "error_type"},
	)

	c.permissionErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "permission_errors_total",
			Help:        "Permission denied errors",
			ConstLabels: config.ConstLabels,
		},
		[]string{"operation"},
	)

	c.notFoundErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "not_found_errors_total",
			Help:        "File/directory not found errors",
			ConstLabels: config.ConstLabels,
		},
		[]string{"operation"},
	)

	c.timeoutErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "timeout_errors_total",
			Help:        "Timeout errors",
			ConstLabels: config.ConstLabels,
		},
		[]string{"operation"},
	)

	// Initialize file descriptor gauges
	c.openFilesGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "open_files",
			Help:        "Currently open files",
			ConstLabels: config.ConstLabels,
		},
	)

	c.openFilesMaxGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   config.Namespace,
			Subsystem:   config.Subsystem,
			Name:        "open_files_max",
			Help:        "Maximum concurrent open files observed",
			ConstLabels: config.ConstLabels,
		},
	)

	// Initialize path metrics (if enabled)
	if config.EnablePathMetrics {
		c.pathAccessTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   config.Namespace,
				Subsystem:   config.Subsystem,
				Name:        "path_access_total",
				Help:        "Access counts for specific paths",
				ConstLabels: config.ConstLabels,
			},
			[]string{"path", "operation"},
		)
	}

	return c
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	c.operationsTotal.Describe(ch)
	c.fileOpensTotal.Describe(ch)
	c.fileCreatesTotal.Describe(ch)
	c.dirOperationsTotal.Describe(ch)

	if c.config.EnableLatencyMetrics {
		c.operationDuration.Describe(ch)
		c.readDuration.Describe(ch)
		c.writeDuration.Describe(ch)
		c.statDuration.Describe(ch)
		c.openDuration.Describe(ch)
	}

	if c.config.EnableBandwidthMetrics {
		c.bytesReadTotal.Describe(ch)
		c.bytesWrittenTotal.Describe(ch)
		c.readSizeBytes.Describe(ch)
		c.writeSizeBytes.Describe(ch)
	}

	c.errorsTotal.Describe(ch)
	c.permissionErrorsTotal.Describe(ch)
	c.notFoundErrorsTotal.Describe(ch)
	c.timeoutErrorsTotal.Describe(ch)

	c.openFilesGauge.Describe(ch)
	c.openFilesMaxGauge.Describe(ch)

	if c.config.EnablePathMetrics {
		c.pathAccessTotal.Describe(ch)
	}
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	// Update gauges before collecting
	c.openFilesGauge.Set(float64(c.openFiles.Load()))
	c.openFilesMaxGauge.Set(float64(c.openFilesMax.Load()))

	c.operationsTotal.Collect(ch)
	c.fileOpensTotal.Collect(ch)
	c.fileCreatesTotal.Collect(ch)
	c.dirOperationsTotal.Collect(ch)

	if c.config.EnableLatencyMetrics {
		c.operationDuration.Collect(ch)
		c.readDuration.Collect(ch)
		c.writeDuration.Collect(ch)
		c.statDuration.Collect(ch)
		c.openDuration.Collect(ch)
	}

	if c.config.EnableBandwidthMetrics {
		c.bytesReadTotal.Collect(ch)
		c.bytesWrittenTotal.Collect(ch)
		c.readSizeBytes.Collect(ch)
		c.writeSizeBytes.Collect(ch)
	}

	c.errorsTotal.Collect(ch)
	c.permissionErrorsTotal.Collect(ch)
	c.notFoundErrorsTotal.Collect(ch)
	c.timeoutErrorsTotal.Collect(ch)

	c.openFilesGauge.Collect(ch)
	c.openFilesMaxGauge.Collect(ch)

	if c.config.EnablePathMetrics {
		c.pathAccessTotal.Collect(ch)
	}
}

// recordOperation records metrics for a filesystem operation.
func (c *Collector) recordOperation(op, path string, duration time.Duration, bytesTransferred int64, err error) {
	// Determine status
	status := "success"
	if err != nil {
		status = "error"
		c.recordError(op, err)
	}

	// Record operation count
	c.operationsTotal.WithLabelValues(op, status).Inc()

	// Record latency if enabled
	if c.config.EnableLatencyMetrics {
		c.operationDuration.WithLabelValues(op).Observe(duration.Seconds())

		// Also record in specific operation histograms
		switch op {
		case "read":
			c.readDuration.Observe(duration.Seconds())
		case "write":
			c.writeDuration.Observe(duration.Seconds())
		case "stat":
			c.statDuration.Observe(duration.Seconds())
		case "open":
			c.openDuration.Observe(duration.Seconds())
		}
	}

	// Record bandwidth if enabled
	if c.config.EnableBandwidthMetrics && bytesTransferred > 0 {
		switch op {
		case "read":
			c.bytesReadTotal.Add(float64(bytesTransferred))
			c.readSizeBytes.WithLabelValues(op).Observe(float64(bytesTransferred))
		case "write":
			c.bytesWrittenTotal.Add(float64(bytesTransferred))
			c.writeSizeBytes.WithLabelValues(op).Observe(float64(bytesTransferred))
		}
	}

	// Record path metrics if enabled
	if c.config.EnablePathMetrics && path != "" {
		c.recordPathAccess(path, op)
	}

	// Call user callback if provided
	if c.config.OnOperation != nil {
		c.config.OnOperation(Operation{
			Name:             op,
			Duration:         duration,
			BytesTransferred: bytesTransferred,
			Path:             path,
			Error:            err,
		})
	}
}

// recordError records error metrics.
func (c *Collector) recordError(op string, err error) {
	if err == nil {
		return
	}

	// Categorize error
	errorType := "unknown"
	if errors.Is(err, os.ErrNotExist) {
		errorType = "not_found"
		c.notFoundErrorsTotal.WithLabelValues(op).Inc()
	} else if errors.Is(err, os.ErrPermission) {
		errorType = "permission"
		c.permissionErrorsTotal.WithLabelValues(op).Inc()
	} else if errors.Is(err, os.ErrDeadlineExceeded) {
		errorType = "timeout"
		c.timeoutErrorsTotal.WithLabelValues(op).Inc()
	}

	c.errorsTotal.WithLabelValues(op, errorType).Inc()

	// Call user callback if provided
	if c.config.OnError != nil {
		c.config.OnError(op, err)
	}
}

// recordPathAccess records path-level metrics with cardinality protection.
func (c *Collector) recordPathAccess(path, op string) {
	c.pathMutex.RLock()
	tracked := c.trackedPaths[path]
	count := len(c.trackedPaths)
	c.pathMutex.RUnlock()

	// If already tracked or under limit, record it
	if tracked || count < c.config.MaxTrackedPaths {
		if !tracked {
			c.pathMutex.Lock()
			c.trackedPaths[path] = true
			c.pathMutex.Unlock()
		}
		c.pathAccessTotal.WithLabelValues(path, op).Inc()
	}
}

// trackFileOpen increments the open file counter.
func (c *Collector) trackFileOpen() {
	current := c.openFiles.Add(1)

	// Update max if needed
	for {
		max := c.openFilesMax.Load()
		if current <= max {
			break
		}
		if c.openFilesMax.CompareAndSwap(max, current) {
			break
		}
	}
}

// trackFileClose decrements the open file counter.
func (c *Collector) trackFileClose() {
	c.openFiles.Add(-1)
}

// recordFileOpen records a file open operation.
func (c *Collector) recordFileOpen(mode string) {
	c.fileOpensTotal.WithLabelValues(mode).Inc()
}

// recordFileCreate records a file creation.
func (c *Collector) recordFileCreate() {
	c.fileCreatesTotal.Inc()
}

// recordDirOperation records a directory operation.
func (c *Collector) recordDirOperation(op string) {
	c.dirOperationsTotal.WithLabelValues(op).Inc()
}
