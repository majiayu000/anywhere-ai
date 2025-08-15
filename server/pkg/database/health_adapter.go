package database

import (
	"context"

	"github.com/majiayu000/gin-starter/pkg/monitor"
)

// DatabaseHealthAdapter 数据库健康检查适配器
type DatabaseHealthAdapter struct {
	*HealthCheck
}

// Name 返回健康检查名称
func (d *DatabaseHealthAdapter) Name() string {
	return d.HealthCheck.Name()
}

// Check 执行健康检查并返回monitor包的结果类型
func (d *DatabaseHealthAdapter) Check(ctx context.Context) monitor.HealthCheckResult {
	result := d.HealthCheck.Check(ctx)
	return monitor.HealthCheckResult{
		Status:  monitor.HealthStatus(result.Status),
		Message: result.Message,
		Details: result.Details,
	}
}
