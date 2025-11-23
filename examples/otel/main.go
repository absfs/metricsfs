package main

import (
	"context"
	"log"
	"os"

	"github.com/absfs/metricsfs"
	"github.com/absfs/osfs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func main() {
	// Set up OpenTelemetry resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("metricsfs-example"),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	// Set up trace exporter
	traceExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		log.Fatalf("Failed to create trace exporter: %v", err)
	}

	// Set up trace provider
	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(traceExporter),
	)
	defer tracerProvider.Shutdown(context.Background())
	otel.SetTracerProvider(tracerProvider)

	// Set up metric exporter
	metricExporter, err := stdoutmetric.New(
		stdoutmetric.WithPrettyPrint(),
	)
	if err != nil {
		log.Fatalf("Failed to create metric exporter: %v", err)
	}

	// Set up meter provider
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	defer meterProvider.Shutdown(context.Background())
	otel.SetMeterProvider(meterProvider)

	// Create base filesystem
	base := osfs.NewFS()

	// Wrap with OpenTelemetry metrics
	fs, err := metricsfs.NewWithOTel(base, metricsfs.OTelConfig{
		MeterProvider:  meterProvider,
		TracerProvider: tracerProvider,
		EnableTracing:  true,
		ConstAttributes: []attribute.KeyValue{
			attribute.String("service.name", "filesystem-service"),
			attribute.String("deployment.environment", "development"),
		},
	})
	if err != nil {
		log.Fatalf("Failed to create metricsfs: %v", err)
	}

	log.Println("OpenTelemetry-instrumented filesystem operations:")

	// Perform operations with context
	ctx := context.Background()

	// Create a file
	f, err := fs.OpenWithContext(ctx, "/tmp/otel-test.txt")
	if err != nil {
		// File doesn't exist, create it
		log.Println("Creating test file...")
		file, err := os.Create("/tmp/otel-test.txt")
		if err != nil {
			log.Fatalf("Failed to create file: %v", err)
		}
		file.WriteString("OpenTelemetry test data")
		file.Close()

		f, err = fs.OpenWithContext(ctx, "/tmp/otel-test.txt")
		if err != nil {
			log.Fatalf("Failed to open file: %v", err)
		}
	}

	// Read data
	data := make([]byte, 100)
	n, _ := f.Read(data)
	log.Printf("Read %d bytes: %s", n, string(data[:n]))

	// Close file
	f.Close()

	// Get file stats
	info, err := fs.StatWithContext(ctx, "/tmp/otel-test.txt")
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}
	log.Printf("File size: %d bytes", info.Size())

	// Clean up
	os.Remove("/tmp/otel-test.txt")

	log.Println("All operations completed!")
	log.Println("Traces and metrics have been exported to stdout")
}
