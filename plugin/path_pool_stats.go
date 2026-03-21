package teradata

import (
	"context"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathPoolStats() *framework.Path {
	return &framework.Path{
		Pattern:         "pool-stats",
		HelpSynopsis:    "Connection pool statistics",
		HelpDescription: "Returns statistics about the database connection pool.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathPoolStatsRead,
			},
		},
	}
}

func (b *Backend) pathPoolStatsRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if cfg == nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"initialized": false,
			},
		}, nil
	}

	registry := b.getDBRegistry()
	connectionNames := registry.ListConnections()

	pools := make([]map[string]interface{}, 0)
	totalActive := 0
	totalIdle := 0
	totalInUse := 0
	totalWaitCount := int64(0)
	totalErrors := 0

	for _, name := range connectionNames {
		stats, connErr := registry.GetDetailedConnectionStats(name)

		poolInfo := map[string]interface{}{
			"name":                name,
			"active":              stats.OpenConnections - stats.Idle,
			"idle":                stats.Idle,
			"in_use":              stats.InUse,
			"total":               stats.OpenConnections,
			"max_open":            stats.MaxOpen,
			"min":                 stats.MinConnections,
			"state":               stateToString(stats.State),
			"wait_count":          stats.WaitCount,
			"wait_duration_nanos": stats.WaitDurationNanos,
			"max_idle_closed":     stats.MaxIdleClosed,
			"max_lifetime_closed": stats.MaxLifetimeClosed,
			"last_health_check":   stats.LastHealthCheck,
			"error":               nil,
		}

		if connErr != nil {
			poolInfo["error"] = connErr.Error()
			totalErrors++
		} else if stats.HealthError != nil {
			poolInfo["error"] = stats.HealthError.Error()
			totalErrors++
		}

		totalActive += stats.OpenConnections - stats.Idle
		totalIdle += stats.Idle
		totalInUse += stats.InUse
		totalWaitCount += stats.WaitCount

		pools = append(pools, poolInfo)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"initialized":      true,
			"total_pools":      len(pools),
			"total_active":     totalActive,
			"total_idle":       totalIdle,
			"total_in_use":     totalInUse,
			"total_wait_count": totalWaitCount,
			"total_errors":     totalErrors,
			"pools":            pools,
		},
	}, nil
}

func stateToString(state storage.ConnectionState) string {
	switch state {
	case storage.StateHealthy:
		return "healthy"
	case storage.StateUnhealthy:
		return "unhealthy"
	case storage.StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}
