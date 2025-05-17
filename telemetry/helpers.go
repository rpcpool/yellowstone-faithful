package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TraceExecutionTime measures the execution time of a function and records it in the span
func TraceExecutionTime(ctx context.Context, name string, fn func() error) error {
	ctx, span := StartSpan(ctx, name)
	defer span.End()

	start := time.Now()
	err := fn()
	elapsed := time.Since(start)

	span.SetAttributes(
		attribute.Int64("execution_time_ms", elapsed.Milliseconds()),
	)

	if err != nil {
		RecordError(span, err, "Operation failed")
	}

	return err
}

// TraceFunctionExecution is a simple helper to trace the execution of a function
func TraceFunctionExecution(ctx context.Context, name string) (context.Context, trace.Span, func()) {
	ctx, span := StartSpan(ctx, name)
	start := time.Now()
	
	return ctx, span, func() {
		elapsed := time.Since(start)
		span.SetAttributes(attribute.Int64("execution_time_ms", elapsed.Milliseconds()))
		span.End()
	}
}

// TraceFileOperation helps trace file operations with common attributes
func TraceFileOperation(ctx context.Context, operation string, path string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "FileOperation."+operation)
	span.SetAttributes(
		attribute.String("file.path", path),
		attribute.String("file.operation", operation),
	)
	return ctx, span
}