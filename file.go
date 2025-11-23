package metricsfs

import (
	"io"
	"os"
	"time"

	"github.com/absfs/absfs"
)

// MetricsFile wraps an absfs.File and collects metrics on file operations.
type MetricsFile struct {
	file      absfs.File
	collector *Collector
	path      string
}

// newMetricsFile creates a new MetricsFile wrapper.
func newMetricsFile(f absfs.File, collector *Collector, path string) *MetricsFile {
	mf := &MetricsFile{
		file:      f,
		collector: collector,
		path:      path,
	}

	// Track file open
	collector.trackFileOpen()

	return mf
}

// Read reads data from the file.
func (f *MetricsFile) Read(p []byte) (n int, err error) {
	start := time.Now()
	n, err = f.file.Read(p)
	duration := time.Since(start)

	f.collector.recordOperation("read", f.path, duration, int64(n), err)

	return n, err
}

// ReadAt reads data from the file at a specific offset.
func (f *MetricsFile) ReadAt(p []byte, off int64) (n int, err error) {
	start := time.Now()
	n, err = f.file.ReadAt(p, off)
	duration := time.Since(start)

	f.collector.recordOperation("read", f.path, duration, int64(n), err)

	return n, err
}

// Write writes data to the file.
func (f *MetricsFile) Write(p []byte) (n int, err error) {
	start := time.Now()
	n, err = f.file.Write(p)
	duration := time.Since(start)

	f.collector.recordOperation("write", f.path, duration, int64(n), err)

	return n, err
}

// WriteAt writes data to the file at a specific offset.
func (f *MetricsFile) WriteAt(p []byte, off int64) (n int, err error) {
	start := time.Now()
	n, err = f.file.WriteAt(p, off)
	duration := time.Since(start)

	f.collector.recordOperation("write", f.path, duration, int64(n), err)

	return n, err
}

// WriteString writes a string to the file.
func (f *MetricsFile) WriteString(s string) (n int, err error) {
	start := time.Now()
	n, err = io.WriteString(f.file, s)
	duration := time.Since(start)

	f.collector.recordOperation("write", f.path, duration, int64(n), err)

	return n, err
}

// Seek sets the file offset for the next read or write.
func (f *MetricsFile) Seek(offset int64, whence int) (int64, error) {
	start := time.Now()
	pos, err := f.file.Seek(offset, whence)
	duration := time.Since(start)

	f.collector.recordOperation("seek", f.path, duration, 0, err)

	return pos, err
}

// Close closes the file.
func (f *MetricsFile) Close() error {
	start := time.Now()
	err := f.file.Close()
	duration := time.Since(start)

	f.collector.recordOperation("close", f.path, duration, 0, err)
	f.collector.trackFileClose()

	return err
}

// Stat returns file information.
func (f *MetricsFile) Stat() (os.FileInfo, error) {
	start := time.Now()
	info, err := f.file.Stat()
	duration := time.Since(start)

	f.collector.recordOperation("stat", f.path, duration, 0, err)

	return info, err
}

// Sync commits the current contents of the file to stable storage.
func (f *MetricsFile) Sync() error {
	start := time.Now()
	err := f.file.Sync()
	duration := time.Since(start)

	f.collector.recordOperation("sync", f.path, duration, 0, err)

	return err
}

// Truncate changes the size of the file.
func (f *MetricsFile) Truncate(size int64) error {
	start := time.Now()
	err := f.file.Truncate(size)
	duration := time.Since(start)

	f.collector.recordOperation("truncate", f.path, duration, 0, err)

	return err
}

// Readdir reads directory entries.
func (f *MetricsFile) Readdir(n int) ([]os.FileInfo, error) {
	start := time.Now()
	infos, err := f.file.Readdir(n)
	duration := time.Since(start)

	f.collector.recordOperation("readdir", f.path, duration, 0, err)
	f.collector.recordDirOperation("readdir")

	return infos, err
}

// Readdirnames reads directory entry names.
func (f *MetricsFile) Readdirnames(n int) ([]string, error) {
	start := time.Now()
	names, err := f.file.Readdirnames(n)
	duration := time.Since(start)

	f.collector.recordOperation("readdir", f.path, duration, 0, err)
	f.collector.recordDirOperation("readdir")

	return names, err
}

// Name returns the name of the file.
func (f *MetricsFile) Name() string {
	return f.file.Name()
}
