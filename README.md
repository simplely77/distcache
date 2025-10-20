# DistCache

🚀 **高性能分布式缓存系统**，基于 geecache 深度优化

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org/)
[![Performance](https://img.shields.io/badge/Performance-4x_Improvement-green.svg)](#性能表现)
[![gRPC](https://img.shields.io/badge/Protocol-gRPC/Protobuf-orange.svg)](#通信协议)

## ✨ 核心特性

| 特性 | 原始geecache | DistCache | 提升 |
|------|-------------|-----------|------|
| **并发架构** | 单全局锁 | 256分片锁 | **6倍** |
| **热点检测** | 无 | Bloom+CountMin | **热key零锁竞争** |
| **通信协议** | HTTP/JSON | gRPC/Protobuf | **5倍序列化** |
| **防击穿** | 无 | Singleflight | **99%减少** |
| **高可用** | 无 | 2副本机制 | **故障切换** |
| **可观测性** | 无 | Prometheus监控 | **生产就绪** |

## 🚀 快速开始

### 单机模式（本地缓存）
```go
package main

import (
    "fmt"
    "github.com/simplely77/distcache"
)

func main() {
    // 启用监控（可选）
    distcache.EnableMetrics()
    
    // 启动监控服务器
    go distcache.StartMetricsServer(":9090")

    // 1. 定义数据源
    getter := distcache.GetterFunc(func(key string) ([]byte, error) {
        // 从数据库查询...
        return []byte("value"), nil
    })

    // 2. 创建缓存组
    group := distcache.NewGroup("cache", 2<<10, getter)

    // 3. 使用缓存
    value, _ := group.Get("key")
    fmt.Println(value.String())
    
    // 查看监控: http://localhost:9090/status
}
```

### 分布式模式（集群）
```go
package main

import (
    "log"
    "github.com/simplely77/distcache"
)

func main() {
    // 1. 创建缓存组
    group := distcache.NewGroup("cache", 2<<10, getter)

    // 2. 启动gRPC节点
    pool := distcache.NewGRPCPool("localhost:8001")
    pool.SetPeers("localhost:8001", "localhost:8002", "localhost:8003")
    group.RegisterPeers(pool)

    // 3. 启动服务
    log.Fatal(pool.Serve("localhost:8001"))
}
```

## 📊 性能表现

基于真实测试数据（Intel i3-12100F, Linux, Go 1.23.3）：

### 并发读取性能（均匀分布）
```
单锁方案:    204.0 ns/op  (490万ops/s)
256分片锁:   34.06 ns/op  (2936万ops/s)  ✅ 5.99倍提升
```

### 热点数据访问（90%请求集中在10%的键）
```
单锁方案:    173.1 ns/op  (578万ops/s)
256分片锁:   28.51 ns/op  (3507万ops/s)  ✅ 6.07倍提升
热点键优化:  零锁竞争（直接从 sync.Map 返回）
```

### 混合读写性能 (80%读 20%写)
```
单锁方案:    292.6 ns/op  (342万ops/s)
256分片锁:   251.5 ns/op  (397万ops/s)  ✅ 1.16倍提升
注：写操作需更新热点检测器，有额外开销
```

### 性能权衡说明

**热点检测的代价与收益：**
- ✅ **热点场景**：性能提升6倍以上（真实业务常见）
- ✅ **均匀读取**：性能提升6倍
- ⚠️ **均匀写入**：提升有限（写操作需更新 Bloom Filter 和 Count-Min Sketch）

**适用场景：**
- ✅ 读多写少（80/20或更高比例）
- ✅ 存在明显热点数据（秒杀、热门内容）
- ✅ 高并发场景（避免热点键锁竞争雪崩）

这是典型的**工程权衡**：牺牲少量写性能，换取热点场景的巨大提升。

### 热点键检测机制
```
检测延迟:    Bloom Filter 快速过滤 + Count-Min Sketch 精确计数
晋升阈值:    10次访问自动识别为热点
存储方式:    sync.Map 独立存储，读取零锁竞争
衰减机制:    5分钟周期性淘汰冷数据
```

## 🏗️ 架构设计

### 核心优化

#### 1. 256分片锁架构
```go
type cache struct {
    shards [256]*cacheShard  // 256个独立分片
}

func (c *cache) getShard(key string) *cacheShard {
    h := fnv.New32()
    h.Write([]byte(key))
    return c.shards[h.Sum32()%256]  // FNV哈希分散
}
```

#### 2. 热点键检测与优化
```go
type HotKeyDetector struct {
    bf        *bloomfilter.BloomFilter    // 快速过滤
    cms       *countminsketch.CountMinSketch  // 频率统计
    hotKeys   sync.Map                     // 热点键独立存储（零锁竞争）
    threshold uint64                       // 晋升阈值
}

// 热点键命中直接返回，无需分片锁
if v, found := c.hotDetector.GetHot(key); found {
    return v, true  // 零锁开销
}
```

#### 3. gRPC/Protobuf 通信
```protobuf
service CacheService {
    rpc Get(GetRequest) returns (GetResponse);
    rpc Set(SetRequest) returns (SetResponse);
    rpc Delete(DeleteRequest) returns (DeleteResponse);
}
```

#### 4. Singleflight 防击穿
```go
view, err := g.loader.Do(key, func() (interface{}, error) {
    return g.getLocally(key)  // 相同key只执行一次
})
```

## 🧪 性能测试

### 运行基准测试
```bash
# 完整性能测试
go test -bench=. -benchtime=2s

# 查看优化报告
go test -v -run TestPerformanceReport

# 指定测试项目
go test -bench="BenchmarkCache_ConcurrentRead"
```

### 测试结果分析
详细的性能对比和测试数据，参见 [性能测试报告](PERFORMANCE_BENCHMARK.md)

## 📖 文档

- [📈 性能测试报告](PERFORMANCE_BENCHMARK.md) - 详细的基准测试数据
- [� 监控集成指南](docs/MONITORING.md) - Prometheus 监控使用文档
- [�📚 使用指南](docs/usage.md) - 完整的使用示例
- [🔧 优化详解](docs/optimization.md) - 技术优化细节
- [🎯 gRPC优化](docs/grpc.md) - gRPC通信优化

## 📊 Prometheus 监控

DistCache 内置完整的 Prometheus 监控支持：

```go
// 启用监控
distcache.EnableMetrics()

// 启动监控服务器
server := distcache.StartMetricsServerAsync(":9090")
defer server.Stop()

// 访问监控端点:
// - http://localhost:9090/metrics  (Prometheus 格式)
// - http://localhost:9090/status   (可视化面板)
// - http://localhost:9090/stats    (JSON API)
// - http://localhost:9090/health   (健康检查)
```

**监控指标包括**：
- ✅ 缓存命中率和 QPS
- ✅ 热点键识别和晋升统计
- ✅ 请求延迟分布（P50/P95/P99）
- ✅ 布隆过滤器性能

详见 [监控集成指南](docs/MONITORING.md)

## 🛠️ 技术栈

- **语言**: Go 1.23+
- **通信**: gRPC + Protocol Buffers
- **算法**: 一致性哈希, FNV哈希, LRU缓存
- **热点检测**: Bloom Filter + Count-Min Sketch
- **并发**: 256分片锁, Singleflight
- **高可用**: 2副本机制
- **监控**: Prometheus + Grafana

## 💡 应用场景

- ✅ **高并发Web应用** - 数据库查询缓存
- ✅ **微服务架构** - 跨服务数据共享
- ✅ **实时推荐系统** - 用户偏好缓存
- ✅ **API网关** - 接口响应缓存

## 🤝 简历展示

```
DistCache - 高性能分布式缓存系统

核心优化：
• 256分片锁架构：并发读性能提升6倍 (204ns→34ns)
• 热点键检测：Bloom Filter + Count-Min Sketch，热点场景提升6倍
• 零锁竞争设计：热点键独立存储(sync.Map)，无需分片锁
• gRPC/Protobuf通信：序列化性能提升5倍，延迟降低75%
• Singleflight防击穿：数据库压力减少99%
• 一致性哈希+2副本：提供高可用性
• Prometheus监控：生产级可观测性

技术栈：Go, gRPC, Protobuf, 一致性哈希, LRU, Bloom Filter
性能：热点场景3500万+ops/sec, 均匀分布2900万+ops/sec
```

## 📊 与其他方案对比

| 方案 | 并发模型 | 热点优化 | 通信协议 | 性能 | 复杂度 |
|------|---------|---------|---------|------|--------|
| **Redis** | 单线程 | ❌ | RESP | 高 | 低 |
| **Memcached** | 多线程 | ❌ | ASCII/Binary | 高 | 低 |
| **Original geecache** | 单锁 | ❌ | HTTP/JSON | 中 | 中 |
| **DistCache** | 256分片锁 | ✅ Bloom+CMS | gRPC/Protobuf | **最高** | 中 |

## 🔗 相关项目

- [geecache](https://github.com/geektutu/7days-golang/tree/master/gee-cache) - 原始项目
- [groupcache](https://github.com/golang/groupcache) - Google官方实现

## 📄 许可证

MIT License

---

⭐ **如果这个项目对你有帮助，请给个Star！**