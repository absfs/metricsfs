package main

import (
	"log"
	"net/http"

	"github.com/absfs/metricsfs"
	"github.com/absfs/osfs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Create base filesystem
	base := osfs.NewFS()

	// Wrap with metrics, using custom configuration
	fs := metricsfs.NewWithConfig(base, metricsfs.Config{
		Namespace:              "fileserver",
		Subsystem:              "storage",
		EnableLatencyMetrics:   true,
		EnableBandwidthMetrics: true,
		ConstLabels: prometheus.Labels{
			"service": "http-fileserver",
			"env":     "development",
		},
	})

	// Register metrics
	prometheus.MustRegister(fs.Collector())

	// Create HTTP handlers
	mux := http.NewServeMux()

	// File server handler (serves files from current directory)
	fileHandler := http.StripPrefix("/files/",
		http.FileServer(http.Dir(".")))
	mux.Handle("/files/", fileHandler)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Root handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<head><title>Monitored File Server</title></head>
			<body>
				<h1>Monitored File Server</h1>
				<p>This file server is instrumented with metricsfs.</p>
				<ul>
					<li><a href="/files/">Browse Files</a></li>
					<li><a href="/metrics">View Metrics</a></li>
					<li><a href="/health">Health Check</a></li>
				</ul>
				<h2>Metrics Collected</h2>
				<ul>
					<li>Operation counts (read, write, stat, etc.)</li>
					<li>Operation latencies</li>
					<li>Bytes read/written</li>
					<li>Error rates</li>
					<li>Open file descriptors</li>
				</ul>
			</body>
			</html>
		`))
	})

	log.Println("Starting monitored file server on :8080")
	log.Println("Browse files: http://localhost:8080/files/")
	log.Println("View metrics: http://localhost:8080/metrics")
	log.Println("Health check: http://localhost:8080/health")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
