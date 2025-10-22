package telemetry

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

// InitTelemetry sets up OpenTelemetry tracing
func InitTelemetry(ctx context.Context, serviceName string) (func(), error) {
	// Check if telemetry is disabled via environment variable
	if os.Getenv("DISABLE_TELEMETRY") == "true" {
		klog.Info("Telemetry is disabled via DISABLE_TELEMETRY environment variable")
		return func() {}, nil
	}

	// Create a resource that identifies your service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("environment", os.Getenv("ENVIRONMENT")),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set up the exporter
	var exporter sdktrace.SpanExporter
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	if otlpEndpoint != "" {
		// Configure OTLP exporter
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, otlpEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}

		exporter, err = otlptrace.New(
			ctx,
			otlptracegrpc.NewClient(
				otlptracegrpc.WithGRPCConn(conn),
			),
		)
		if err != nil {
			return nil, err
		}
		klog.Infof("Telemetry configured to export to OTLP endpoint: %s", otlpEndpoint)
	} else {
		// Default to stdout exporter for local development
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		klog.Info("Telemetry configured to export to stdout (no OTEL_EXPORTER_OTLP_ENDPOINT set)")
	}

	// Create trace provider with the exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Set global propagator to tracecontext (default is no-op)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	klog.Info("Telemetry initialized successfully")

	// Return a cleanup function that uses the original context
	return func() {
		// Use a shorter timeout for telemetry shutdown to avoid blocking the main shutdown
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("Error shutting down telemetry provider: %v", err)
		} else {
			klog.Info("Telemetry provider shut down successfully")
		}
	}, nil
}

// GetTracer returns a named tracer
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
