# 热点数据检测与优化实现方案

## 🎯 核心思路

热点数据优化需要分为两个阶段：
1. **检测阶段**：统计访问频率，识别热点key
2. **优化阶段**：将热点数据提升到无锁结构

## 📊 方案一：简单计数器法

### 数据结构设计
```go
type HotKeyDetector struct {
    accessCount map[string]*int64    // 访问计数
    mu          sync.RWMutex         // 保护计数器
    threshold   int64                // 热点阈值
    checkPeriod time.Duration        // 检查周期
}

type cache struct {
    shards     [shardCount]*cacheShard
    cacheBytes int64
    
    // 热点检测器
    detector   *HotKeyDetector
    hotKeys    sync.Map            // 热点数据存储
    hotKeySet  map[string]bool     // 热点key集合
    hotMu      sync.RWMutex        // 保护热点key集合
}
```

### 访问计数逻辑
```go
func (c *cache) get(key string) (value ByteView, ok bool) {
    // 1. 先检查是否为热点数据
    if c.isHotKey(key) {
        if v, found := c.hotKeys.Load(key); found {
            c.recordAccess(key) // 记录访问
            return v.(ByteView), true
        }
    }
    
    // 2. 从普通缓存获取
    shard := c.getShard(key)
    shard.mu.Lock()
    defer shard.mu.Unlock()
    
    if shard.lru == nil {
        return
    }
    
    if v, found := shard.lru.Get(key); found {
        value = v.(ByteView)
        ok = true
        
        // 3. 记录访问并检查是否应该提升为热点
        go c.recordAccessAndCheck(key, value)
    }
    return
}

func (c *cache) recordAccessAndCheck(key string, value ByteView) {
    count := c.detector.recordAccess(key)
    
    // 如果访问次数超过阈值，提升为热点
    if count >= c.detector.threshold {
        c.promoteToHot(key, value)
    }
}
```

### 热点提升机制
```go
func (c *cache) promoteToHot(key string, value ByteView) {
    c.hotMu.Lock()
    defer c.hotMu.Unlock()
    
    // 避免重复提升
    if c.hotKeySet[key] {
        return
    }
    
    // 1. 添加到热点存储
    c.hotKeys.Store(key, value)
    c.hotKeySet[key] = true
    
    // 2. 从普通缓存中删除（可选，节省内存）
    shard := c.getShard(key)
    shard.mu.Lock()
    shard.lru.Remove(key)
    shard.mu.Unlock()
    
    log.Printf("Key %s promoted to hot cache", key)
}
```

---

## 📊 方案二：滑动窗口法（推荐）

### 更精确的频率检测
```go
type TimeWindow struct {
    count     int64
    timestamp int64
}

type SlidingWindowDetector struct {
    windows    map[string][]*TimeWindow  // 每个key的时间窗口
    mu         sync.RWMutex
    windowSize time.Duration             // 窗口大小（如5分钟）
    threshold  int64                     // 阈值（如100次/5分钟）
}

func (d *SlidingWindowDetector) recordAccess(key string) int64 {
    now := time.Now().Unix()
    
    d.mu.Lock()
    defer d.mu.Unlock()
    
    windows := d.windows[key]
    if windows == nil {
        windows = make([]*TimeWindow, 0)
    }
    
    // 清理过期窗口
    cutoff := now - int64(d.windowSize.Seconds())
    validWindows := make([]*TimeWindow, 0)
    var totalCount int64
    
    for _, w := range windows {
        if w.timestamp >= cutoff {
            validWindows = append(validWindows, w)
            totalCount += w.count
        }
    }
    
    // 添加当前访问
    if len(validWindows) > 0 && validWindows[len(validWindows)-1].timestamp == now {
        validWindows[len(validWindows)-1].count++
        totalCount++
    } else {
        validWindows = append(validWindows, &TimeWindow{count: 1, timestamp: now})
        totalCount++
    }
    
    d.windows[key] = validWindows
    return totalCount
}
```

---

## 📊 方案三：LFU + 布隆过滤器（高效）

### 内存友好的检测方案
```go
type BloomBasedDetector struct {
    // 第一级：布隆过滤器快速过滤
    bloom1 *bloom.BloomFilter
    bloom2 *bloom.BloomFilter
    
    // 第二级：精确计数
    counters map[string]*int64
    mu       sync.RWMutex
    
    threshold int64
    epoch     int64  // 时间纪元，用于定期重置
}

func (d *BloomBasedDetector) recordAccess(key string) bool {
    // 1. 布隆过滤器检查
    if !d.bloom1.TestAndAdd([]byte(key)) {
        // 第一次访问，加入第一级过滤器
        return false
    }
    
    if !d.bloom2.TestAndAdd([]byte(key)) {
        // 第二次访问，加入第二级过滤器
        return false
    }
    
    // 2. 多次访问，进行精确计数
    d.mu.Lock()
    defer d.mu.Unlock()
    
    if d.counters[key] == nil {
        count := int64(3) // 布隆过滤器已经过滤了2次
        d.counters[key] = &count
    } else {
        atomic.AddInt64(d.counters[key], 1)
    }
    
    return *d.counters[key] >= d.threshold
}
```

---

## 🎯 完整实现示例

让我为你当前的cache结构添加热点检测：

