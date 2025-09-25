# WhatsSignal Metrics and Observability

## Overview

WhatsSignal includes a comprehensive built-in metrics and observability system that provides operational insights without requiring external dependencies. The system tracks performance metrics, request patterns, and operational health through counters, timers, and gauges.

## Key Features

- **Lightweight**: Embedded metrics with no external dependencies
- **Privacy-First**: Automatic masking of sensitive data in logs
- **Request Tracing**: Correlation IDs for debugging across operations
- **Real-Time Metrics**: JSON endpoint for monitoring integration
- **Automatic Instrumentation**: HTTP middleware captures metrics automatically

## Accessing Metrics

### Metrics Endpoint

Access real-time metrics in JSON format:

```bash
curl http://localhost:8082/metrics
```

### Example Response

```json
{
  "counters": {
    "http_requests_total_method:GET_endpoint:/health": {
      "name": "http_requests_total",
      "type": "counter",
      "value": 42,
      "labels": {"method": "GET", "endpoint": "/health"},
      "description": "Total HTTP requests",
      "last_update": "2025-09-25T10:30:00Z"
    },
    "webhook_success_total_type:whatsapp": {
      "name": "webhook_success_total",
      "type": "counter", 
      "value": 1523,
      "labels": {"type": "whatsapp"},
      "description": "Successful webhook processing"
    }
  },
  "timers": {
    "http_request_duration_method:POST_endpoint:/webhook/whatsapp_status_code:200": {
      "count": 1523,
      "sum_ms": 45690.5,
      "min_ms": 10.2,
      "max_ms": 250.8,
      "avg_ms": 30.0,
      "p95_ms": 75.5,
      "p99_ms": 120.3
    }
  },
  "gauges": {},
  "uptime_ms": 3600000,
  "timestamp": 1695634800
}
```

## Metrics Categories

### HTTP Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `http_requests_total` | Counter | Total HTTP requests | method, endpoint |
| `http_requests_active` | Counter | Currently active requests | - |
| `http_responses_total` | Counter | HTTP responses by status | method, endpoint, status_code |
| `http_request_duration` | Timer | Request processing time | method, endpoint, status_code |
| `http_response_size` | Timer | Response size in bytes | method, endpoint |

### Webhook Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `webhook_requests_total` | Counter | Total webhook requests | type |
| `webhook_success_total` | Counter | Successful webhook processing | type |
| `webhook_errors_total` | Counter | Failed webhook processing | type, status_code |
| `webhook_processing_duration` | Timer | Webhook processing time | type, status_code |

### Signal Polling Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `signal_poll_attempts_total` | Counter | Total polling attempts | - |
| `signal_poll_success_total` | Counter | Successful polls | - |
| `signal_poll_failures_total` | Counter | Failed polls (all retries exhausted) | - |
| `signal_poll_attempt_failures_total` | Counter | Individual attempt failures | attempt |
| `signal_poll_attempt_duration` | Timer | Duration per attempt | attempt |
| `signal_poll_total_duration` | Timer | Total operation duration | status |

### Message Processing Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `message_processing_total` | Counter | Total messages processed | direction, session, has_media |
| `message_processing_success` | Counter | Successfully processed messages | direction, session, has_media |
| `message_processing_failures` | Counter | Failed message processing | direction, session, stage |
| `message_processing_duration` | Timer | Message processing time | direction, session |

## Request Tracing

Every HTTP request receives unique tracing identifiers:

- **Request ID**: Unique identifier for each request (`req_xxxxx`)
- **Trace ID**: Correlation ID for distributed tracing (32 hex chars)
- **Span ID**: Individual operation identifier (16 hex chars)

These IDs appear in all related log entries:

```json
{
  "level": "info",
  "msg": "Processing WhatsApp message",
  "request_id": "req_a1b2c3d4",
  "trace_id": "f47ac10b58d4e1a898c5b9e884961234",
  "session": "primary-*******-****123",
  "chat_id": "******7890@c.us",
  "message_id": "true_******7890@c.us_*****3XYZ",
  "platform": "whatsapp",
  "direction": "incoming"
}
```

## Privacy Protection

### Automatic Data Masking

Sensitive data is automatically masked in logs:

- **Phone Numbers**: `+1234567890` → `+******7890`
- **Chat IDs**: `1234567890@c.us` → `******7890@c.us`
- **Message IDs**: `true_1234567890@c.us_ABC123` → `true_******7890@c.us_***123`
- **Session Names**: `primary-session-user123` → `primary-*******-****123`

### Masking Rules

- Phone numbers show only last 4 digits
- Chat IDs preserve structure (@c.us, @g.us)
- Message IDs maintain format for debugging
- Session names mask middle segments

## Structured Logging

### Standard Field Names

Use these field names for consistent logging:

```go
// Core identifiers
LogFieldSession   = "session"
LogFieldMessageID = "message_id"
LogFieldChatID    = "chat_id"
LogFieldRequestID = "request_id"
LogFieldTraceID   = "trace_id"

// Performance metrics
LogFieldDuration = "duration_ms"
LogFieldSize     = "size_bytes"

// Network fields
LogFieldRemoteIP   = "remote_ip"
LogFieldStatusCode = "status_code"
LogFieldMethod     = "method"
```

### Log Levels

- **DEBUG**: Detailed diagnostic information (verbose mode only)
- **INFO**: Normal operational events
- **WARN**: Recoverable issues, fallback behavior
- **ERROR**: Failed operations requiring attention
- **FATAL**: Critical failures causing shutdown

