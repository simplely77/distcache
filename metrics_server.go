package distcache

import (
	"encoding/json"
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

// 全局统计信息（用于简单的内部统计）
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

	// Prometheus 指标端点（核心端点，供 Prometheus 抓取）
	mux.Handle("/metrics", promhttp.Handler())

	// 健康检查端点
	mux.HandleFunc("/health", ms.healthHandler)

	ms.server = &http.Server{
		Addr:    ms.addr,
		Handler: mux,
	}

	log.Printf("[MetricsServer] Starting metrics server on %s", ms.addr)
	log.Printf("[MetricsServer] Prometheus endpoint: http://%s/metrics", ms.addr)

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

// StartMetricsServer 启动监控服务器（阻塞模式）
func StartMetricsServer(addr string) error {
	server := NewMetricsServer(addr)
	return server.Start()
}

// StartMetricsServerAsync 异步启动监控服务器（推荐）
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
