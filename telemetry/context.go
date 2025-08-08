package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan starts a new span and returns the context with the span
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("yellowstone-faithful")
	return tracer.Start(ctx, name, opts...)
}

// StartDiskIOSpan starts a span specifically for disk I/O operations
func StartDiskIOSpan(ctx context.Context, operation string, details map[string]string) (context.Context, trace.Span) {
	tracer := otel.Tracer("yellowstone-faithful")

	attrs := []attribute.KeyValue{
		attribute.String("operation.type", "disk_io"),
		attribute.String("disk.operation", operation),
	}

	for k, v := range details {
		attrs = append(attrs, attribute.String(k, v))
	}

	return tracer.Start(ctx, fmt.Sprintf("disk.%s", operation), trace.WithAttributes(attrs...))
}

// MeasureExecutionTime measures the execution time of a function and adds it to a span
func MeasureExecutionTime(span trace.Span, name string, fn func() error) error {
	start := time.Now()
	err := fn()
	elapsed := time.Since(start)

	span.SetAttributes(
		attribute.String("execution.step", name),
		attribute.Int64("execution.time_ms", elapsed.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// RecordError records an error in the span and sets the span status to error
func RecordError(span trace.Span, err error, message string) {
	if err != nil {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.message", message),
		))
		span.SetStatus(codes.Error, message)
	}
}
