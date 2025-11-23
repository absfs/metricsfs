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

// Create creates a new file.
func (m *OTelMetricsFS) Create(name string) (absfs.File, error) {
	return m.CreateWithContext(context.Background(), name)
}

// CreateWithContext creates a new file with context and tracing.
func (m *OTelMetricsFS) CreateWithContext(ctx context.Context, name string) (absfs.File, error) {
	ctx, span := m.startSpan(ctx, "Create", name)
	defer span.End()

	start := time.Now()
	f, err := m.fs.Create(name)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "create", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return newOTelMetricsFile(f, m.collector, name, ctx), nil
}

// Mkdir creates a directory.
func (m *OTelMetricsFS) Mkdir(name string, perm os.FileMode) error {
	return m.MkdirWithContext(context.Background(), name, perm)
}

// MkdirWithContext creates a directory with context and tracing.
func (m *OTelMetricsFS) MkdirWithContext(ctx context.Context, name string, perm os.FileMode) error {
	ctx, span := m.startSpan(ctx, "Mkdir", name)
	defer span.End()

	start := time.Now()
	err := m.fs.Mkdir(name, perm)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "mkdir", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// MkdirAll creates a directory and all necessary parent directories.
func (m *OTelMetricsFS) MkdirAll(name string, perm os.FileMode) error {
	return m.MkdirAllWithContext(context.Background(), name, perm)
}

// MkdirAllWithContext creates a directory and parents with context and tracing.
func (m *OTelMetricsFS) MkdirAllWithContext(ctx context.Context, name string, perm os.FileMode) error {
	ctx, span := m.startSpan(ctx, "MkdirAll", name)
	defer span.End()

	start := time.Now()
	err := m.fs.MkdirAll(name, perm)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "mkdirall", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// Remove removes a file or directory.
func (m *OTelMetricsFS) Remove(name string) error {
	return m.RemoveWithContext(context.Background(), name)
}

// RemoveWithContext removes a file or directory with context and tracing.
func (m *OTelMetricsFS) RemoveWithContext(ctx context.Context, name string) error {
	ctx, span := m.startSpan(ctx, "Remove", name)
	defer span.End()

	start := time.Now()
	err := m.fs.Remove(name)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "remove", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// RemoveAll removes a path and all children.
func (m *OTelMetricsFS) RemoveAll(name string) error {
	return m.RemoveAllWithContext(context.Background(), name)
}

// RemoveAllWithContext removes a path and all children with context and tracing.
func (m *OTelMetricsFS) RemoveAllWithContext(ctx context.Context, name string) error {
	ctx, span := m.startSpan(ctx, "RemoveAll", name)
	defer span.End()

	start := time.Now()
	err := m.fs.RemoveAll(name)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "removeall", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// Rename renames a file or directory.
func (m *OTelMetricsFS) Rename(oldpath, newpath string) error {
	return m.RenameWithContext(context.Background(), oldpath, newpath)
}

// RenameWithContext renames a file or directory with context and tracing.
func (m *OTelMetricsFS) RenameWithContext(ctx context.Context, oldpath, newpath string) error {
	ctx, span := m.startSpan(ctx, "Rename", oldpath)
	span.SetAttributes(attribute.String("fs.newpath", newpath))
	defer span.End()

	start := time.Now()
	err := m.fs.Rename(oldpath, newpath)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "rename", oldpath, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// Chmod changes file permissions.
func (m *OTelMetricsFS) Chmod(name string, mode os.FileMode) error {
	return m.ChmodWithContext(context.Background(), name, mode)
}

// ChmodWithContext changes file permissions with context and tracing.
func (m *OTelMetricsFS) ChmodWithContext(ctx context.Context, name string, mode os.FileMode) error {
	ctx, span := m.startSpan(ctx, "Chmod", name)
	span.SetAttributes(attribute.String("fs.mode", mode.String()))
	defer span.End()

	start := time.Now()
	err := m.fs.Chmod(name, mode)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "chmod", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// Chown changes file ownership.
func (m *OTelMetricsFS) Chown(name string, uid, gid int) error {
	return m.ChownWithContext(context.Background(), name, uid, gid)
}

// ChownWithContext changes file ownership with context and tracing.
func (m *OTelMetricsFS) ChownWithContext(ctx context.Context, name string, uid, gid int) error {
	ctx, span := m.startSpan(ctx, "Chown", name)
	span.SetAttributes(
		attribute.Int("fs.uid", uid),
		attribute.Int("fs.gid", gid),
	)
	defer span.End()

	start := time.Now()
	err := m.fs.Chown(name, uid, gid)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "chown", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// Chtimes changes file access and modification times.
func (m *OTelMetricsFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return m.ChtimesWithContext(context.Background(), name, atime, mtime)
}

// ChtimesWithContext changes file times with context and tracing.
func (m *OTelMetricsFS) ChtimesWithContext(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	ctx, span := m.startSpan(ctx, "Chtimes", name)
	defer span.End()

	start := time.Now()
	err := m.fs.Chtimes(name, atime, mtime)
	duration := time.Since(start)

	m.collector.recordOperation(ctx, "chtimes", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

// Lstat returns file information without following symlinks.
func (m *OTelMetricsFS) Lstat(name string) (os.FileInfo, error) {
	return m.LstatWithContext(context.Background(), name)
}

// LstatWithContext returns file information without following symlinks with context and tracing.
func (m *OTelMetricsFS) LstatWithContext(ctx context.Context, name string) (os.FileInfo, error) {
	ctx, span := m.startSpan(ctx, "Lstat", name)
	defer span.End()

	start := time.Now()

	// Check if underlying filesystem supports Lstat
	var info os.FileInfo
	var err error
	if sfs, ok := m.fs.(interface {
		Lstat(name string) (os.FileInfo, error)
	}); ok {
		info, err = sfs.Lstat(name)
	} else {
		// Fallback to Stat if Lstat not available
		info, err = m.fs.Stat(name)
	}

	duration := time.Since(start)
	m.collector.recordOperation(ctx, "lstat", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return info, err
}

// Readlink reads the target of a symbolic link.
func (m *OTelMetricsFS) Readlink(name string) (string, error) {
	return m.ReadlinkWithContext(context.Background(), name)
}

// ReadlinkWithContext reads the target of a symbolic link with context and tracing.
func (m *OTelMetricsFS) ReadlinkWithContext(ctx context.Context, name string) (string, error) {
	ctx, span := m.startSpan(ctx, "Readlink", name)
	defer span.End()

	start := time.Now()

	// Check if underlying filesystem supports Readlink
	var target string
	var err error
	if sfs, ok := m.fs.(interface {
		Readlink(name string) (string, error)
	}); ok {
		target, err = sfs.Readlink(name)
	} else {
		err = os.ErrInvalid
	}

	duration := time.Since(start)
	m.collector.recordOperation(ctx, "readlink", name, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return "", err
	}

	span.SetAttributes(attribute.String("fs.target", target))
	return target, nil
}

// Symlink creates a symbolic link.
func (m *OTelMetricsFS) Symlink(oldname, newname string) error {
	return m.SymlinkWithContext(context.Background(), oldname, newname)
}

// SymlinkWithContext creates a symbolic link with context and tracing.
func (m *OTelMetricsFS) SymlinkWithContext(ctx context.Context, oldname, newname string) error {
	ctx, span := m.startSpan(ctx, "Symlink", newname)
	span.SetAttributes(attribute.String("fs.target", oldname))
	defer span.End()

	start := time.Now()

	// Check if underlying filesystem supports Symlink
	var err error
	if sfs, ok := m.fs.(interface {
		Symlink(oldname, newname string) error
	}); ok {
		err = sfs.Symlink(oldname, newname)
	} else {
		err = os.ErrInvalid
	}

	duration := time.Since(start)
	m.collector.recordOperation(ctx, "symlink", newname, duration, 0, err)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}

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
