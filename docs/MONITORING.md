# Monitoring Guide

The OpenBAO Teradata Secret Plugin exposes Prometheus metrics for monitoring connection pools, credential operations, cache performance, and more.

## Quick Start

### 1. Enable Metrics Endpoint

The plugin exposes a `/metrics` endpoint at `/v1/teradata/metrics`. This endpoint returns Prometheus-formatted metrics.

### 2. Configure Prometheus

Copy the provided `prometheus.yml` to your Prometheus configuration directory:

```bash
cp prometheus.yml /etc/prometheus/prometheus.yml
```

Update the following in your Prometheus configuration:
- `targets`: Your OpenBAO server address
- `bearer_token_file`: Path to your OpenBao token
- TLS certificates if using TLS

Start Prometheus:

```bash
prometheus --config.file=/etc/prometheus/prometheus.yml
```

### 3. Import Grafana Dashboard

1. Open Grafana
2. Navigate to Dashboards → Import
3. Upload `dashboards/grafana/teradata-plugin-overview.json`
4. Select your Prometheus datasource
5. Click Import

## Available Metrics

### Connection Pool Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `teradata_pool_open_connections` | Gauge | Number of open connections |
| `teradata_pool_idle_connections` | Gauge | Number of idle connections |
| `teradata_pool_active_connections` | Gauge | Number of active (in-use) connections |
| `teradata_pool_health_check_duration_seconds` | Histogram | Health check duration |
| `teradata_pool_health_check_total` | Counter | Total health checks performed |
| `teradata_pool_connection_errors_total` | Counter | Connection errors |
| `teradata_pool_idle_closed_total` | Counter | Idle connections closed |

### Credential Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `teradata_credentials_created_total` | Counter | Credentials created |
| `teradata_credentials_creation_duration_seconds` | Histogram | Creation duration |
| `teradata_credentials_creation_errors_total` | Counter | Creation errors |
| `teradata_credentials_revoked_total` | Counter | Credentials revoked |
| `teradata_credentials_expired_total` | Counter | Credentials expired |

### Cache Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `teradata_cache_hits_total` | Counter | Cache hits |
| `teradata_cache_misses_total` | Counter | Cache misses |
| `teradata_cache_evictions_total` | Counter | Cache evictions |

### Rate Limiting Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `teradata_rate_limit_allowed_total` | Counter | Requests allowed |
| `teradata_rate_limit_rejected_total` | Counter | Requests rejected |

### SQL Execution Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `teradata_sql_execution_duration_seconds` | Histogram | SQL execution duration |
| `teradata_sql_execution_errors_total` | Counter | SQL execution errors |

## Grafana Dashboard Panels

The provided dashboard includes:

- **Connection Pool Overview**: Open, idle, and active connections over time
- **Health Check Latency**: p50, p95, p99 latency for health checks
- **Credential Operations**: Creation, revocation, and expiration rates
- **Credential Creation Latency**: Time to create credentials
- **Cache Performance**: Hit/miss rates and hit ratio gauge
- **Rate Limiting**: Allowed vs rejected requests
- **SQL Execution Latency**: Query execution time percentiles
- **Error Rates**: Connection, SQL, and credential creation errors
- **Connection Pool Health**: Idle closures and health check results

## Prometheus Alerts

Pre-configured alerting rules are available in `dashboards/prometheus/alerts/teradata-plugin.yml`:

| Alert | Severity | Description |
|-------|----------|-------------|
| `TeradataPluginDown` | Critical | Plugin unreachable |
| `HighConnectionPoolUsage` | Warning | Pool >80% capacity |
| `ConnectionPoolExhausted` | Critical | All connections in use |
| `HighCredentialCreationLatency` | Warning | p95 creation >5s |
| `HighCredentialCreationErrorRate` | Critical | Error rate >0.1/s |
| `HighConnectionErrorRate` | Critical | Error rate >0.05/s |
| `LowCacheHitRate` | Warning | Hit rate <60% |
| `HighRateLimitRejections` | Warning | Rejections >10/s |
| `UnhealthyPool` | Critical | More failures than successes |
| `HighSQLExecutionLatency` | Warning | p99 SQL >10s |

## Useful PromQL Queries

### Connection Pool Utilization
```promql
100 * teradata_pool_active_connections / teradata_pool_open_connections
```

### Credential Creation Success Rate
```promql
100 * rate(teradata_credentials_created_total[5m]) /
    (rate(teradata_credentials_created_total[5m]) + rate(teradata_credentials_creation_errors_total[5m]))
```

### Cache Hit Ratio
```promql
100 * sum(rate(teradata_cache_hits_total[5m])) /
    (sum(rate(teradata_cache_hits_total[5m])) + sum(rate(teradata_cache_misses_total[5m])))
```

### Request Success Rate
```promql
100 * rate(teradata_rate_limit_allowed_total[5m]) /
    (rate(teradata_rate_limit_allowed_total[5m]) + rate(teradata_rate_limit_rejected_total[5m]))
```

## Additional Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/teradata/metrics` | GET | Prometheus metrics |
| `/v1/teradata/pool-stats` | GET | Detailed pool statistics |
| `/v1/teradata/health` | GET | Health check |
| `/v1/teradata/readiness` | GET | Kubernetes readiness probe |
| `/v1/teradata/liveness` | GET | Kubernetes liveness probe |
| `/v1/teradata/info` | GET | Database/driver information |

## Kubernetes Deployment

For Kubernetes deployments, add these annotations to your OpenBAO pod:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8200"
  prometheus.io/path: "/v1/teradata/metrics"
```
