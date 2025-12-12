package metricsfs

import (
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/absfs/absfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// mockFS is a minimal mock filesystem for testing.
// Note: Does not implement Readlink/Symlink - tests for those use symlinkerMockFS
type mockFS struct {
	cwd string
}

func newMockFS() *mockFS {
	return &mockFS{cwd: "/"}
}

func (m *mockFS) Chdir(dir string) error {
	m.cwd = dir
	return nil
}

func (m *mockFS) Getwd() (string, error) {
	return m.cwd, nil
}

func (m *mockFS) TempDir() string {
	return os.TempDir()
}

func (m *mockFS) Truncate(name string, size int64) error {
	return nil
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

func (m *mockFS) Chmod(name string, mode os.FileMode) error {
	return nil
}

func (m *mockFS) Chown(name string, uid, gid int) error {
	return nil
}

func (m *mockFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return nil
}

func (m *mockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return nil, nil
}

func (m *mockFS) ReadFile(name string) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockFS) Sub(dir string) (fs.FS, error) {
	return absfs.FilerToFS(m, dir)
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

func (f *mockFile) ReadDir(n int) ([]fs.DirEntry, error) {
	return nil, nil
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

func TestMkdirAll(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.MkdirAll("/path/to/dir", 0755)
	if err != nil {
		t.Errorf("MkdirAll failed: %v", err)
	}
}

func TestRemove(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.Remove("/test.txt")
	if err != nil {
		t.Errorf("Remove failed: %v", err)
	}
}

func TestRemoveAll(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.RemoveAll("/testdir")
	if err != nil {
		t.Errorf("RemoveAll failed: %v", err)
	}
}

func TestRename(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.Rename("/old.txt", "/new.txt")
	if err != nil {
		t.Errorf("Rename failed: %v", err)
	}
}

func TestOpenFile(t *testing.T) {
	base := newMockFS()
	config := DefaultConfig()
	fs := NewWithConfig(base, config)

	// Test read mode
	f, err := fs.OpenFile("/test.txt", os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	f.Close()

	// Test write mode
	f, err = fs.OpenFile("/test.txt", os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile WRONLY failed: %v", err)
	}
	f.Close()

	// Test readwrite mode
	f, err = fs.OpenFile("/test.txt", os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("OpenFile RDWR failed: %v", err)
	}
	f.Close()

	// Test append mode
	f, err = fs.OpenFile("/test.txt", os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("OpenFile APPEND failed: %v", err)
	}
	f.Close()
}

func TestLstat(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	info, err := fs.Lstat("/test.txt")
	if err != nil {
		t.Errorf("Lstat failed: %v", err)
	}
	if info == nil {
		t.Error("Lstat returned nil FileInfo")
	}
}

func TestChmod(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.Chmod("/test.txt", 0644)
	if err != nil {
		t.Errorf("Chmod failed: %v", err)
	}
}

func TestChown(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.Chown("/test.txt", 1000, 1000)
	if err != nil {
		t.Errorf("Chown failed: %v", err)
	}
}

func TestChtimes(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	now := time.Now()
	err := fs.Chtimes("/test.txt", now, now)
	if err != nil {
		t.Errorf("Chtimes failed: %v", err)
	}
}

func TestReadlink(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	// mockFS doesn't implement Readlink, so this should return os.ErrInvalid
	_, err := fs.Readlink("/test.txt")
	if err != os.ErrInvalid {
		t.Errorf("Expected os.ErrInvalid, got: %v", err)
	}
}

func TestSymlink(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	// mockFS doesn't implement Symlink, so this should return os.ErrInvalid
	err := fs.Symlink("/old", "/new")
	if err != os.ErrInvalid {
		t.Errorf("Expected os.ErrInvalid, got: %v", err)
	}
}

// symlinkerMockFS extends mockFS to support symlink operations
type symlinkerMockFS struct {
	mockFS
}

func (s *symlinkerMockFS) Readlink(name string) (string, error) {
	return "/target", nil
}

func (s *symlinkerMockFS) Symlink(oldname, newname string) error {
	return nil
}

func TestReadlinkWithSupport(t *testing.T) {
	base := &symlinkerMockFS{}
	fs := New(base)

	target, err := fs.Readlink("/link")
	if err != nil {
		t.Errorf("Readlink failed: %v", err)
	}
	if target != "/target" {
		t.Errorf("Expected target '/target', got '%s'", target)
	}
}

func TestSymlinkWithSupport(t *testing.T) {
	base := &symlinkerMockFS{}
	fs := New(base)

	err := fs.Symlink("/old", "/new")
	if err != nil {
		t.Errorf("Symlink failed: %v", err)
	}
}

func TestFileRead(t *testing.T) {
	base := newMockFS()
	config := DefaultConfig()
	config.EnableBandwidthMetrics = true
	fs := NewWithConfig(base, config)

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 10)
	_, err = f.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
}

func TestFileReadAt(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 10)
	_, err = f.ReadAt(buf, 0)
	if err != nil {
		t.Errorf("ReadAt failed: %v", err)
	}
}

func TestFileWriteAt(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer f.Close()

	_, err = f.WriteAt([]byte("hello"), 0)
	if err != nil {
		t.Errorf("WriteAt failed: %v", err)
	}
}

func TestFileWriteString(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer f.Close()

	_, err = f.WriteString("hello")
	if err != nil {
		t.Errorf("WriteString failed: %v", err)
	}
}

func TestFileSeek(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	_, err = f.Seek(0, 0)
	if err != nil {
		t.Errorf("Seek failed: %v", err)
	}
}

func TestFileStat(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}
	if info == nil {
		t.Error("Stat returned nil FileInfo")
	}
}

func TestFileSync(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer f.Close()

	err = f.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}
}

