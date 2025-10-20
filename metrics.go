package distcache

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 包含所有的 Prometheus 指标
type Metrics struct {
	// 请求计数器
	RequestsTotal *prometheus.CounterVec
	// 缓存命中计数器
	HitsTotal *prometheus.CounterVec
	// 热点键命中计数器
	HotKeyHitsTotal prometheus.Counter
	// 热点键统计
	HotKeysTotal *prometheus.CounterVec
	// 请求延迟直方图
	RequestDuration *prometheus.HistogramVec
	// 布隆过滤器查询计数器
	BloomFilterQueries *prometheus.CounterVec
	// 当前缓存大小
	CacheSize *prometheus.GaugeVec
}

var (
	// 全局指标实例
	globalMetrics *Metrics
	metricsOnce   sync.Once
)

// GetMetrics 获取全局 Metrics 实例（单例模式）
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = NewMetrics()
	})
	return globalMetrics
}

// NewMetrics 创建一个新的 Metrics 实例
func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "distcache_requests_total",
				Help: "The total number of cache requests",
			},
			[]string{"method", "status"},
		),
		HitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "distcache_hits_total",
				Help: "The total number of cache hits",
			},
			[]string{"type"}, // local, hot, remote
		),
		HotKeyHitsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "distcache_hot_key_hits_total",
				Help: "The total number of hot key hits",
			},
		),
		HotKeysTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "distcache_hot_keys_total",
				Help: "The total number of hot keys identified",
			},
			[]string{"action"}, // promoted, demoted
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "distcache_request_duration_seconds",
				Help:    "The request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "status"},
		),
		BloomFilterQueries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "distcache_bloom_filter_queries_total",
				Help: "The total number of bloom filter queries",
			},
			[]string{"result"}, // hit, miss
		),
		CacheSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "distcache_cache_size_bytes",
				Help: "The current size of cache in bytes",
			},
			[]string{"group"},
		),
	}
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(method, status string) {
	m.RequestsTotal.WithLabelValues(method, status).Inc()
}

// RecordHit 记录缓存命中
func (m *Metrics) RecordHit(hitType string) {
	m.HitsTotal.WithLabelValues(hitType).Inc()
}

// RecordHotKeyHit 记录热点键命中
func (m *Metrics) RecordHotKeyHit() {
	m.HotKeyHitsTotal.Inc()
}

// RecordHotKey 记录热点键操作
func (m *Metrics) RecordHotKey(action string) {
	m.HotKeysTotal.WithLabelValues(action).Inc()
}

// RecordDuration 记录请求延迟
func (m *Metrics) RecordDuration(method, status string, duration float64) {
	m.RequestDuration.WithLabelValues(method, status).Observe(duration)
}

// RecordBloomFilter 记录布隆过滤器查询
func (m *Metrics) RecordBloomFilter(result string) {
	m.BloomFilterQueries.WithLabelValues(result).Inc()
}

// SetCacheSize 设置缓存大小
func (m *Metrics) SetCacheSize(group string, size int64) {
	m.CacheSize.WithLabelValues(group).Set(float64(size))
}

// EnableMetrics 启用 Prometheus 指标收集（可选调用）
// 如果不调用此函数，指标收集将被禁用
var metricsEnabled bool

func EnableMetrics() {
	metricsEnabled = true
}

// DisableMetrics 禁用 Prometheus 指标收集
func DisableMetrics() {
	metricsEnabled = false
}

// IsMetricsEnabled 检查指标收集是否启用
func IsMetricsEnabled() bool {
	return metricsEnabled
}
