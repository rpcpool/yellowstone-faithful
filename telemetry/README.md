# Yellowstone-Faithful Telemetry

## Overview

This package implements OpenTelemetry instrumentation for the Yellowstone-Faithful service. It provides visibility into the performance of the gRPC server and can help identify bottlenecks in the application, particularly around disk I/O operations and data processing.

## Configuration

The telemetry configuration is controlled via environment variables:

- `DISABLE_TELEMETRY`: Set to "true" to disable all telemetry collection
- `OTEL_EXPORTER_OTLP_ENDPOINT`: The endpoint URL for the OpenTelemetry collector (e.g., "localhost:4317")
- `ENVIRONMENT`: Environment name (e.g., "production", "staging", "development")

## Usage

The telemetry is automatically initialized when the gRPC server starts. No additional configuration is needed for basic usage.

### Exporting to Different Backends

By default, if no OTLP endpoint is specified, telemetry data will be printed to stdout, which is useful for local development.

To send telemetry data to different backends:

1. **Jaeger**:
   - Run a Jaeger collector with OTLP enabled
   - Set `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317`

2. **Honeycomb.io**:
   - Set `OTEL_EXPORTER_OTLP_ENDPOINT=api.honeycomb.io:443`
   - Set `OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=<your-api-key>`

3. **Prometheus** (for metrics only):
   - Configure an OpenTelemetry Collector with Prometheus exporter
   - Set `OTEL_EXPORTER_OTLP_ENDPOINT` to point to your collector

## Interpreting Results

The telemetry data can help pinpoint performance bottlenecks. Key areas to look for:

1. **Disk I/O Operations**:
   - `disk.prefetch_car`: Overall CAR file prefetching operation
   - `disk.find_cids`: Finding content IDs
   - `disk.find_offsets`: Finding offsets in files
   - `disk.read_car_section`: Reading data from disk

2. **Data Processing**:
   - `ProcessEntries`: Processing block entries
   - `ProcessEntry`: Processing individual entries
   - `ProcessTransaction`: Processing individual transactions

3. **Network and Serialization**:
   - Track latency of gRPC methods through the automatic trace data

## Key Performance Indicators

When analyzing the telemetry data, pay attention to these metrics:

1. **Seek Times**: Look for long durations in `find_cids`, `find_offsets`, and `read_car_section` spans
2. **Processing Time**: Check the duration of data transformation operations
3. **Cache Effectiveness**: Compare subsequent calls to the same resources

## Extending the Telemetry

To add instrumentation to more code:

```go
// Start a new span
ctx, span := telemetry.StartSpan(ctx, "OperationName")
defer span.End()

// Add attributes
span.SetAttributes(attribute.String("key", "value"))

// Record errors
if err != nil {
    telemetry.RecordError(span, err, "Operation failed")
}
```

For disk operations specifically:

```go
ctx, span := telemetry.StartDiskIOSpan(ctx, "read_operation", map[string]string{
    "file": "some_file.bin",
    "offset": "1024",
})
defer span.End()
```