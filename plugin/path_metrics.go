package teradata

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

func init() {
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
}

func (b *Backend) pathMetrics() *framework.Path {
	return &framework.Path{
		Pattern:         "metrics",
		HelpSynopsis:    "Prometheus metrics",
		HelpDescription: "Returns Prometheus metrics for monitoring the Teradata plugin.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathMetricsRead,
			},
		},
	}
}

func (b *Backend) pathMetricsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.collectPoolMetrics()

	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	recorder := newWriterResponse()
	httpReq, _ := http.NewRequest("GET", "/metrics", nil)
	httpReq = httpReq.WithContext(ctx)
	handler.ServeHTTP(recorder, httpReq)

	return &logical.Response{
		Data: map[string]interface{}{
			"format":  "prometheus",
			"content": recorder.Writer.String(),
		},
	}, nil
}

func (b *Backend) collectPoolMetrics() {
	dbRegistry := b.getDBRegistry()
	if dbRegistry != nil {
		dbRegistry.UpdatePoolMetrics()
	}
}

type writerResponse struct {
	Writer  *strings.Builder
	headers http.Header
	status  int
}

func newWriterResponse() *writerResponse {
	return &writerResponse{
		Writer:  new(strings.Builder),
		headers: make(http.Header),
		status:  200,
	}
}

func (w *writerResponse) Header() http.Header {
	return w.headers
}

func (w *writerResponse) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *writerResponse) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

type PoolMetrics struct {
	Name                string        `json:"name"`
	OpenConnections     int           `json:"open_connections"`
	IdleConnections     int           `json:"idle_connections"`
	ActiveConnections   int           `json:"active_connections"`
	State               string        `json:"state"`
	LastHealthCheck     time.Time     `json:"last_health_check"`
	HealthCheckErrors   int           `json:"health_check_errors"`
	InUseConnections    int           `json:"in_use_connections"`
	WaitQueueLength     int           `json:"wait_queue_length"`
	MaxOpenConnections  int           `json:"max_open_connections"`
	MinConnections      int           `json:"min_connections"`
	ConnectionTimeout   time.Duration `json:"connection_timeout"`
	IdleTimeout         time.Duration `json:"idle_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

type MetricsResponse struct {
	Timestamp   time.Time      `json:"timestamp"`
	PoolMetrics []PoolMetrics  `json:"pool_metrics"`
	Summary     MetricsSummary `json:"summary"`
}

type MetricsSummary struct {
	TotalPools        int `json:"total_pools"`
	TotalOpenConns    int `json:"total_open_connections"`
	TotalIdleConns    int `json:"total_idle_connections"`
	TotalActiveConns  int `json:"total_active_connections"`
	HealthyPools      int `json:"healthy_pools"`
	UnhealthyPools    int `json:"unhealthy_pools"`
	TotalHealthErrors int `json:"total_health_errors"`
}

func CollectPoolMetricsForRegistry(reg *storage.DBRegistry) *MetricsResponse {
	if reg == nil {
		return nil
	}

	response := &MetricsResponse{
		Timestamp:   time.Now(),
		PoolMetrics: []PoolMetrics{},
		Summary:     MetricsSummary{},
	}

	connectionNames := reg.ListConnections()
	var totalOpen, totalIdle, totalActive, totalErrors int
	var healthyPools, unhealthyPools int

	for _, name := range connectionNames {
		state, openConns, idleConns, err := reg.GetConnectionStats(name)
		conn, _ := reg.GetConnection(name)

		poolMetric := PoolMetrics{
			Name:              name,
			OpenConnections:   openConns,
			IdleConnections:   idleConns,
			ActiveConnections: openConns - idleConns,
			InUseConnections:  openConns - idleConns,
		}

		if conn != nil {
			poolMetric.LastHealthCheck = conn.LastHealthCheck()
			poolMetric.MaxOpenConnections = conn.Config.MaxOpenConnections
			poolMetric.MinConnections = conn.Config.MinConnections
			poolMetric.ConnectionTimeout = conn.Config.ConnectionTimeout
			poolMetric.IdleTimeout = conn.Config.IdleTimeout
			poolMetric.HealthCheckInterval = conn.Config.HealthCheckInterval
		}

		switch state {
		case storage.StateHealthy:
			poolMetric.State = "healthy"
			healthyPools++
		case storage.StateUnhealthy:
			poolMetric.State = "unhealthy"
			unhealthyPools++
			totalErrors++
		case storage.StateClosed:
			poolMetric.State = "closed"
		default:
			poolMetric.State = "unknown"
		}

		if err != nil {
			poolMetric.HealthCheckErrors++
		}

		totalOpen += openConns
		totalIdle += idleConns
		totalActive += openConns - idleConns

		response.PoolMetrics = append(response.PoolMetrics, poolMetric)
	}

	response.Summary = MetricsSummary{
		TotalPools:        len(connectionNames),
		TotalOpenConns:    totalOpen,
		TotalIdleConns:    totalIdle,
		TotalActiveConns:  totalActive,
		HealthyPools:      healthyPools,
		UnhealthyPools:    unhealthyPools,
		TotalHealthErrors: totalErrors,
	}

	return response
}

func getDetailedPoolStats(ctx context.Context, req *logical.Request) (*logical.Response, error) {
	reg := getDBRegistryFromBackend(req)
	if reg == nil {
		return nil, fmt.Errorf("database registry not available")
	}

	metricsResp := CollectPoolMetricsForRegistry(reg)

	return &logical.Response{
		Data: map[string]interface{}{
			"timestamp":     metricsResp.Timestamp,
			"pool_metrics":  metricsResp.PoolMetrics,
			"summary":       metricsResp.Summary,
			"health_status": getHealthStatus(metricsResp.Summary),
			"cache_stats":   getCacheStats(req),
		},
	}, nil
}

func getDBRegistryFromBackend(req *logical.Request) *storage.DBRegistry {
	if req == nil {
		return nil
	}
	return nil
}

func getHealthStatus(summary MetricsSummary) string {
	if summary.TotalPools == 0 {
		return "uninitialized"
	}
	if summary.UnhealthyPools > 0 {
		return "degraded"
	}
	if summary.HealthyPools == summary.TotalPools {
		return "healthy"
	}
	return "unknown"
}

func getCacheStats(req *logical.Request) map[string]interface{} {
	return map[string]interface{}{
		"credential_cache": map[string]interface{}{
			"enabled": true,
		},
		"query_cache": map[string]interface{}{
			"enabled": true,
		},
	}
}