func TestFileTruncate(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer f.Close()

	err = f.Truncate(100)
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}
}

func TestFileReaddir(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Open("/")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	_, err = f.Readdir(-1)
	if err != nil {
		t.Errorf("Readdir failed: %v", err)
	}
}

func TestFileReaddirnames(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Open("/")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	_, err = f.Readdirnames(-1)
	if err != nil {
		t.Errorf("Readdirnames failed: %v", err)
	}
}

func TestFileName(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	f, err := fs.Open("/test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	name := f.Name()
	if name != "/test.txt" {
		t.Errorf("Expected name '/test.txt', got '%s'", name)
	}
}

func TestPathMetrics(t *testing.T) {
	base := newMockFS()
	config := DefaultConfig()
	config.EnablePathMetrics = true
	fs := NewWithConfig(base, config)

	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	// Access multiple paths
	for i := 0; i < 5; i++ {
		fs.Stat("/test" + string(rune('0'+i)) + ".txt")
	}

	// Verify metrics were collected
	count, err := testutil.GatherAndCount(registry)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}
	if count == 0 {
		t.Error("No metrics were collected with path metrics enabled")
	}
}

func TestChdir(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	err := fs.Chdir("/home/test")
	if err != nil {
		t.Errorf("Chdir failed: %v", err)
	}

	// Verify underlying filesystem received the change
	cwd, _ := base.Getwd()
	if cwd != "/home/test" {
		t.Errorf("Expected cwd /home/test, got %s", cwd)
	}

	// Check that metrics were collected
	count, err := testutil.GatherAndCount(registry)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}
	if count == 0 {
		t.Error("No metrics were collected for Chdir")
	}
}

func TestChdirMetrics(t *testing.T) {
	base := newMockFS()

	var capturedOp Operation
	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	fs.Chdir("/tmp")

	if capturedOp.Name != "chdir" {
		t.Errorf("Expected operation 'chdir', got '%s'", capturedOp.Name)
	}
}

func TestGetwd(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	// Set a known directory first
	base.Chdir("/home/test")

	dir, err := fs.Getwd()
	if err != nil {
		t.Errorf("Getwd failed: %v", err)
	}

	if dir != "/home/test" {
		t.Errorf("Expected dir /home/test, got %s", dir)
	}

	// Check that metrics were collected
	count, err := testutil.GatherAndCount(registry)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}
	if count == 0 {
		t.Error("No metrics were collected for Getwd")
	}
}

func TestGetwdMetrics(t *testing.T) {
	base := newMockFS()

	var capturedOp Operation
	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	fs.Getwd()

	if capturedOp.Name != "getwd" {
		t.Errorf("Expected operation 'getwd', got '%s'", capturedOp.Name)
	}
}

