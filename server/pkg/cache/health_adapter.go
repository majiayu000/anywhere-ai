package cache

import (
	"context"

	"github.com/majiayu000/gin-starter/pkg/monitor"
)

// CacheHealthAdapter 缓存健康检查适配器
type CacheHealthAdapter struct {
	Cache Cache
}

func (c *CacheHealthAdapter) Name() string {
	return "cache"
}

func (c *CacheHealthAdapter) Check(ctx context.Context) monitor.HealthCheckResult {
	checker := &CacheHealthCheck{Cache: c.Cache}
	result := checker.Check(ctx)

	// 转换状态
	var status monitor.HealthStatus
	switch result.Status {
	case "healthy":
		status = monitor.Healthy
	case "unhealthy":
		status = monitor.Unhealthy
	default:
		status = monitor.Degraded
	}

	return monitor.HealthCheckResult{
		Status:  status,
		Message: result.Message,
		Details: result.Details,
	}
}
