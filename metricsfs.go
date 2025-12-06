package metricsfs

import (
	"os"
	"path/filepath"
	"time"

	"github.com/absfs/absfs"
)

// Compile-time interface compliance check
var _ absfs.FileSystem = (*MetricsFS)(nil)

// MetricsFS wraps an absfs.FileSystem and collects metrics on all operations.
type MetricsFS struct {
	fs        absfs.FileSystem
	collector *Collector
}

// New creates a new MetricsFS that wraps the given filesystem.
// It uses default configuration. For custom configuration, use NewWithConfig.
func New(fs absfs.FileSystem) *MetricsFS {
	config := DefaultConfig()
	return NewWithConfig(fs, config)
}

// NewWithConfig creates a new MetricsFS with custom configuration.
func NewWithConfig(fs absfs.FileSystem, config Config) *MetricsFS {
	return &MetricsFS{
		fs:        fs,
		collector: NewCollector(config),
	}
}

// Collector returns the Prometheus collector for this filesystem.
// Register this with prometheus.MustRegister() to expose metrics.
func (m *MetricsFS) Collector() *Collector {
	return m.collector
}

// Open opens a file for reading.
func (m *MetricsFS) Open(name string) (absfs.File, error) {
	start := time.Now()
	f, err := m.fs.Open(name)
	duration := time.Since(start)

	m.collector.recordOperation("open", name, duration, 0, err)
	m.collector.recordFileOpen("read")

	if err != nil {
		return nil, err
	}

	return newMetricsFile(f, m.collector, name), nil
}

// OpenFile opens a file with the specified flags and mode.
func (m *MetricsFS) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	start := time.Now()
	f, err := m.fs.OpenFile(name, flag, perm)
	duration := time.Since(start)

	// Determine mode
	mode := "read"
	if flag&os.O_WRONLY != 0 {
		mode = "write"
	} else if flag&os.O_RDWR != 0 {
		mode = "readwrite"
	}
	if flag&os.O_APPEND != 0 {
		mode = "append"
	}

	m.collector.recordOperation("open", name, duration, 0, err)
	m.collector.recordFileOpen(mode)

	if err != nil {
		return nil, err
	}

	return newMetricsFile(f, m.collector, name), nil
}

// Create creates a new file.
func (m *MetricsFS) Create(name string) (absfs.File, error) {
	start := time.Now()
	f, err := m.fs.Create(name)
	duration := time.Since(start)

	m.collector.recordOperation("create", name, duration, 0, err)
	m.collector.recordFileCreate()
	m.collector.recordFileOpen("write")

	if err != nil {
		return nil, err
	}

	return newMetricsFile(f, m.collector, name), nil
}

// Mkdir creates a directory.
func (m *MetricsFS) Mkdir(name string, perm os.FileMode) error {
	start := time.Now()
	err := m.fs.Mkdir(name, perm)
	duration := time.Since(start)

	m.collector.recordOperation("mkdir", name, duration, 0, err)
	m.collector.recordDirOperation("mkdir")

	return err
}

// MkdirAll creates a directory and all necessary parent directories.
func (m *MetricsFS) MkdirAll(name string, perm os.FileMode) error {
	start := time.Now()
	err := m.fs.MkdirAll(name, perm)
	duration := time.Since(start)

	m.collector.recordOperation("mkdirall", name, duration, 0, err)
	m.collector.recordDirOperation("mkdirall")

	return err
}

// Remove removes a file or directory.
func (m *MetricsFS) Remove(name string) error {
	start := time.Now()
	err := m.fs.Remove(name)
	duration := time.Since(start)

	m.collector.recordOperation("remove", name, duration, 0, err)
	m.collector.recordDirOperation("remove")

	return err
}

// RemoveAll removes a path and all children.
func (m *MetricsFS) RemoveAll(name string) error {
	start := time.Now()
	err := m.fs.RemoveAll(name)
	duration := time.Since(start)

	m.collector.recordOperation("removeall", name, duration, 0, err)
	m.collector.recordDirOperation("removeall")

	return err
}

// Rename renames a file or directory.
func (m *MetricsFS) Rename(oldpath, newpath string) error {
	start := time.Now()
	err := m.fs.Rename(oldpath, newpath)
	duration := time.Since(start)

	m.collector.recordOperation("rename", oldpath, duration, 0, err)

	return err
}

// Stat returns file information.
func (m *MetricsFS) Stat(name string) (os.FileInfo, error) {
	start := time.Now()
	info, err := m.fs.Stat(name)
	duration := time.Since(start)

	m.collector.recordOperation("stat", name, duration, 0, err)

	return info, err
}

// Lstat returns file information without following symlinks.
// This method is only available if the underlying filesystem implements SymlinkFileSystem.
func (m *MetricsFS) Lstat(name string) (os.FileInfo, error) {
	start := time.Now()

	// Check if underlying filesystem supports Lstat
	if sfs, ok := m.fs.(interface {
		Lstat(name string) (os.FileInfo, error)
	}); ok {
		info, err := sfs.Lstat(name)
		duration := time.Since(start)
		m.collector.recordOperation("lstat", name, duration, 0, err)
		return info, err
	}

	// Fallback to Stat if Lstat not available
	return m.Stat(name)
}

