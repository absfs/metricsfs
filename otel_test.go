package metricsfs

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestNewOTelCollector(t *testing.T) {
	config := OTelConfig{
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}

	collector, err := NewOTelCollector(config)
	if err != nil {
		t.Fatalf("NewOTelCollector failed: %v", err)
	}

	if collector == nil {
		t.Fatal("NewOTelCollector returned nil")
	}
}

func TestNewOTelCollectorDefaults(t *testing.T) {
	// Test that defaults are applied
	config := OTelConfig{}

	collector, err := NewOTelCollector(config)
	if err != nil {
		t.Fatalf("NewOTelCollector failed: %v", err)
	}

	if collector == nil {
		t.Fatal("NewOTelCollector returned nil")
	}
}

func TestNewWithOTel(t *testing.T) {
	base := newMockFS()
	otelConfig := OTelConfig{
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}

	fs, err := NewWithOTel(base, otelConfig)
	if err != nil {
		t.Fatalf("NewWithOTel failed: %v", err)
	}

	if fs == nil {
		t.Fatal("NewWithOTel returned nil")
	}
}

func TestOTelMetricsFSOperations(t *testing.T) {
	base := newMockFS()
	otelConfig := OTelConfig{
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
		EnableTracing:  true,
	}

	fs, err := NewWithOTel(base, otelConfig)
	if err != nil {
		t.Fatalf("NewWithOTel failed: %v", err)
	}

	// Test Open
	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}
	if f != nil {
		f.Close()
	}

	// Test OpenWithContext
	ctx := context.Background()
	f, err = fs.OpenWithContext(ctx, "/test.txt")
	if err != nil {
		t.Errorf("OpenWithContext failed: %v", err)
	}
	if f != nil {
		f.Close()
	}

	// Test OpenFile
	f, err = fs.OpenFile("/test.txt", os.O_RDONLY, 0644)
	if err != nil {
		t.Errorf("OpenFile failed: %v", err)
	}
	if f != nil {
		f.Close()
	}

	// Test OpenFileWithContext
	f, err = fs.OpenFileWithContext(ctx, "/test.txt", os.O_RDONLY, 0644)
	if err != nil {
		t.Errorf("OpenFileWithContext failed: %v", err)
	}
	if f != nil {
		f.Close()
	}

	// Test Create
	f, err = fs.Create("/new.txt")
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}
	if f != nil {
		f.Close()
	}

	// Test CreateWithContext
	f, err = fs.CreateWithContext(ctx, "/new.txt")
	if err != nil {
		t.Errorf("CreateWithContext failed: %v", err)
	}
	if f != nil {
		f.Close()
	}

	// Test Stat
	_, err = fs.Stat("/test.txt")
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}

	// Test StatWithContext
	_, err = fs.StatWithContext(ctx, "/test.txt")
	if err != nil {
		t.Errorf("StatWithContext failed: %v", err)
	}

	// Test Mkdir
	err = fs.Mkdir("/newdir", 0755)
	if err != nil {
		t.Errorf("Mkdir failed: %v", err)
	}

	// Test MkdirWithContext
	err = fs.MkdirWithContext(ctx, "/newdir2", 0755)
	if err != nil {
		t.Errorf("MkdirWithContext failed: %v", err)
	}

	// Test MkdirAll
	err = fs.MkdirAll("/path/to/dir", 0755)
	if err != nil {
		t.Errorf("MkdirAll failed: %v", err)
	}

	// Test MkdirAllWithContext
	err = fs.MkdirAllWithContext(ctx, "/path/to/dir2", 0755)
	if err != nil {
		t.Errorf("MkdirAllWithContext failed: %v", err)
	}

	// Test Remove
	err = fs.Remove("/test.txt")
	if err != nil {
		t.Errorf("Remove failed: %v", err)
	}

	// Test RemoveWithContext
	err = fs.RemoveWithContext(ctx, "/test2.txt")
	if err != nil {
		t.Errorf("RemoveWithContext failed: %v", err)
	}

	// Test RemoveAll
	err = fs.RemoveAll("/dir")
	if err != nil {
		t.Errorf("RemoveAll failed: %v", err)
	}

	// Test RemoveAllWithContext
	err = fs.RemoveAllWithContext(ctx, "/dir2")
	if err != nil {
		t.Errorf("RemoveAllWithContext failed: %v", err)
	}

	// Test Rename
	err = fs.Rename("/old.txt", "/new.txt")
	if err != nil {
		t.Errorf("Rename failed: %v", err)
	}

	// Test RenameWithContext
	err = fs.RenameWithContext(ctx, "/old2.txt", "/new2.txt")
	if err != nil {
		t.Errorf("RenameWithContext failed: %v", err)
	}

	// Test Chmod
	err = fs.Chmod("/test.txt", 0644)
	if err != nil {
		t.Errorf("Chmod failed: %v", err)
	}

	// Test ChmodWithContext
	err = fs.ChmodWithContext(ctx, "/test.txt", 0644)
	if err != nil {
		t.Errorf("ChmodWithContext failed: %v", err)
	}

	// Test Chown
	err = fs.Chown("/test.txt", 1000, 1000)
	if err != nil {
		t.Errorf("Chown failed: %v", err)
	}

	// Test ChownWithContext
	err = fs.ChownWithContext(ctx, "/test.txt", 1000, 1000)
	if err != nil {
		t.Errorf("ChownWithContext failed: %v", err)
	}

	// Test Chtimes
	now := time.Now()
	err = fs.Chtimes("/test.txt", now, now)
	if err != nil {
		t.Errorf("Chtimes failed: %v", err)
	}

	// Test ChtimesWithContext
	err = fs.ChtimesWithContext(ctx, "/test.txt", now, now)
	if err != nil {
		t.Errorf("ChtimesWithContext failed: %v", err)
	}

	// Test Lstat
	_, err = fs.Lstat("/test.txt")
	if err != nil {
		t.Errorf("Lstat failed: %v", err)
	}

	// Test LstatWithContext
	_, err = fs.LstatWithContext(ctx, "/test.txt")
	if err != nil {
		t.Errorf("LstatWithContext failed: %v", err)
	}

	// Test Readlink (should fail as mockFS doesn't implement it)
	_, err = fs.Readlink("/link")
	if err == nil {
		t.Error("Expected Readlink to fail")
	}

	// Test ReadlinkWithContext (should fail as mockFS doesn't implement it)
	_, err = fs.ReadlinkWithContext(ctx, "/link")
	if err == nil {
		t.Error("Expected ReadlinkWithContext to fail")
	}

	// Test Symlink (should fail as mockFS doesn't implement it)
	err = fs.Symlink("/target", "/link")
	if err == nil {
		t.Error("Expected Symlink to fail")
	}

	// Test SymlinkWithContext (should fail as mockFS doesn't implement it)
	err = fs.SymlinkWithContext(ctx, "/target", "/link")
	if err == nil {
		t.Error("Expected SymlinkWithContext to fail")
	}

	// Test Collector
	collector := fs.Collector()
	if collector == nil {
		t.Error("Collector returned nil")
	}
}

