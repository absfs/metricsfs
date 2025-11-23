package metricsfs

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Config holds configuration options for the metrics filesystem.
type Config struct {
	// Namespace for Prometheus metrics (default: "fs")
	Namespace string

	// Subsystem name for Prometheus metrics (default: "")
	Subsystem string

	// ConstLabels are labels that will be applied to all metrics
	ConstLabels prometheus.Labels

	// EnableLatencyMetrics controls whether operation latency histograms are collected
	EnableLatencyMetrics bool

	// EnableBandwidthMetrics controls whether bandwidth counters are collected
	EnableBandwidthMetrics bool

	// EnablePathMetrics controls whether path-level metrics are collected
	// WARNING: This can lead to high cardinality - disabled by default
	EnablePathMetrics bool

	// LatencyBuckets defines histogram buckets for operation latency (in seconds)
	// Default: [0.001, 0.01, 0.1, 1.0, 10.0]
	LatencyBuckets []float64

	// SizeBuckets defines histogram buckets for data transfer sizes (in bytes)
	// Default: prometheus.ExponentialBuckets(1024, 2, 10)
	SizeBuckets []float64

	// MaxTrackedPaths is the maximum number of unique paths to track
	// Only used when EnablePathMetrics is true (default: 100)
	MaxTrackedPaths int

	// PathSampleRate controls sampling rate for path metrics (0.0 to 1.0)
	// Only used when EnablePathMetrics is true (default: 0.01)
	PathSampleRate float64

	// OnOperation is called after each filesystem operation
	OnOperation func(op Operation)

	// OnError is called when an operation encounters an error
	OnError func(operation string, err error)
}

// Operation represents a completed filesystem operation with metrics.
type Operation struct {
	// Name of the operation (e.g., "read", "write", "stat")
	Name string

	// Duration of the operation
	Duration time.Duration

	// BytesTransferred is the number of bytes read or written
	BytesTransferred int64

	// Path is the file path involved in the operation
	Path string

	// Error that occurred during the operation, if any
	Error error
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Namespace:              "fs",
		Subsystem:              "",
		ConstLabels:            nil,
		EnableLatencyMetrics:   true,
		EnableBandwidthMetrics: true,
		EnablePathMetrics:      false,
		LatencyBuckets:         []float64{0.001, 0.01, 0.1, 1.0, 10.0},
		SizeBuckets:            prometheus.ExponentialBuckets(1024, 2, 10),
		MaxTrackedPaths:        100,
		PathSampleRate:         0.01,
	}
}

// applyDefaults fills in default values for unset configuration options.
func (c *Config) applyDefaults() {
	if c.Namespace == "" {
		c.Namespace = "fs"
	}
	if c.LatencyBuckets == nil {
		c.LatencyBuckets = []float64{0.001, 0.01, 0.1, 1.0, 10.0}
	}
	if c.SizeBuckets == nil {
		c.SizeBuckets = prometheus.ExponentialBuckets(1024, 2, 10)
	}
	if c.MaxTrackedPaths == 0 {
		c.MaxTrackedPaths = 100
	}
	if c.PathSampleRate == 0 {
		c.PathSampleRate = 0.01
	}
}