func TestTempDir(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	tempDir := fs.TempDir()
	if tempDir == "" {
		t.Error("TempDir() returned empty string")
	}

	// Should match os.TempDir() since mockFS uses that
	expected := os.TempDir()
	if tempDir != expected {
		t.Errorf("Expected temp dir %s, got %s", expected, tempDir)
	}
}

func TestTempDirDelegation(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	// Since mockFS implements TempDir(), it should delegate
	tempDir := fs.TempDir()
	expected := base.TempDir()
	if tempDir != expected {
		t.Errorf("Expected temp dir %s from delegation, got %s", expected, tempDir)
	}
}

func TestTruncate(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	registry := prometheus.NewRegistry()
	registry.MustRegister(fs.Collector())

	err := fs.Truncate("/test.txt", 100)
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}

	// Check that metrics were collected
	count, err := testutil.GatherAndCount(registry)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}
	if count == 0 {
		t.Error("No metrics were collected for Truncate")
	}
}

func TestTruncateMetrics(t *testing.T) {
	base := newMockFS()

	var capturedOp Operation
	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	fs.Truncate("/test.txt", 500)

	if capturedOp.Name != "truncate" {
		t.Errorf("Expected operation 'truncate', got '%s'", capturedOp.Name)
	}

	if capturedOp.BytesTransferred != 500 {
		t.Errorf("Expected bytes transferred 500, got %d", capturedOp.BytesTransferred)
	}
}

func TestTruncateZeroSize(t *testing.T) {
	base := newMockFS()
	fs := New(base)

	err := fs.Truncate("/test.txt", 0)
	if err != nil {
		t.Errorf("Truncate to zero size failed: %v", err)
	}
}

// errorChdirMockFS is a mock that returns errors for Chdir
type errorChdirMockFS struct {
	mockFS
}

func (e *errorChdirMockFS) Chdir(dir string) error {
	return os.ErrPermission
}

func (e *errorChdirMockFS) Getwd() (string, error) {
	return "", os.ErrPermission
}

func (e *errorChdirMockFS) Truncate(name string, size int64) error {
	return os.ErrNotExist
}

func TestChdirError(t *testing.T) {
	base := &errorChdirMockFS{}

	var capturedOp Operation
	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	err := fs.Chdir("/tmp")
	if err != os.ErrPermission {
		t.Errorf("Expected os.ErrPermission, got %v", err)
	}

	if capturedOp.Name != "chdir" {
		t.Errorf("Expected operation 'chdir', got '%s'", capturedOp.Name)
	}

	if capturedOp.Error != os.ErrPermission {
		t.Errorf("Expected error os.ErrPermission, got %v", capturedOp.Error)
	}
}

func TestGetwdError(t *testing.T) {
	base := &errorChdirMockFS{}

	var capturedOp Operation
	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	_, err := fs.Getwd()
	if err != os.ErrPermission {
		t.Errorf("Expected os.ErrPermission, got %v", err)
	}

	if capturedOp.Name != "getwd" {
		t.Errorf("Expected operation 'getwd', got '%s'", capturedOp.Name)
	}

	if capturedOp.Error != os.ErrPermission {
		t.Errorf("Expected error os.ErrPermission, got %v", capturedOp.Error)
	}
}

func TestTruncateError(t *testing.T) {
	base := &errorChdirMockFS{}

	var capturedOp Operation
	config := DefaultConfig()
	config.OnOperation = func(op Operation) {
		capturedOp = op
	}

	fs := NewWithConfig(base, config)

	err := fs.Truncate("/test.txt", 100)
	if err != os.ErrNotExist {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}

	if capturedOp.Name != "truncate" {
		t.Errorf("Expected operation 'truncate', got '%s'", capturedOp.Name)
	}

	if capturedOp.Error != os.ErrNotExist {
		t.Errorf("Expected error os.ErrNotExist, got %v", capturedOp.Error)
	}

	if capturedOp.BytesTransferred != 100 {
		t.Errorf("Expected bytes transferred 100, got %d", capturedOp.BytesTransferred)
	}
}