```go
package distcache

import (
    "hash/fnv"
    "sync"
    "sync/atomic"
    "time"
    "log"
    "github.com/simplely77/distcache/lru"
)

const (
    shardCount = 256
    hotThreshold = 10        // 10次访问后提升为热点
    maxHotKeys = 1000       // 最大热点key数量
    checkInterval = 30 * time.Second  // 检查间隔
)

type HotKeyDetector struct {
    accessCount sync.Map    // key -> *int64
    threshold   int64
    maxHotKeys  int
}

type cache struct {
    shards     [shardCount]*cacheShard
    cacheBytes int64
    
    // 热点优化
    detector   *HotKeyDetector
    hotKeys    sync.Map         // 热点数据无锁存储
    hotKeySet  sync.Map         // 热点key标记
    hotCount   int64            // 热点key计数
}

func newCache(cacheBytes int64) *cache {
    c := &cache{
        cacheBytes: cacheBytes,
        detector: &HotKeyDetector{
            threshold:  hotThreshold,
            maxHotKeys: maxHotKeys,
        },
    }
    
    perBytes := cacheBytes / shardCount
    for i := 0; i < shardCount; i++ {
        c.shards[i] = &cacheShard{lru: lru.New(perBytes, nil)}
    }
    
    // 启动热点检测协程
    go c.hotKeyMaintenance()
    
    return c
}

func (c *cache) get(key string) (value ByteView, ok bool) {
    // 1. 优先检查热点缓存
    if _, isHot := c.hotKeySet.Load(key); isHot {
        if v, found := c.hotKeys.Load(key); found {
            c.recordAccess(key)
            return v.(ByteView), true
        }
    }
    
    // 2. 从分片缓存获取
    shard := c.getShard(key)
    shard.mu.Lock()
    defer shard.mu.Unlock()
    
    if shard.lru == nil {
        return
    }
    
    if v, found := shard.lru.Get(key); found {
        value = v.(ByteView)
        ok = true
        
        // 3. 记录访问并检查热点提升
        if c.recordAccess(key) {
            go c.promoteToHot(key, value)
        }
    }
    return
}

func (c *cache) recordAccess(key string) bool {
    // 原子增加访问计数
    countPtr, _ := c.detector.accessCount.LoadOrStore(key, new(int64))
    count := atomic.AddInt64(countPtr.(*int64), 1)
    
    return count >= c.detector.threshold
}

func (c *cache) promoteToHot(key string, value ByteView) {
    // 检查是否已经是热点
    if _, exists := c.hotKeySet.Load(key); exists {
        return
    }
    
    // 检查热点数量限制
    if atomic.LoadInt64(&c.hotCount) >= int64(c.detector.maxHotKeys) {
        return
    }
    
    // 提升为热点
    c.hotKeys.Store(key, value)
    c.hotKeySet.Store(key, true)
    atomic.AddInt64(&c.hotCount, 1)
    
    log.Printf("[HotKey] Promoted key: %s (count: %d)", key, c.getAccessCount(key))
}

func (c *cache) getAccessCount(key string) int64 {
    if countPtr, ok := c.detector.accessCount.Load(key); ok {
        return atomic.LoadInt64(countPtr.(*int64))
    }
    return 0
}

// 热点维护协程
func (c *cache) hotKeyMaintenance() {
    ticker := time.NewTicker(checkInterval)
    defer ticker.Stop()
    
    for range ticker.C {
        c.cleanupAccessCounts()
        c.rebalanceHotKeys()
    }
}

func (c *cache) cleanupAccessCounts() {
    // 定期清理低频访问的计数，防止内存泄漏
    c.detector.accessCount.Range(func(key, value interface{}) bool {
        count := atomic.LoadInt64(value.(*int64))
        if count < c.detector.threshold/2 {
            c.detector.accessCount.Delete(key)
        }
        return true
    })
}

func (c *cache) rebalanceHotKeys() {
    // 检查热点key是否还热，如果不热了就降级
    c.hotKeySet.Range(func(key, value interface{}) bool {
        keyStr := key.(string)
        recentCount := c.getAccessCount(keyStr)
        
        // 如果最近访问很少，降级
        if recentCount < c.detector.threshold/4 {
            c.demoteFromHot(keyStr)
        }
        return true
    })
}

func (c *cache) demoteFromHot(key string) {
    if v, ok := c.hotKeys.Load(key); ok {
        // 移回普通缓存
        c.add(key, v.(ByteView))
        
        // 从热点缓存删除
        c.hotKeys.Delete(key)
        c.hotKeySet.Delete(key)
        atomic.AddInt64(&c.hotCount, -1)
        
        log.Printf("[HotKey] Demoted key: %s", key)
    }
}
```

---

## 🎯 关键设计点

### 1. **渐进式检测**
- 使用原子操作记录访问次数
- 避免每次访问都加锁
- 异步提升，不影响读取性能

### 2. **内存控制**
- 限制最大热点key数量
- 定期清理低频计数器
- 热点降级机制

### 3. **性能优化**
- 热点数据用`sync.Map`无锁访问
- 检测和提升异步进行
- 最小化锁竞争

### 4. **可观测性**
- 记录提升/降级日志
- 提供访问计数查询
- 便于调试和优化

## 💡 使用效果

```go
// 第1-9次访问：正常分片缓存
cache.get("hot_key")  // 分片锁 + LRU

// 第10次访问：自动提升为热点
cache.get("hot_key")  // 提升为热点，后续无锁访问

// 后续访问：无锁高性能
cache.get("hot_key")  // sync.Map，无锁并发
```

这样既保持了现有架构的稳定性，又为真正的热点数据提供了额外的性能提升！🚀