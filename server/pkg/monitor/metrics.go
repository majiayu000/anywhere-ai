package monitor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Metrics 指标收集器
type Metrics struct {
	mu sync.RWMutex

	// HTTP指标
	TotalRequests    int64                    `json:"total_requests"`
	ActiveRequests   int64                    `json:"active_requests"`
	RequestsByStatus map[int]int64            `json:"requests_by_status"`
	RequestsByMethod map[string]int64         `json:"requests_by_method"`
	RequestsByPath   map[string]int64         `json:"requests_by_path"`
	ResponseTimes    map[string]*ResponseTime `json:"response_times"`

	// 系统指标
	StartTime time.Time `json:"start_time"`
	Uptime    string    `json:"uptime"`
}

// ResponseTime 响应时间统计
type ResponseTime struct {
	Count   int64         `json:"count"`
	Total   time.Duration `json:"total_ns"`
	Min     time.Duration `json:"min_ns"`
	Max     time.Duration `json:"max_ns"`
	Average time.Duration `json:"average_ns"`
}

// NewMetrics 创建指标收集器
func NewMetrics() *Metrics {
	return &Metrics{
		RequestsByStatus: make(map[int]int64),
		RequestsByMethod: make(map[string]int64),
		RequestsByPath:   make(map[string]int64),
		ResponseTimes:    make(map[string]*ResponseTime),
		StartTime:        time.Now(),
	}
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	atomic.AddInt64(&m.TotalRequests, 1)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 记录状态码
	m.RequestsByStatus[statusCode]++

	// 记录方法
	m.RequestsByMethod[method]++

	// 记录路径
	m.RequestsByPath[path]++

	// 记录响应时间
	key := method + " " + path
	if rt, exists := m.ResponseTimes[key]; exists {
		rt.Count++
		rt.Total += duration
		if duration < rt.Min || rt.Min == 0 {
			rt.Min = duration
		}
		if duration > rt.Max {
			rt.Max = duration
		}
		rt.Average = rt.Total / time.Duration(rt.Count)
	} else {
		m.ResponseTimes[key] = &ResponseTime{
			Count:   1,
			Total:   duration,
			Min:     duration,
			Max:     duration,
			Average: duration,
		}
	}
}

// IncrementActiveRequests 增加活跃请求数
func (m *Metrics) IncrementActiveRequests() {
	atomic.AddInt64(&m.ActiveRequests, 1)
}

// DecrementActiveRequests 减少活跃请求数
func (m *Metrics) DecrementActiveRequests() {
	atomic.AddInt64(&m.ActiveRequests, -1)
}

// GetSnapshot 获取指标快照
func (m *Metrics) GetSnapshot() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &Metrics{
		TotalRequests:    atomic.LoadInt64(&m.TotalRequests),
		ActiveRequests:   atomic.LoadInt64(&m.ActiveRequests),
		RequestsByStatus: make(map[int]int64),
		RequestsByMethod: make(map[string]int64),
		RequestsByPath:   make(map[string]int64),
		ResponseTimes:    make(map[string]*ResponseTime),
		StartTime:        m.StartTime,
		Uptime:           time.Since(m.StartTime).String(),
	}

	// 复制映射
	for k, v := range m.RequestsByStatus {
		snapshot.RequestsByStatus[k] = v
	}
	for k, v := range m.RequestsByMethod {
		snapshot.RequestsByMethod[k] = v
	}
	for k, v := range m.RequestsByPath {
		snapshot.RequestsByPath[k] = v
	}
	for k, v := range m.ResponseTimes {
		snapshot.ResponseTimes[k] = &ResponseTime{
			Count:   v.Count,
			Total:   v.Total,
			Min:     v.Min,
			Max:     v.Max,
			Average: v.Average,
		}
	}

	return snapshot
}

// Reset 重置指标
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.StoreInt64(&m.TotalRequests, 0)
	atomic.StoreInt64(&m.ActiveRequests, 0)

	m.RequestsByStatus = make(map[int]int64)
	m.RequestsByMethod = make(map[string]int64)
	m.RequestsByPath = make(map[string]int64)
	m.ResponseTimes = make(map[string]*ResponseTime)
	m.StartTime = time.Now()
}

// 全局指标实例
var globalMetrics = NewMetrics()

// GetGlobalMetrics 获取全局指标实例
func GetGlobalMetrics() *Metrics {
	return globalMetrics
}

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 增加活跃请求数
		globalMetrics.IncrementActiveRequests()
		defer globalMetrics.DecrementActiveRequests()

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start)
		globalMetrics.RecordRequest(
			c.Request.Method,
			c.FullPath(),
			c.Writer.Status(),
			duration,
		)
	}
}

// MetricsHandler 指标查看处理器
func MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics := globalMetrics.GetSnapshot()
		c.JSON(200, metrics)
	}
}

// ErrorRate 计算错误率
func (m *Metrics) ErrorRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := atomic.LoadInt64(&m.TotalRequests)
	if total == 0 {
		return 0
	}

	errorCount := int64(0)
	for status, count := range m.RequestsByStatus {
		if status >= 400 {
			errorCount += count
		}
	}

	return float64(errorCount) / float64(total) * 100
}

// AverageResponseTime 计算平均响应时间
func (m *Metrics) AverageResponseTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalDuration := time.Duration(0)
	totalCount := int64(0)

	for _, rt := range m.ResponseTimes {
		totalDuration += rt.Total
		totalCount += rt.Count
	}

	if totalCount == 0 {
		return 0
	}

	return totalDuration / time.Duration(totalCount)
}

// RequestsPerSecond 计算每秒请求数
func (m *Metrics) RequestsPerSecond() float64 {
	uptime := time.Since(m.StartTime).Seconds()
	if uptime == 0 {
		return 0
	}

	total := atomic.LoadInt64(&m.TotalRequests)
	return float64(total) / uptime
}

// Summary 获取指标摘要
type Summary struct {
	TotalRequests       int64         `json:"total_requests"`
	ActiveRequests      int64         `json:"active_requests"`
	ErrorRate           float64       `json:"error_rate_percent"`
	AverageResponseTime time.Duration `json:"average_response_time_ns"`
	RequestsPerSecond   float64       `json:"requests_per_second"`
	Uptime              string        `json:"uptime"`
}

// GetSummary 获取指标摘要
func (m *Metrics) GetSummary() Summary {
	return Summary{
		TotalRequests:       atomic.LoadInt64(&m.TotalRequests),
		ActiveRequests:      atomic.LoadInt64(&m.ActiveRequests),
		ErrorRate:           m.ErrorRate(),
		AverageResponseTime: m.AverageResponseTime(),
		RequestsPerSecond:   m.RequestsPerSecond(),
		Uptime:              time.Since(m.StartTime).String(),
	}
}

// SummaryHandler 指标摘要处理器
func SummaryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		summary := globalMetrics.GetSummary()
		c.JSON(200, summary)
	}
}
