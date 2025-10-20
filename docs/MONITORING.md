# Prometheus 监控集成指南

DistCache 内置了完整的 Prometheus 监控支持，让您轻松监控缓存性能和热点检测效果。

## 快速开始

### 1. 启用监控

```go
import "github.com/simplely77/distcache"

// 在应用启动时启用监控
distcache.EnableMetrics()

// 创建缓存组
cache := distcache.NewGroup("mydata", 2<<20, getter)

// 启动监控服务器
server := distcache.StartMetricsServerAsync(":9090")
defer server.Stop()
```

### 2. 访问监控端点

启动应用后，您可以访问以下端点：

- **Prometheus 指标**: `http://localhost:9090/metrics`  
  标准 Prometheus 格式，可被 Prometheus 服务器抓取

- **可视化面板**: `http://localhost:9090/status`  
  实时 HTML 仪表板，显示缓存性能统计

- **JSON API**: `http://localhost:9090/stats`  
  RESTful API 返回结构化统计数据

- **健康检查**: `http://localhost:9090/health`  
  服务健康状态和版本信息

## 监控指标详解

### 缓存性能指标

```promql
# 总请求数（按方法和状态分类）
distcache_requests_total{method="get", status="success"}

# 缓存命中数（按类型分类：local/hot/remote）
distcache_hits_total{type="local"}
distcache_hits_total{type="hot"}

# 请求延迟分布（直方图）
distcache_request_duration_seconds
```

### 热点检测指标

```promql
# 热点键命中总数
distcache_hot_key_hits_total

# 热点键操作统计（promoted/demoted）
distcache_hot_keys_total{action="promoted"}
distcache_hot_keys_total{action="demoted"}

# 布隆过滤器查询统计
distcache_bloom_filter_queries_total{result="hit"}
distcache_bloom_filter_queries_total{result="miss"}
```

### 系统指标

```promql
# 缓存大小（按组分类）
distcache_cache_size_bytes{group="mydata"}
```

## Prometheus 集成

### 配置 Prometheus

创建 `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'distcache'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: /metrics
```

启动 Prometheus:

```bash
prometheus --config.file=prometheus.yml
```

### Grafana 可视化

1. 添加 Prometheus 数据源
2. 导入 Grafana 面板（JSON 配置见 `configs/grafana-dashboard.json`）
3. 查看实时监控图表

## 常用 PromQL 查询

### 缓存命中率

```promql
# 整体命中率
rate(distcache_hits_total[5m]) / rate(distcache_requests_total[5m])

# 热点键命中率
rate(distcache_hits_total{type="hot"}[5m]) / rate(distcache_hits_total[5m])
```

### 请求 QPS

```promql
# 每秒请求数
rate(distcache_requests_total[1m])

# 成功请求 QPS
rate(distcache_requests_total{status="success"}[1m])
```

### 延迟分位数

```promql
# P50 延迟
histogram_quantile(0.50, rate(distcache_request_duration_seconds_bucket[5m]))

# P95 延迟
histogram_quantile(0.95, rate(distcache_request_duration_seconds_bucket[5m]))

# P99 延迟
histogram_quantile(0.99, rate(distcache_request_duration_seconds_bucket[5m]))
```

### 热点检测效率

```promql
# 热点键晋升速率
rate(distcache_hot_keys_total{action="promoted"}[5m])

# 布隆过滤器命中率
rate(distcache_bloom_filter_queries_total{result="hit"}[5m]) / 
rate(distcache_bloom_filter_queries_total[5m])
```

## 高级用法

### 自定义指标

```go
// 获取全局 Metrics 实例
metrics := distcache.GetMetrics()

// 手动记录指标
metrics.RecordRequest("custom_method", "success")
metrics.RecordDuration("custom_method", "success", 0.123)
metrics.SetCacheSize("mygroup", 1024000)
```

### 禁用监控

```go
// 动态禁用监控（减少性能开销）
distcache.DisableMetrics()

// 检查监控状态
if distcache.IsMetricsEnabled() {
    // 执行监控相关操作
}
```

### 自定义监控服务器

```go
// 使用自定义地址启动（阻塞模式）
if err := distcache.StartMetricsServer(":8080"); err != nil {
    log.Fatal(err)
}

// 或异步启动（推荐）
server := distcache.StartMetricsServerAsync(":8080")
defer server.Stop()
```

## 性能影响

监控功能设计为低开销：

- 使用原子操作更新计数器
- 仅在启用时记录指标
- 异步 HTTP 服务器不阻塞主线程
- Prometheus 拉取模型，不主动推送

**基准测试结果**：
- 监控开启后性能下降 < 5%
- 内存增加约 1-2 MB
- 适合生产环境使用

## 示例代码

完整示例见 `examples/monitoring/main.go`:

```bash
cd examples/monitoring
go run main.go

# 访问 http://localhost:9090/status 查看监控面板
```

## 故障排查

### 端口冲突

```go
// 使用不同端口
distcache.StartMetricsServerAsync(":9091")
```

### 指标未更新

```go
// 确保已启用监控
distcache.EnableMetrics()

// 检查是否正确调用缓存操作
cache.Get("key")
```

### Prometheus 无法抓取

检查防火墙规则：
```bash
# 允许 9090 端口
sudo ufw allow 9090
```

## 最佳实践

1. **生产环境**：始终启用监控以便问题诊断
2. **报警规则**：配置 Prometheus 告警（如命中率低于 80%）
3. **长期存储**：使用 Prometheus remote storage 保存历史数据
4. **安全性**：在公网环境使用反向代理保护监控端点
5. **性能优化**：根据监控数据调整缓存大小和热点阈值

## 参考资料

- [Prometheus 文档](https://prometheus.io/docs/)
- [Grafana 文档](https://grafana.com/docs/)
- [PromQL 查询指南](https://prometheus.io/docs/prometheus/latest/querying/basics/)
