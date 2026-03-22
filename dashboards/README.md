# Monitoring Dashboards

This directory contains pre-configured dashboards for monitoring the OpenBao Teradata Secret Plugin.

## Contents

### Grafana Dashboard

**File:** `grafana/teradata-plugin-dashboard.json`

A comprehensive Grafana dashboard covering:

- **Connection Pool Overview**
  - Total open/active/idle connections
  - Connection errors
  - Pool dynamics over time
  - Health check duration percentiles

- **Credentials Management**
  - Credentials created/revoked/expired totals
  - Credential creation rate by role
  - Credential creation duration percentiles

- **Cache Performance**
  - Cache hits/misses/evictions
  - Cache hit rate
  - Cache performance by type

- **Rate Limiting**
  - Requests allowed/rejected
  - Rate limit success rate
  - Rate limiting trends

- **SQL Execution**
  - SQL execution duration percentiles
  - SQL execution errors by type

### Prometheus Configuration

**File:** `prometheus/prometheus.yml`

Pre-configured Prometheus scrape configuration for:

- Main plugin metrics (`/v1/teradata/metrics`)
- Pool statistics (`/v1/teradata/pool-stats`)
- Health checks (`/v1/teradata/health`)

## Importing the Grafana Dashboard

1. Open Grafana
2. Navigate to Dashboards → Import
3. Upload `grafana/teradata-plugin-dashboard.json`
4. Select your Prometheus data source
5. Click Import

## Using the Prometheus Configuration

### Standalone Prometheus

```bash
prometheus --config.file=prometheus/prometheus.yml
```

### Docker

```bash
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v $(pwd)/dashboards/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

### Kubernetes

```bash
kubectl apply -f dashboards/prometheus/prometheus.yml
```

## Variables

The Prometheus configuration supports the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PLUGIN_HOST` | `localhost` | Plugin host |
| `PLUGIN_PORT` | `8200` | Plugin port |
| `ENVIRONMENT` | `production` | Environment label |
| `REMOTE_WRITE_URL` | empty | Remote write endpoint (optional) |

## Metrics Reference

For a complete list of exposed metrics, see [metrics/metrics.go](../metrics/metrics.go).