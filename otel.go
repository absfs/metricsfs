package metricsfs

import (
	"context"
	"os"
	"time"

	"github.com/absfs/absfs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OTelConfig holds configuration for OpenTelemetry integration.
type OTelConfig struct {
	// MeterProvider for creating metrics instruments
	MeterProvider metric.MeterProvider

	// TracerProvider for creating traces
	TracerProvider trace.TracerProvider

	// MeterName is the name of the meter (default: "github.com/absfs/metricsfs")
	MeterName string

	// TracerName is the name of the tracer (default: "github.com/absfs/metricsfs")
	TracerName string

	// EnableTracing enables distributed tracing for filesystem operations
	EnableTracing bool

	// ConstAttributes are attributes that will be applied to all metrics and spans
	ConstAttributes []attribute.KeyValue
}

// OTelCollector collects filesystem metrics using OpenTelemetry.
type OTelCollector struct {
	config OTelConfig
	meter  metric.Meter
	tracer trace.Tracer

	// Metric instruments
	operationsCounter   metric.Int64Counter
	bytesReadCounter    metric.Int64Counter
	bytesWrittenCounter metric.Int64Counter
	operationDuration   metric.Float64Histogram
	openFilesGauge      metric.Int64UpDownCounter
	errorsCounter       metric.Int64Counter
}

// NewOTelCollector creates a new OpenTelemetry metrics collector.
func NewOTelCollector(config OTelConfig) (*OTelCollector, error) {
	if config.MeterProvider == nil {
		config.MeterProvider = otel.GetMeterProvider()
	}

	if config.TracerProvider == nil {
		config.TracerProvider = otel.GetTracerProvider()
	}

	if config.MeterName == "" {
		config.MeterName = "github.com/absfs/metricsfs"
	}

	if config.TracerName == "" {
		config.TracerName = "github.com/absfs/metricsfs"
	}

	c := &OTelCollector{
		config: config,
		meter:  config.MeterProvider.Meter(config.MeterName),
		tracer: config.TracerProvider.Tracer(config.TracerName),
	}

	var err error

	// Initialize operation counter
	c.operationsCounter, err = c.meter.Int64Counter(
		"fs.operations",
		metric.WithDescription("Total filesystem operations"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize bytes read counter
	c.bytesReadCounter, err = c.meter.Int64Counter(
		"fs.bytes.read",
		metric.WithDescription("Total bytes read"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize bytes written counter
	c.bytesWrittenCounter, err = c.meter.Int64Counter(
		"fs.bytes.written",
		metric.WithDescription("Total bytes written"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize operation duration histogram
	c.operationDuration, err = c.meter.Float64Histogram(
		"fs.operation.duration",
		metric.WithDescription("Filesystem operation duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize open files gauge
	c.openFilesGauge, err = c.meter.Int64UpDownCounter(
		"fs.open_files",
		metric.WithDescription("Currently open files"),
		metric.WithUnit("{file}"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize errors counter
	c.errorsCounter, err = c.meter.Int64Counter(
		"fs.errors",
		metric.WithDescription("Filesystem errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// recordOperation records metrics for a filesystem operation.
func (c *OTelCollector) recordOperation(ctx context.Context, op, path string, duration time.Duration, bytesTransferred int64, err error) {
	attrs := c.buildAttributes(op, path, err)

	// Record operation count
	c.operationsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record duration
	c.operationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	// Record bytes transferred
	if bytesTransferred > 0 {
		switch op {
		case "read":
			c.bytesReadCounter.Add(ctx, bytesTransferred, metric.WithAttributes(attrs...))
		case "write":
			c.bytesWrittenCounter.Add(ctx, bytesTransferred, metric.WithAttributes(attrs...))
		}
	}

	// Record errors
	if err != nil {
		c.errorsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// buildAttributes builds attributes for metrics and spans.
func (c *OTelCollector) buildAttributes(op, path string, err error) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(c.config.ConstAttributes)+3)
	attrs = append(attrs, c.config.ConstAttributes...)
	attrs = append(attrs, attribute.String("operation", op))

	if path != "" {
		attrs = append(attrs, attribute.String("path", path))
	}

	if err != nil {
		attrs = append(attrs, attribute.String("error.type", categorizeError(err)))
	} else {
		attrs = append(attrs, attribute.String("status", "success"))
	}

	return attrs
}

// categorizeError categorizes errors into types.
func categorizeError(err error) string {
	if err == nil {
		return ""
	}

	if os.IsNotExist(err) {
		return "not_found"
	} else if os.IsPermission(err) {
		return "permission"
	} else if os.IsTimeout(err) {
		return "timeout"
	}

	return "unknown"
}

// OTelMetricsFS wraps an absfs.FileSystem with OpenTelemetry instrumentation.
type OTelMetricsFS struct {
	fs        absfs.FileSystem
	collector *OTelCollector
}

// NewWithOTel creates a new filesystem wrapper with OpenTelemetry instrumentation.
func NewWithOTel(fs absfs.FileSystem, config OTelConfig) (*OTelMetricsFS, error) {
	collector, err := NewOTelCollector(config)
	if err != nil {
		return nil, err
	}

	return &OTelMetricsFS{
		fs:        fs,
		collector: collector,
	}, nil
}

// Collector returns the OpenTelemetry collector.
func (m *OTelMetricsFS) Collector() *OTelCollector {
	return m.collector
}

// Open opens a file for reading with tracing support.
func (m *OTelMetricsFS) Open(name string) (absfs.File, error) {
	return m.OpenWithContext(context.Background(), name)
}

// OpenWithContext opens a file for reading with context and tracing.
func (m *OTelMetricsFS) OpenWithContext(ctx context.Context, name string) (absfs.File, error) {
	ctx, span := m.startSpan(ctx, "Open", name)
	defer span.End()

	start := time.Now()
	f, err := m.fs.Open(name)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "open", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return newOTelMetricsFile(f, m.collector, name, ctx), nil
}

// OpenFile opens a file with the specified flags and mode.
func (m *OTelMetricsFS) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	return m.OpenFileWithContext(context.Background(), name, flag, perm)
}

// OpenFileWithContext opens a file with context and tracing.
func (m *OTelMetricsFS) OpenFileWithContext(ctx context.Context, name string, flag int, perm os.FileMode) (absfs.File, error) {
	ctx, span := m.startSpan(ctx, "OpenFile", name)
	defer span.End()

	start := time.Now()
	f, err := m.fs.OpenFile(name, flag, perm)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "openfile", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return newOTelMetricsFile(f, m.collector, name, ctx), nil
}

// Stat returns file information with tracing.
func (m *OTelMetricsFS) Stat(name string) (os.FileInfo, error) {
	return m.StatWithContext(context.Background(), name)
}

// StatWithContext returns file information with context and tracing.
func (m *OTelMetricsFS) StatWithContext(ctx context.Context, name string) (os.FileInfo, error) {
	ctx, span := m.startSpan(ctx, "Stat", name)
	defer span.End()

	start := time.Now()
	info, err := m.fs.Stat(name)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "stat", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return info, err
}

// startSpan starts a new span for tracing if enabled.
func (m *OTelMetricsFS) startSpan(ctx context.Context, operation, path string) (context.Context, trace.Span) {
	if !m.collector.config.EnableTracing {
		return ctx, trace.SpanFromContext(ctx)
	}

	return m.collector.tracer.Start(ctx, operation,
		trace.WithAttributes(
			attribute.String("fs.operation", operation),
			attribute.String("fs.path", path),
		),
	)
}

// Additional filesystem methods would follow the same pattern...
// (truncated for brevity - in practice, all methods would be implemented)

// otelMetricsFile wraps a file with OpenTelemetry instrumentation.
type otelMetricsFile struct {
	file      absfs.File
	collector *OTelCollector
	path      string
	ctx       context.Context
}

// newOTelMetricsFile creates a new OpenTelemetry instrumented file wrapper.
func newOTelMetricsFile(f absfs.File, collector *OTelCollector, path string, ctx context.Context) *otelMetricsFile {
	collector.openFilesGauge.Add(ctx, 1)

	return &otelMetricsFile{
		file:      f,
		collector: collector,
		path:      path,
		ctx:       ctx,
	}
}

// Read reads data from the file with metrics.
func (f *otelMetricsFile) Read(p []byte) (n int, err error) {
	ctx, span := f.startSpan("Read")
	defer span.End()

	start := time.Now()
	n, err = f.file.Read(p)
	duration := time.Since(start)

	f.collector.recordOperation(ctx, "read", f.path, duration, int64(n), err)

	if err != nil && err != os.ErrClosed {
		span.RecordError(err)
	}

	return n, err
}

// Write writes data to the file with metrics.
func (f *otelMetricsFile) Write(p []byte) (n int, err error) {
	ctx, span := f.startSpan("Write")
	defer span.End()

	start := time.Now()
	n, err = f.file.Write(p)
	duration := time.Since(start)

	f.collector.recordOperation(ctx, "write", f.path, duration, int64(n), err)

	if err != nil {
		span.RecordError(err)
	}

	return n, err
}

// Close closes the file.
func (f *otelMetricsFile) Close() error {
	ctx, span := f.startSpan("Close")
	defer span.End()

	start := time.Now()
	err := f.file.Close()
	duration := time.Since(start)

	f.collector.recordOperation(ctx, "close", f.path, duration, 0, err)
	f.collector.openFilesGauge.Add(ctx, -1)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

// startSpan starts a new span for file operations.
func (f *otelMetricsFile) startSpan(operation string) (context.Context, trace.Span) {
	if !f.collector.config.EnableTracing {
		return f.ctx, trace.SpanFromContext(f.ctx)
	}

	return f.collector.tracer.Start(f.ctx, operation,
		trace.WithAttributes(
			attribute.String("fs.operation", operation),
			attribute.String("fs.path", f.path),
		),
	)
}

// Delegate other methods to underlying file
func (f *otelMetricsFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.file.ReadAt(p, off)
}

func (f *otelMetricsFile) WriteAt(p []byte, off int64) (n int, err error) {
	return f.file.WriteAt(p, off)
}

func (f *otelMetricsFile) WriteString(s string) (n int, err error) {
	return f.file.WriteString(s)
}

func (f *otelMetricsFile) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

func (f *otelMetricsFile) Stat() (os.FileInfo, error) {
	return f.file.Stat()
}

func (f *otelMetricsFile) Sync() error {
	return f.file.Sync()
}

func (f *otelMetricsFile) Truncate(size int64) error {
	return f.file.Truncate(size)
}

func (f *otelMetricsFile) Readdir(n int) ([]os.FileInfo, error) {
	return f.file.Readdir(n)
}

func (f *otelMetricsFile) Readdirnames(n int) ([]string, error) {
	return f.file.Readdirnames(n)
}

func (f *otelMetricsFile) Name() string {
	return f.file.Name()
}
