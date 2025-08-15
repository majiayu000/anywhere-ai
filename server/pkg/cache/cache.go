package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	Flush(ctx context.Context) error
	Close() error
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `mapstructure:"addr" yaml:"addr"`
	Password     string `mapstructure:"password" yaml:"password"`
	DB           int    `mapstructure:"db" yaml:"db"`
	PoolSize     int    `mapstructure:"pool_size" yaml:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns" yaml:"min_idle_conns"`
	DialTimeout  int    `mapstructure:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout" yaml:"idle_timeout"`
}

// DefaultRedisConfig 默认Redis配置
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5,
		ReadTimeout:  3,
		WriteTimeout: 3,
		IdleTimeout:  300,
	}
}

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client
	config RedisConfig
}

// NewRedisCache 创建Redis缓存
func NewRedisCache(config RedisConfig) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  time.Duration(config.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
	})

	return &RedisCache{
		client: rdb,
		config: config,
	}
}

// Get 获取缓存
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set 设置缓存
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		val = string(data)
	}
	return r.client.Set(ctx, key, val, expiration).Err()
}

// Del 删除缓存
func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	return result > 0, err
}

// Expire 设置过期时间
func (r *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL 获取剩余生存时间
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Flush 清空所有缓存
func (r *RedisCache) Flush(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Ping 测试连接
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*item
}

type item struct {
	value      string
	expiration int64
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{
		items: make(map[string]*item),
	}

	// 启动清理协程
	go mc.cleanup()

	return mc
}

// Get 获取缓存
func (m *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return "", fmt.Errorf("key not found")
	}

	// 检查是否过期
	if item.expiration > 0 && time.Now().UnixNano() > item.expiration {
		return "", fmt.Errorf("key expired")
	}

	return item.value, nil
}

// Set 设置缓存
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var val string
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		val = string(data)
	}

	var exp int64
	if expiration > 0 {
		exp = time.Now().Add(expiration).UnixNano()
	}

	m.items[key] = &item{
		value:      val,
		expiration: exp,
	}

	return nil
}

// Del 删除缓存
func (m *MemoryCache) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.items, key)
	}

	return nil
}

// Exists 检查键是否存在
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if item.expiration > 0 && time.Now().UnixNano() > item.expiration {
		return false, nil
	}

	return true, nil
}

// Expire 设置过期时间
func (m *MemoryCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[key]
	if !exists {
		return fmt.Errorf("key not found")
	}

	if expiration > 0 {
		item.expiration = time.Now().Add(expiration).UnixNano()
	} else {
		item.expiration = 0
	}

	return nil
}

// TTL 获取剩余生存时间
func (m *MemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return -2 * time.Second, nil // key不存在
	}

	if item.expiration == 0 {
		return -1 * time.Second, nil // 永不过期
	}

	remaining := time.Duration(item.expiration - time.Now().UnixNano())
	if remaining <= 0 {
		return -2 * time.Second, nil // 已过期
	}

	return remaining, nil
}

// Flush 清空所有缓存
func (m *MemoryCache) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]*item)
	return nil
}

// Close 关闭缓存
func (m *MemoryCache) Close() error {
	return nil
}

// cleanup 清理过期项
func (m *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now().UnixNano()
		for key, item := range m.items {
			if item.expiration > 0 && now > item.expiration {
				delete(m.items, key)
			}
		}
		m.mu.Unlock()
	}
}

// CacheHealthCheck 缓存健康检查
type CacheHealthCheck struct {
	Cache Cache
}

func (c *CacheHealthCheck) Name() string {
	return "cache"
}

func (c *CacheHealthCheck) Check(ctx context.Context) CacheHealthCheckResult {
	if c.Cache == nil {
		return CacheHealthCheckResult{
			Status:  "unhealthy",
			Message: "缓存实例未初始化",
		}
	}

	// 测试设置和获取
	testKey := "health_check_test"
	testValue := "test_value"

	if err := c.Cache.Set(ctx, testKey, testValue, time.Minute); err != nil {
		return CacheHealthCheckResult{
			Status:  "unhealthy",
			Message: fmt.Sprintf("缓存设置失败: %v", err),
		}
	}

	value, err := c.Cache.Get(ctx, testKey)
	if err != nil {
		return CacheHealthCheckResult{
			Status:  "unhealthy",
			Message: fmt.Sprintf("缓存获取失败: %v", err),
		}
	}

	if value != testValue {
		return CacheHealthCheckResult{
			Status:  "unhealthy",
			Message: "缓存值不匹配",
		}
	}

	// 清理测试键
	c.Cache.Del(ctx, testKey)

	return CacheHealthCheckResult{
		Status:  "healthy",
		Message: "缓存连接正常",
	}
}

// CacheHealthCheckResult 缓存健康检查结果
type CacheHealthCheckResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}
