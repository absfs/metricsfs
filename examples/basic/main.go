package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/absfs/absfs"
	"github.com/absfs/metricsfs"
	"github.com/absfs/osfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Create base filesystem (using OS filesystem)
	base := osfs.NewFS()

	// Wrap with metrics collection
	fs := metricsfs.New(base)

	// Register metrics with Prometheus
	prometheus.MustRegister(fs.Collector())

	// Start HTTP server to expose metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Println("Metrics server started on :9090")
		log.Println("Visit http://localhost:9090/metrics to see metrics")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	// Perform some filesystem operations
	log.Println("Performing filesystem operations...")

	// Create a test file
	f, err := fs.Create("/tmp/test.txt")
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}

	// Write some data
	data := []byte("Hello, metricsfs!")
	n, err := f.Write(data)
	if err != nil {
		log.Fatalf("Failed to write data: %v", err)
	}
	log.Printf("Wrote %d bytes", n)

	// Close the file
	if err := f.Close(); err != nil {
		log.Fatalf("Failed to close file: %v", err)
	}

	// Read the file back
	f, err = fs.Open("/tmp/test.txt")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	readBuf := make([]byte, 100)
	n, err = f.Read(readBuf)
	if err != nil && err != absfs.ErrEOF {
		log.Fatalf("Failed to read file: %v", err)
	}
	log.Printf("Read %d bytes: %s", n, string(readBuf[:n]))

	// Get file info
	info, err := fs.Stat("/tmp/test.txt")
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}
	log.Printf("File size: %d bytes", info.Size())

	// Create a directory
	if err := fs.MkdirAll("/tmp/testdir/subdir", 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}
	log.Println("Created directory: /tmp/testdir/subdir")

	// Remove the test file and directory
	if err := fs.Remove("/tmp/test.txt"); err != nil {
		log.Fatalf("Failed to remove file: %v", err)
	}

	if err := fs.RemoveAll("/tmp/testdir"); err != nil {
		log.Fatalf("Failed to remove directory: %v", err)
	}

	log.Println("All operations completed successfully!")
	log.Println("Metrics are being collected. Check http://localhost:9090/metrics")

	// Keep the server running
	select {}
}
