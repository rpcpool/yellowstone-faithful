# Metrics Monitoring Guide

## Overview

This document describes the metrics available in Yellowstone Faithful and how to monitor them effectively.

## RPC Metrics

### Latency Metrics

1. **Histogram Metrics** (`rpc_response_latency_histogram`)
   - Provides latency distribution for all RPC methods
   - Can calculate percentiles using PromQL:
     ```promql
     histogram_quantile(0.90, rate(rpc_response_latency_histogram_bucket[5m]))
     histogram_quantile(0.95, rate(rpc_response_latency_histogram_bucket[5m]))
     histogram_quantile(0.99, rate(rpc_response_latency_histogram_bucket[5m]))
     ```

2. **Summary Metrics** (`rpc_response_latency_summary`)
   - Pre-calculated p50, p90, p95, and p99 percentiles
   - Updated every 10 minutes with a sliding window
   - Query directly:
     ```promql
     rpc_response_latency_summary{quantile="0.9"}
     rpc_response_latency_summary{quantile="0.95"}
     rpc_response_latency_summary{quantile="0.99"}
     ```

### Request Metrics

- `rpc_requests_by_method`: Total count of requests per RPC method
- `method_to_code`: Request count by method and HTTP status code
- `method_to_success_or_failure`: Success/failure rate by method
- `method_to_num_proxied`: Count of proxied requests by method

## External Metrics

### Disk Latency (via Node Exporter)

To monitor disk latency, deploy Prometheus Node Exporter alongside Yellowstone Faithful:

```yaml
# Example prometheus scrape config
scrape_configs:
  - job_name: 'node'
    static_configs:
      - targets: ['localhost:9100']
```

Key disk metrics to monitor:
- `node_disk_read_time_seconds_total`: Total time spent reading
- `node_disk_write_time_seconds_total`: Total time spent writing
- `node_disk_io_time_seconds_total`: Total I/O time

Calculate disk latency:
```promql
rate(node_disk_read_time_seconds_total[5m]) / rate(node_disk_reads_completed_total[5m])
```

### HAProxy Metrics

If using HAProxy as a load balancer, enable the stats endpoint:

```
stats enable
stats uri /stats
stats refresh 30s
```

Or use HAProxy exporter for Prometheus:
```yaml
scrape_configs:
  - job_name: 'haproxy'
    static_configs:
      - targets: ['localhost:9101']
```

Key HAProxy metrics:
- `haproxy_backend_http_responses_total`: Response count by backend
- `haproxy_backend_response_time_average_seconds`: Average response time
- `haproxy_backend_queue_time_average_seconds`: Average queue time

## Dashboards

### Grafana Dashboard Example

Create a dashboard with these panels:

1. **RPC Latency Panel**
   ```promql
   rpc_response_latency_summary{quantile="0.99", rpc_method=~"$method"}
   ```

2. **Request Rate Panel**
   ```promql
   rate(rpc_requests_by_method[5m])
   ```

3. **Error Rate Panel**
   ```promql
   rate(method_to_success_or_failure{status="failure"}[5m])
   ```

4. **Disk I/O Panel** (requires Node Exporter)
   ```promql
   rate(node_disk_io_time_seconds_total[5m])
   ```

## Alerts

Example Prometheus alerting rules:

```yaml
groups:
  - name: yellowstone_faithful
    rules:
      - alert: HighRPCLatency
        expr: rpc_response_latency_summary{quantile="0.99"} > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High RPC latency detected"
          description: "99th percentile latency is {{ $value }}s for method {{ $labels.rpc_method }}"
      
      - alert: HighErrorRate
        expr: rate(method_to_success_or_failure{status="failure"}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} for method {{ $labels.method }}"
```

## Integration with Monitoring Stack

1. **Prometheus**: Scrape metrics from `/metrics` endpoint
2. **Grafana**: Visualize metrics with dashboards
3. **AlertManager**: Handle alerts from Prometheus
4. **OpenTelemetry Collector**: Collect traces and forward to backends

Example docker-compose setup:

```yaml
version: '3'
services:
  yellowstone-faithful:
    image: yellowstone-faithful
    ports:
      - "8899:8899"
    environment:
      - OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
  
  prometheus:
    image: prom/prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
  
  node-exporter:
    image: prom/node-exporter
    ports:
      - "9100:9100"
  
  grafana:
    image: grafana/grafana
    ports:
      - "3000:3000"
```