func TestOTelMetricsFileOperations(t *testing.T) {
	base := newMockFS()
	otelConfig := OTelConfig{
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
		EnableTracing:  true,
	}

	fs, err := NewWithOTel(base, otelConfig)
	if err != nil {
		t.Fatalf("NewWithOTel failed: %v", err)
	}

	// Open a file
	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Test Read
	buf := make([]byte, 10)
	_, err = f.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}

	// Test Write
	_, err = f.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Test ReadAt
	_, err = f.ReadAt(buf, 0)
	if err != nil {
		t.Errorf("ReadAt failed: %v", err)
	}

	// Test WriteAt
	_, err = f.WriteAt([]byte("hello"), 0)
	if err != nil {
		t.Errorf("WriteAt failed: %v", err)
	}

	// Test WriteString
	_, err = f.WriteString("hello")
	if err != nil {
		t.Errorf("WriteString failed: %v", err)
	}

	// Test Seek
	_, err = f.Seek(0, 0)
	if err != nil {
		t.Errorf("Seek failed: %v", err)
	}

	// Test Stat
	_, err = f.Stat()
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}

	// Test Sync
	err = f.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// Test Truncate
	err = f.Truncate(10)
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}

	// Test Readdir
	_, err = f.Readdir(-1)
	if err != nil {
		t.Errorf("Readdir failed: %v", err)
	}

	// Test Readdirnames
	_, err = f.Readdirnames(-1)
	if err != nil {
		t.Errorf("Readdirnames failed: %v", err)
	}

	// Test Name
	name := f.Name()
	if name != "/test.txt" {
		t.Errorf("Expected name '/test.txt', got '%s'", name)
	}
}

func TestOTelCategorizeError(t *testing.T) {
	base := newMockFS()
	otelConfig := OTelConfig{
		MeterProvider:  noop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}

	fs, err := NewWithOTel(base, otelConfig)
	if err != nil {
		t.Fatalf("NewWithOTel failed: %v", err)
	}

	// Test with error-returning filesystem to trigger error paths
	errorFS := &errorMockFS{}
	errorOTelFS, err := NewWithOTel(errorFS, otelConfig)
	if err != nil {
		t.Fatalf("NewWithOTel with error fs failed: %v", err)
	}

	// These should fail and test error categorization
	_, err = errorOTelFS.Open("/nonexistent")
	if err == nil {
		t.Error("Expected Open to fail")
	}

	_, err = errorOTelFS.Stat("/nonexistent")
	if err == nil {
		t.Error("Expected Stat to fail")
	}

	// Use the successful fs for other operations
	_ = fs
}
