package distcache

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer HTTP 监控服务器
type MetricsServer struct {
	addr   string
	server *http.Server
}

// 全局统计信息
var (
	totalRequests int64
	totalHits     int64
	hotKeyHits    int64
)

// NewMetricsServer 创建一个新的监控服务器
func NewMetricsServer(addr string) *MetricsServer {
	return &MetricsServer{
		addr: addr,
	}
}

// Start 启动监控服务器
func (ms *MetricsServer) Start() error {
	mux := http.NewServeMux()

	// Prometheus 指标端点
	mux.Handle("/metrics", promhttp.Handler())

	// 健康检查端点
	mux.HandleFunc("/health", ms.healthHandler)

	// 统计信息端点（JSON 格式）
	mux.HandleFunc("/stats", ms.statsHandler)

	// 状态面板（HTML 格式）
	mux.HandleFunc("/status", ms.statusHandler)

	ms.server = &http.Server{
		Addr:    ms.addr,
		Handler: mux,
	}

	log.Printf("[MetricsServer] Starting metrics server on %s", ms.addr)
	log.Printf("[MetricsServer] Prometheus metrics: http://%s/metrics", ms.addr)
	log.Printf("[MetricsServer] Status dashboard: http://%s/status", ms.addr)

	return ms.server.ListenAndServe()
}

// Stop 停止监控服务器
func (ms *MetricsServer) Stop() error {
	if ms.server != nil {
		return ms.server.Close()
	}
	return nil
}

// healthHandler 健康检查处理器
func (ms *MetricsServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// statsHandler 统计信息处理器
func (ms *MetricsServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	requests := atomic.LoadInt64(&totalRequests)
	hits := atomic.LoadInt64(&totalHits)
	hotHits := atomic.LoadInt64(&hotKeyHits)

	hitRate := 0.0
	if requests > 0 {
		hitRate = float64(hits) / float64(requests)
	}

	hotKeyHitRate := 0.0
	if hits > 0 {
		hotKeyHitRate = float64(hotHits) / float64(hits)
	}

	stats := map[string]interface{}{
		"cache_stats": map[string]interface{}{
			"total_requests":   requests,
			"total_hits":       hits,
			"hot_key_hits":     hotHits,
			"hit_rate":         hitRate,
			"hot_key_hit_rate": hotKeyHitRate,
		},
		"system_info": map[string]interface{}{
			"shard_count":            256,
			"default_hot_threshold":  10,
			"default_decay_interval": "5m0s",
		},
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// statusHandler 状态面板处理器
func (ms *MetricsServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	requests := atomic.LoadInt64(&totalRequests)
	hits := atomic.LoadInt64(&totalHits)
	hotHits := atomic.LoadInt64(&hotKeyHits)

	hitRate := 0.0
	if requests > 0 {
		hitRate = float64(hits) / float64(requests) * 100
	}

	hotKeyHitRate := 0.0
	if hits > 0 {
		hotKeyHitRate = float64(hotHits) / float64(hits) * 100
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>DistCache 监控面板</title>
    <meta charset="UTF-8">
    <meta http-equiv="refresh" content="5">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #4CAF50; padding-bottom: 10px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 30px 0; }
        .stat-card { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 20px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .stat-card h3 { margin: 0 0 10px 0; font-size: 14px; opacity: 0.9; }
        .stat-card .value { font-size: 32px; font-weight: bold; margin: 10px 0; }
        .stat-card .label { font-size: 12px; opacity: 0.8; }
        .links { margin-top: 30px; padding: 20px; background: #f9f9f9; border-radius: 8px; }
        .links a { display: inline-block; margin: 5px 10px 5px 0; padding: 10px 20px; background: #4CAF50; color: white; text-decoration: none; border-radius: 4px; transition: background 0.3s; }
        .links a:hover { background: #45a049; }
        .timestamp { text-align: right; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 DistCache 实时监控面板</h1>
        
        <div class="stats">
            <div class="stat-card">
                <h3>总请求数</h3>
                <div class="value">%d</div>
                <div class="label">Total Requests</div>
            </div>
            
            <div class="stat-card">
                <h3>缓存命中数</h3>
                <div class="value">%d</div>
                <div class="label">Cache Hits</div>
            </div>
            
            <div class="stat-card">
                <h3>缓存命中率</h3>
                <div class="value">%.2f%%</div>
                <div class="label">Hit Rate</div>
            </div>
            
            <div class="stat-card">
                <h3>热点键命中</h3>
                <div class="value">%d</div>
                <div class="label">Hot Key Hits</div>
            </div>
            
            <div class="stat-card">
                <h3>热点命中率</h3>
                <div class="value">%.2f%%</div>
                <div class="label">Hot Key Hit Rate</div>
            </div>
        </div>
        
        <div class="links">
            <h3>📈 监控端点</h3>
            <a href="/metrics" target="_blank">Prometheus 指标</a>
            <a href="/stats" target="_blank">JSON 统计</a>
            <a href="/health" target="_blank">健康检查</a>
        </div>
        
        <div class="timestamp">
            🕒 更新时间: %s | 自动刷新: 5秒
        </div>
    </div>
</body>
</html>
`, requests, hits, hitRate, hotHits, hotKeyHitRate, time.Now().Format("2006-01-02 15:04:05"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// StartMetricsServer 启动监控服务器（阻塞）
func StartMetricsServer(addr string) error {
	server := NewMetricsServer(addr)
	return server.Start()
}

// StartMetricsServerAsync 异步启动监控服务器
func StartMetricsServerAsync(addr string) *MetricsServer {
	server := NewMetricsServer(addr)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[MetricsServer] Error starting metrics server: %v", err)
		}
	}()
	return server
}

// 内部函数：更新统计计数器
func incrementTotalRequests() {
	atomic.AddInt64(&totalRequests, 1)
}

func incrementTotalHits() {
	atomic.AddInt64(&totalHits, 1)
}

func incrementHotKeyHits() {
	atomic.AddInt64(&hotKeyHits, 1)
}
