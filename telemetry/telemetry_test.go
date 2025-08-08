package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

func TestStartSpan(t *testing.T) {
	// Basic test that span creation doesn't panic
	ctx := context.Background()
	ctx, span := telemetry.StartSpan(ctx, "TestSpan")
	span.SetAttributes(attribute.String("test", "value"))
	span.End()
}

func TestDiskIOSpan(t *testing.T) {
	// Test disk IO span creation
	ctx := context.Background()
	ctx, span := telemetry.StartDiskIOSpan(ctx, "read", map[string]string{
		"path":   "/tmp/test",
		"offset": "0",
		"size":   "1024",
	})
	span.End()
}

func TestHelpers(t *testing.T) {
	// Test TraceExecutionTime
	ctx := context.Background()
	err := telemetry.TraceExecutionTime(ctx, "SlowOperation", func() error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Test TraceFunctionExecution
	ctx, span, done := telemetry.TraceFunctionExecution(ctx, "ImportantFunction")
	time.Sleep(10 * time.Millisecond) // Simulate work
	done()

	// Test TraceFileOperation
	ctx, span = telemetry.TraceFileOperation(ctx, "read", "/path/to/file.txt")
	span.End()
}
