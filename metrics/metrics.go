package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PoolOpenConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "open_connections",
			Help:      "Number of open connections in the pool",
		},
		[]string{"pool_name"},
	)

	PoolIdleConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "idle_connections",
			Help:      "Number of idle connections in the pool",
		},
		[]string{"pool_name"},
	)

	PoolActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "active_connections",
			Help:      "Number of active (in-use) connections in the pool",
		},
		[]string{"pool_name"},
	)

	PoolHealthCheckDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "health_check_duration_seconds",
			Help:      "Duration of health check operations in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"pool_name", "status"},
	)

	PoolHealthCheckTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "health_check_total",
			Help:      "Total number of health checks performed",
		},
		[]string{"pool_name", "status"},
	)

	PoolConnectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "connection_errors_total",
			Help:      "Total number of connection errors",
		},
		[]string{"pool_name", "error_type"},
	)

	PoolIdleClosedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "pool",
			Name:      "idle_closed_total",
			Help:      "Total number of idle connections closed due to timeout",
		},
		[]string{"pool_name"},
	)

	CredentialsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "credentials",
			Name:      "created_total",
			Help:      "Total number of credentials created",
		},
		[]string{"role_name"},
	)

	CredentialsCreationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "teradata",
			Subsystem: "credentials",
			Name:      "creation_duration_seconds",
			Help:      "Duration of credential creation operations in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"role_name"},
	)

	CredentialsCreationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "credentials",
			Name:      "creation_errors_total",
			Help:      "Total number of credential creation errors",
		},
		[]string{"role_name", "error_type"},
	)

	CredentialsRevoked = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "credentials",
			Name:      "revoked_total",
			Help:      "Total number of credentials revoked",
		},
		[]string{"role_name"},
	)

	CredentialsExpired = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "credentials",
			Name:      "expired_total",
			Help:      "Total number of credentials expired and cleaned up",
		},
		[]string{"role_name"},
	)

	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "cache",
			Name:      "hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "cache",
			Name:      "misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	CacheEvictions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "cache",
			Name:      "evictions_total",
			Help:      "Total number of cache evictions",
		},
		[]string{"cache_type"},
	)

	RateLimitAllowed = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "rate_limit",
			Name:      "allowed_total",
			Help:      "Total number of requests allowed by rate limiter",
		},
	)

	RateLimitRejected = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "rate_limit",
			Name:      "rejected_total",
			Help:      "Total number of requests rejected by rate limiter",
		},
	)

	SQLExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "teradata",
			Subsystem: "sql",
			Name:      "execution_duration_seconds",
			Help:      "Duration of SQL execution operations in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	SQLExecutionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "teradata",
			Subsystem: "sql",
			Name:      "execution_errors_total",
			Help:      "Total number of SQL execution errors",
		},
		[]string{"operation", "error_type"},
	)
)