## Integration Examples

### Prometheus Integration

Convert JSON metrics to Prometheus format:

```python
import requests
import time

def scrape_metrics():
    response = requests.get("http://localhost:8082/metrics")
    metrics = response.json()
    
    output = []
    
    # Convert counters
    for key, counter in metrics.get("counters", {}).items():
        labels = ",".join([f'{k}="{v}"' for k, v in counter.get("labels", {}).items()])
        if labels:
            output.append(f'{counter["name"]}{{{labels}}} {counter["value"]}')
        else:
            output.append(f'{counter["name"]} {counter["value"]}')
    
    # Convert timers
    for key, timer in metrics.get("timers", {}).items():
        base_name = key.split("_")[0] + "_" + key.split("_")[1]
        output.append(f'{base_name}_count {timer["count"]}')
        output.append(f'{base_name}_sum {timer["sum_ms"]}')
        
        if "p95_ms" in timer:
            output.append(f'{base_name}_p95 {timer["p95_ms"]}')
        if "p99_ms" in timer:
            output.append(f'{base_name}_p99 {timer["p99_ms"]}')
    
    return "\n".join(output)
```

### Grafana Dashboard Query Examples

```sql
-- Request rate over time
SELECT 
  derivative(value) as requests_per_second
FROM http_requests_total
WHERE time > now() - 1h
GROUP BY endpoint

-- Average response time
SELECT 
  avg(avg_ms) as avg_response_time
FROM http_request_duration
WHERE time > now() - 1h
GROUP BY endpoint

-- Error rate
SELECT 
  sum(value) as errors
FROM http_responses_total
WHERE status_code >= 400
  AND time > now() - 1h
GROUP BY endpoint
```

### Health Monitoring Script

```bash
#!/bin/bash

# Monitor WhatsSignal health via metrics
while true; do
  metrics=$(curl -s http://localhost:8082/metrics)
  
  # Extract key metrics
  uptime=$(echo "$metrics" | jq '.uptime_ms / 1000 / 60' | cut -d. -f1)
  requests=$(echo "$metrics" | jq '[.counters[].value] | add')
  errors=$(echo "$metrics" | jq '[.counters | to_entries[] | select(.key | contains("error")) | .value.value] | add // 0')
  
  echo "$(date): Uptime: ${uptime}m | Requests: ${requests} | Errors: ${errors}"
  
  # Alert if error rate is high
  if [ "$errors" -gt 100 ]; then
    echo "ALERT: High error rate detected!"
  fi
  
  sleep 60
done
```

## Performance Tips

### Metric Collection Overhead

The metrics system has minimal overhead:
- Counters: ~10ns per increment
- Timers: ~100ns per recording
- Memory: ~1KB per unique metric

### Best Practices

1. **Use Labels Sparingly**: Each unique label combination creates a new metric
2. **Aggregate at Query Time**: Don't create metrics for every user/chat
3. **Set Reasonable Retention**: Metrics are in-memory only
4. **Monitor Key Paths**: Focus on critical user journeys

## Troubleshooting

### Missing Metrics

If metrics are not appearing:

1. Verify the server is running: `curl http://localhost:8082/health`
2. Check that operations are occurring (messages being processed)
3. Ensure middleware is applied to routes in `server.go`

### High Memory Usage

If memory usage is high:

1. Check for high-cardinality labels (too many unique values)
2. Review timer sample retention (capped at 1000 samples)
3. Consider restarting to clear accumulated metrics

### Debugging with Trace IDs

To trace a specific request:

```bash
# Find request in logs
grep "req_a1b2c3d4" /var/log/whatsignal.log

# Find all related operations
grep "trace_id=f47ac10b58d4e1a898c5b9e884961234" /var/log/whatsignal.log
```

## Configuration

Currently, the metrics system requires no configuration and starts automatically. Future versions may support:

- Configurable retention periods
- Export to external systems
- Custom metric definitions
- Adjustable sampling rates

## Security Considerations

- **No PII in Metrics**: All personal data is excluded from metrics
- **Masked Logging**: Sensitive fields automatically masked
- **Local Only**: Metrics endpoint should not be exposed publicly
- **Access Control**: Use reverse proxy to restrict `/metrics` access

## Example: Monitoring Message Flow

Track message flow from WhatsApp to Signal:

```bash
# Get current message processing stats
curl -s http://localhost:8082/metrics | jq '
  .counters | to_entries[] | 
  select(.key | contains("message_processing")) | 
  {
    metric: .value.name,
    labels: .value.labels,
    count: .value.value
  }
'

# Calculate success rate
curl -s http://localhost:8082/metrics | jq '
  (.counters["message_processing_success_direction:whatsapp_to_signal"].value // 0) /
  (.counters["message_processing_total_direction:whatsapp_to_signal"].value // 1) * 100
  | "Success rate: \(.)%"
'

# Get processing time statistics
curl -s http://localhost:8082/metrics | jq '
  .timers | to_entries[] |
  select(.key | contains("message_processing_duration")) |
  {
    direction: .key | split("_")[3],
    avg_ms: .value.avg_ms,
    p95_ms: .value.p95_ms,
    p99_ms: .value.p99_ms
  }
'
```

## Summary

WhatsSignal's metrics system provides comprehensive operational visibility while maintaining user privacy. The lightweight, embedded design requires no external dependencies while offering the data needed for effective monitoring and debugging of your WhatsApp-Signal bridge deployment.