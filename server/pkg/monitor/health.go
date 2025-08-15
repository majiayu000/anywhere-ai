package monitor

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	Healthy   HealthStatus = "healthy"
	Unhealthy HealthStatus = "unhealthy"
	Degraded  HealthStatus = "degraded"
)

// HealthCheck 健康检查接口
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) HealthCheckResult
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Details interface{}  `json:"details,omitempty"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]HealthCheck
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheck),
	}
}

// Register 注册健康检查
func (hc *HealthChecker) Register(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[check.Name()] = check
}

// Unregister 取消注册健康检查
func (hc *HealthChecker) Unregister(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.checks, name)
}

// Check 执行所有健康检查
func (hc *HealthChecker) Check(ctx context.Context) map[string]HealthCheckResult {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck, len(hc.checks))
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mu.RUnlock()

	results := make(map[string]HealthCheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check HealthCheck) {
			defer wg.Done()

			// 设置超时
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			result := check.Check(checkCtx)

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, check)
	}

	wg.Wait()
	return results
}

// OverallStatus 获取整体健康状态
func (hc *HealthChecker) OverallStatus(ctx context.Context) HealthStatus {
	results := hc.Check(ctx)

	if len(results) == 0 {
		return Healthy
	}

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for _, result := range results {
		switch result.Status {
		case Healthy:
			healthyCount++
		case Degraded:
			degradedCount++
		case Unhealthy:
			unhealthyCount++
		}
	}

	// 如果有任何不健康的检查，整体状态为不健康
	if unhealthyCount > 0 {
		return Unhealthy
	}

	// 如果有降级的检查，整体状态为降级
	if degradedCount > 0 {
		return Degraded
	}

	return Healthy
}

// SystemInfo 系统信息
type SystemInfo struct {
	Version      string     `json:"version"`
	Uptime       string     `json:"uptime"`
	Timestamp    time.Time  `json:"timestamp"`
	GoVersion    string     `json:"go_version"`
	NumCPU       int        `json:"num_cpu"`
	NumGoroutine int        `json:"num_goroutine"`
	Memory       MemoryInfo `json:"memory"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Alloc      uint64 `json:"alloc"`       // 当前分配的内存
	TotalAlloc uint64 `json:"total_alloc"` // 累计分配的内存
	Sys        uint64 `json:"sys"`         // 系统内存
	NumGC      uint32 `json:"num_gc"`      // GC次数
}

var (
	startTime = time.Now()
	version   = "dev"
)

// SetVersion 设置版本号
func SetVersion(v string) {
	version = v
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemInfo{
		Version:      version,
		Uptime:       time.Since(startTime).String(),
		Timestamp:    time.Now(),
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		Memory: MemoryInfo{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
		},
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status HealthStatus                 `json:"status"`
	Checks map[string]HealthCheckResult `json:"checks,omitempty"`
	System SystemInfo                   `json:"system"`
}

// HealthHandler 健康检查处理器
func HealthHandler(checker *HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		checks := checker.Check(ctx)
		overallStatus := checker.OverallStatus(ctx)

		response := HealthResponse{
			Status: overallStatus,
			Checks: checks,
			System: GetSystemInfo(),
		}

		statusCode := http.StatusOK
		if overallStatus == Unhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if overallStatus == Degraded {
			statusCode = http.StatusPartialContent
		}

		c.JSON(statusCode, response)
	}
}

// ReadinessHandler 就绪检查处理器（简化版健康检查）
func ReadinessHandler(checker *HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		overallStatus := checker.OverallStatus(ctx)

		if overallStatus == Unhealthy {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	}
}

// LivenessHandler 存活检查处理器（最基本的检查）
func LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now(),
		})
	}
}

// 内置健康检查实现

// MemoryHealthCheck 内存健康检查
type MemoryHealthCheck struct {
	MaxMemoryMB uint64
}

func (m *MemoryHealthCheck) Name() string {
	return "memory"
}

func (m *MemoryHealthCheck) Check(ctx context.Context) HealthCheckResult {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	currentMemoryMB := memStats.Alloc / 1024 / 1024

	if currentMemoryMB > m.MaxMemoryMB {
		return HealthCheckResult{
			Status:  Unhealthy,
			Message: fmt.Sprintf("内存使用过高: %dMB > %dMB", currentMemoryMB, m.MaxMemoryMB),
			Details: map[string]interface{}{
				"current_mb": currentMemoryMB,
				"max_mb":     m.MaxMemoryMB,
			},
		}
	}

	return HealthCheckResult{
		Status:  Healthy,
		Message: fmt.Sprintf("内存使用正常: %dMB", currentMemoryMB),
		Details: map[string]interface{}{
			"current_mb": currentMemoryMB,
			"max_mb":     m.MaxMemoryMB,
		},
	}
}

// GoroutineHealthCheck Goroutine健康检查
type GoroutineHealthCheck struct {
	MaxGoroutines int
}

func (g *GoroutineHealthCheck) Name() string {
	return "goroutines"
}

func (g *GoroutineHealthCheck) Check(ctx context.Context) HealthCheckResult {
	currentGoroutines := runtime.NumGoroutine()

	if currentGoroutines > g.MaxGoroutines {
		return HealthCheckResult{
			Status:  Unhealthy,
			Message: fmt.Sprintf("Goroutine数量过多: %d > %d", currentGoroutines, g.MaxGoroutines),
			Details: map[string]interface{}{
				"current": currentGoroutines,
				"max":     g.MaxGoroutines,
			},
		}
	}

	return HealthCheckResult{
		Status:  Healthy,
		Message: fmt.Sprintf("Goroutine数量正常: %d", currentGoroutines),
		Details: map[string]interface{}{
			"current": currentGoroutines,
			"max":     g.MaxGoroutines,
		},
	}
}