// Chmod changes file permissions.
func (m *MetricsFS) Chmod(name string, mode os.FileMode) error {
	start := time.Now()
	err := m.fs.Chmod(name, mode)
	duration := time.Since(start)

	m.collector.recordOperation("chmod", name, duration, 0, err)

	return err
}

// Chown changes file ownership.
func (m *MetricsFS) Chown(name string, uid, gid int) error {
	start := time.Now()
	err := m.fs.Chown(name, uid, gid)
	duration := time.Since(start)

	m.collector.recordOperation("chown", name, duration, 0, err)

	return err
}

// Chtimes changes file access and modification times.
func (m *MetricsFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	start := time.Now()
	err := m.fs.Chtimes(name, atime, mtime)
	duration := time.Since(start)

	m.collector.recordOperation("chtimes", name, duration, 0, err)

	return err
}

// Readlink reads the target of a symbolic link.
// This method is only available if the underlying filesystem implements SymlinkFileSystem.
func (m *MetricsFS) Readlink(name string) (string, error) {
	start := time.Now()

	// Check if underlying filesystem supports Readlink
	if sfs, ok := m.fs.(interface {
		Readlink(name string) (string, error)
	}); ok {
		target, err := sfs.Readlink(name)
		duration := time.Since(start)
		m.collector.recordOperation("readlink", name, duration, 0, err)
		return target, err
	}

	duration := time.Since(start)
	err := os.ErrInvalid
	m.collector.recordOperation("readlink", name, duration, 0, err)
	return "", err
}

// Symlink creates a symbolic link.
// This method is only available if the underlying filesystem implements SymlinkFileSystem.
func (m *MetricsFS) Symlink(oldname, newname string) error {
	start := time.Now()

	// Check if underlying filesystem supports Symlink
	if sfs, ok := m.fs.(interface {
		Symlink(oldname, newname string) error
	}); ok {
		err := sfs.Symlink(oldname, newname)
		duration := time.Since(start)
		m.collector.recordOperation("symlink", newname, duration, 0, err)
		return err
	}

	duration := time.Since(start)
	err := os.ErrInvalid
	m.collector.recordOperation("symlink", newname, duration, 0, err)
	return err
}

// Separator returns the OS-specific path separator character.
func (m *MetricsFS) Separator() uint8 {
	// Check if underlying filesystem implements Separator
	if fs, ok := m.fs.(interface {
		Separator() uint8
	}); ok {
		return fs.Separator()
	}
	return filepath.Separator
}

// ListSeparator returns the OS-specific path list separator character.
func (m *MetricsFS) ListSeparator() uint8 {
	// Check if underlying filesystem implements ListSeparator
	if fs, ok := m.fs.(interface {
		ListSeparator() uint8
	}); ok {
		return fs.ListSeparator()
	}
	return filepath.ListSeparator
}

// Chdir changes the current working directory.
func (m *MetricsFS) Chdir(dir string) error {
	start := time.Now()

	// Check if underlying filesystem implements Chdir
	if fs, ok := m.fs.(interface {
		Chdir(dir string) error
	}); ok {
		err := fs.Chdir(dir)
		duration := time.Since(start)
		m.collector.recordOperation("chdir", dir, duration, 0, err)
		return err
	}

	duration := time.Since(start)
	err := os.ErrInvalid
	m.collector.recordOperation("chdir", dir, duration, 0, err)
	return err
}

// Getwd returns the current working directory.
func (m *MetricsFS) Getwd() (string, error) {
	start := time.Now()

	// Check if underlying filesystem implements Getwd
	if fs, ok := m.fs.(interface {
		Getwd() (string, error)
	}); ok {
		dir, err := fs.Getwd()
		duration := time.Since(start)
		m.collector.recordOperation("getwd", dir, duration, 0, err)
		return dir, err
	}

	duration := time.Since(start)
	err := os.ErrInvalid
	m.collector.recordOperation("getwd", "", duration, 0, err)
	return "", err
}

// TempDir returns the path to the temporary directory.
func (m *MetricsFS) TempDir() string {
	// Check if underlying filesystem implements TempDir
	if fs, ok := m.fs.(interface {
		TempDir() string
	}); ok {
		return fs.TempDir()
	}
	return os.TempDir()
}

// Truncate truncates the named file to the specified size.
func (m *MetricsFS) Truncate(name string, size int64) error {
	start := time.Now()

	// Check if underlying filesystem implements Truncate
	if fs, ok := m.fs.(interface {
		Truncate(name string, size int64) error
	}); ok {
		err := fs.Truncate(name, size)
		duration := time.Since(start)
		m.collector.recordOperation("truncate", name, duration, size, err)
		return err
	}

	duration := time.Since(start)
	err := os.ErrInvalid
	m.collector.recordOperation("truncate", name, duration, size, err)
	return err
}
