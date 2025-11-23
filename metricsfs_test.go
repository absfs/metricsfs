package metricsfs

import (
	"os"
	"testing"
	"time"

	"github.com/absfs/absfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// mockFS is a minimal mock filesystem for testing.
type mockFS struct {
	absfs.SymlinkFileSystem
}

func newMockFS() *mockFS {
	return &mockFS{}
}

func (m *mockFS) Open(name string) (absfs.File, error) {
	return &mockFile{name: name}, nil
}

func (m *mockFS) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	return &mockFile{name: name}, nil
}

func (m *mockFS) Create(name string) (absfs.File, error) {
	return &mockFile{name: name}, nil
}

func (m *mockFS) Stat(name string) (os.FileInfo, error) {
	return &mockFileInfo{name: name}, nil
}

func (m *mockFS) Lstat(name string) (os.FileInfo, error) {
	return &mockFileInfo{name: name}, nil
}

func (m *mockFS) Mkdir(name string, perm os.FileMode) error {
	return nil
}

func (m *mockFS) MkdirAll(name string, perm os.FileMode) error {
	return nil
}

func (m *mockFS) Remove(name string) error {
	return nil
}

func (m *mockFS) RemoveAll(name string) error {
	return nil
}

func (m *mockFS) Rename(oldpath, newpath string) error {
	return nil
}

// mockFile is a minimal mock file for testing.
type mockFile struct {
	name   string
	data   []byte
	offset int64
}

func (f *mockFile) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (f *mockFile) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, nil
}

func (f *mockFile) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (f *mockFile) WriteAt(p []byte, off int64) (n int, err error) {
	return len(p), nil
}

func (f *mockFile) WriteString(s string) (n int, err error) {
	return len(s), nil
}

func (f *mockFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (f *mockFile) Close() error {
	return nil
}

func (f *mockFile) Stat() (os.FileInfo, error) {
	return &mockFileInfo{name: f.name}, nil
}

func (f *mockFile) Sync() error {
	return nil
}

func (f *mockFile) Truncate(size int64) error {
	return nil
}

func (f *mockFile) Readdir(n int) ([]os.FileInfo, error) {
	return nil, nil
}

func (f *mockFile) Readdirnames(n int) ([]string, error) {
	return nil, nil
}

func (f *mockFile) Name() string {
	return f.name
}

// mockFileInfo is a minimal mock file info for testing.
type mockFileInfo struct {
	name string
}

func (i *mockFileInfo) Name() string       { return i.name }
func (i *mockFileInfo) Size() int64        { return 0 }
func (i *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (i *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (i *mockFileInfo) IsDir() bool        { return false }
func (i *mockFileInfo) Sys() interface{}   { return nil }

func TestNew(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	if fs == nil {
		t.Fatal("New() returned nil")
	}

	if fs.collector == nil {
		t.Fatal("collector is nil")
	}
}

func TestNewWithConfig(t *testing.T) {
	base := newMockFS()
	config := Config{
		Namespace:              "test",
		Subsystem:              "fs",
		EnableLatencyMetrics:   true,
		EnableBandwidthMetrics: true,
	}
	config.applyDefaults()

	fs := NewWithConfig(base, config)

	if fs == nil {
		t.Fatal("NewWithConfig() returned nil")
	}

	if fs.collector.config.Namespace != "test" {
		t.Errorf("Expected namespace 'test', got '%s'", fs.collector.config.Namespace)
	}
}

func TestOperationCounting(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	// Create a new registry for this test
	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	// Perform some operations
	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	f.Close()

	fs.Stat("/test.txt")
	fs.Mkdir("/testdir", 0755)

	// Check that metrics were collected
	count, err := testutil.GatherAndCount(registry)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if count == 0 {
		t.Error("No metrics were collected")
	}
}

func TestFileOperations(t *testing.T) {
	base := newMockFS()
	config := DefaultConfig()
	config.EnableBandwidthMetrics = true
	fs := NewWithConfig(base, config)

	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	// Test Create
	f, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test Write
	data := []byte("hello world")
	n, err := f.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Test Close
	err = f.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify metrics
	count, err := testutil.GatherAndCount(registry)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if count == 0 {
		t.Error("No metrics were collected")
	}
}

func TestErrorTracking(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	// Trigger a not found error (using mock that returns not found)
	errorFS := &errorMockFS{}
	errorMetricsFS := New(errorFS)

	registry2 := prometheus.NewRegistry()
	registry2.MustRegister(errorMetricsFS.Collector())

	_, err := errorMetricsFS.Open("/nonexistent")
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Verify error metrics were collected
	count, err := testutil.GatherAndCount(registry2)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if count == 0 {
		t.Error("No error metrics were collected")
	}
}

// errorMockFS returns errors for all operations.
type errorMockFS struct {
	mockFS
}

func (e *errorMockFS) Open(name string) (absfs.File, error) {
	return nil, os.ErrNotExist
}

func (e *errorMockFS) Stat(name string) (os.FileInfo, error) {
	return nil, os.ErrPermission
}

func TestFileDescriptorTracking(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	// Open multiple files
	f1, _ := fs.Open("/file1.txt")
	f2, _ := fs.Open("/file2.txt")
	f3, _ := fs.Open("/file3.txt")

	// Check open file count
	openCount := fs.collector.openFiles.Load()
	if openCount != 3 {
		t.Errorf("Expected 3 open files, got %d", openCount)
	}

	// Close files
	f1.Close()
	f2.Close()

	openCount = fs.collector.openFiles.Load()
	if openCount != 1 {
		t.Errorf("Expected 1 open file, got %d", openCount)
	}

	f3.Close()

	openCount = fs.collector.openFiles.Load()
	if openCount != 0 {
		t.Errorf("Expected 0 open files, got %d", openCount)
	}

	// Check max open files
	maxOpen := fs.collector.openFilesMax.Load()
	if maxOpen != 3 {
		t.Errorf("Expected max 3 open files, got %d", maxOpen)
	}
}

func TestConfigDefaults(t *testing.T) {
	config := DefaultConfig()

	if config.Namespace != "fs" {
		t.Errorf("Expected default namespace 'fs', got '%s'", config.Namespace)
	}

	if !config.EnableLatencyMetrics {
		t.Error("Expected latency metrics to be enabled by default")
	}

	if !config.EnableBandwidthMetrics {
		t.Error("Expected bandwidth metrics to be enabled by default")
	}

	if config.EnablePathMetrics {
		t.Error("Expected path metrics to be disabled by default")
	}

	if len(config.LatencyBuckets) == 0 {
		t.Error("Expected latency buckets to be set")
	}

	if len(config.SizeBuckets) == 0 {
		t.Error("Expected size buckets to be set")
	}
}

func TestOnOperationCallback(t *testing.T) {
	base := newMockFS()

	var called bool
	var capturedOp Operation

	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		called = true
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	fs.Stat("/test.txt")

	if !called {
		t.Error("OnOperation callback was not called")
	}

	if capturedOp.Name != "stat" {
		t.Errorf("Expected operation 'stat', got '%s'", capturedOp.Name)
	}
}

func TestOnErrorCallback(t *testing.T) {
	errorFS := &errorMockFS{}

	var called bool
	var capturedOp string

	config := DefaultConfig()
	config.OnError = func(operation string, err error) {
		called = true
		capturedOp = operation
	}

	fs := NewWithConfig(errorFS, config)

	fs.Open("/nonexistent")

	if !called {
		t.Error("OnError callback was not called")
	}

	if capturedOp != "open" {
		t.Errorf("Expected operation 'open', got '%s'", capturedOp)
	}
}
