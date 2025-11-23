package metricsfs

import (
	"testing"

	"github.com/absfs/absfs"
)

// BenchmarkMetricsOverhead measures the overhead of metrics collection.
func BenchmarkMetricsOverhead(b *testing.B) {
	base := newMockFS()
	instrumented := New(base)

	b.Run("baseline", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			base.Stat("/test")
		}
	})

	b.Run("instrumented", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			instrumented.Stat("/test")
		}
	})
}

// BenchmarkFileOperations benchmarks common file operations.
func BenchmarkFileOperations(b *testing.B) {
	base := newMockFS()
	fs := New(base)

	b.Run("Open", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, _ := fs.Open("/test.txt")
			f.Close()
		}
	})

	b.Run("Stat", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})

	b.Run("Create", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, _ := fs.Create("/test.txt")
			f.Close()
		}
	})
}

// BenchmarkReadWrite benchmarks read and write operations.
func BenchmarkReadWrite(b *testing.B) {
	base := newMockFS()
	fs := New(base)

	data := make([]byte, 1024)
	readBuf := make([]byte, 1024)

	b.Run("Write_1KB", func(b *testing.B) {
		f, _ := fs.Create("/test.txt")
		defer f.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f.Write(data)
		}
	})

	b.Run("Read_1KB", func(b *testing.B) {
		f, _ := fs.Open("/test.txt")
		defer f.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f.Read(readBuf)
		}
	})
}

// BenchmarkConfigOptions benchmarks different configuration options.
func BenchmarkConfigOptions(b *testing.B) {
	base := newMockFS()

	b.Run("DefaultConfig", func(b *testing.B) {
		fs := New(base)
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})

	b.Run("LatencyDisabled", func(b *testing.B) {
		config := DefaultConfig()
		config.EnableLatencyMetrics = false
		fs := NewWithConfig(base, config)
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})

	b.Run("BandwidthDisabled", func(b *testing.B) {
		config := DefaultConfig()
		config.EnableBandwidthMetrics = false
		fs := NewWithConfig(base, config)
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})

	b.Run("AllMetricsDisabled", func(b *testing.B) {
		config := DefaultConfig()
		config.EnableLatencyMetrics = false
		config.EnableBandwidthMetrics = false
		fs := NewWithConfig(base, config)
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})
}

// BenchmarkConcurrentOperations benchmarks concurrent access.
func BenchmarkConcurrentOperations(b *testing.B) {
	base := newMockFS()
	fs := New(base)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f, _ := fs.Open("/test.txt")
			data := make([]byte, 100)
			f.Read(data)
			f.Close()
		}
	})
}

// BenchmarkErrorHandling benchmarks error path performance.
func BenchmarkErrorHandling(b *testing.B) {
	errorFS := &errorMockFS{}
	fs := New(errorFS)

	b.Run("NotFoundError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fs.Open("/nonexistent")
		}
	})

	b.Run("PermissionError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fs.Stat("/forbidden")
		}
	})
}

// BenchmarkFileDescriptorTracking benchmarks open file tracking.
func BenchmarkFileDescriptorTracking(b *testing.B) {
	base := newMockFS()
	fs := New(base)

	b.Run("OpenClose", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, _ := fs.Open("/test.txt")
			f.Close()
		}
	})

	b.Run("MultipleFiles", func(b *testing.B) {
		files := make([]absfs.File, 10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				files[j], _ = fs.Open("/test.txt")
			}
			for j := 0; j < 10; j++ {
				files[j].Close()
			}
		}
	})
}

// BenchmarkPathMetrics benchmarks path-level metrics with cardinality limits.
func BenchmarkPathMetrics(b *testing.B) {
	base := newMockFS()
	config := DefaultConfig()
	config.EnablePathMetrics = true
	config.MaxTrackedPaths = 100
	fs := NewWithConfig(base, config)

	b.Run("WithinLimit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			path := "/test.txt"
			fs.Stat(path)
		}
	})

	b.Run("ExceedingLimit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Each operation uses a different path
			// After 100 paths, new paths won't be tracked
			path := "/test" + string(rune(i%200))
			fs.Stat(path)
		}
	})
}

// BenchmarkCallbacks benchmarks performance with callbacks enabled.
func BenchmarkCallbacks(b *testing.B) {
	base := newMockFS()
	config := DefaultConfig()

	b.Run("NoCallbacks", func(b *testing.B) {
		fs := NewWithConfig(base, config)
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})

	b.Run("WithCallbacks", func(b *testing.B) {
		config.OnOperation = func(op Operation) {
			// Minimal callback
		}
		config.OnError = func(operation string, err error) {
			// Minimal callback
		}
		fs := NewWithConfig(base, config)
		for i := 0; i < b.N; i++ {
			fs.Stat("/test.txt")
		}
	})
}

// BenchmarkMemoryAllocation tests memory allocation overhead.
func BenchmarkMemoryAllocation(b *testing.B) {
	base := newMockFS()
	fs := New(base)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, _ := fs.Open("/test.txt")
		data := make([]byte, 1024)
		f.Read(data)
		f.Close()
	}
}
