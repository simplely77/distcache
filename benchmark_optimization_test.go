package distcache

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/simplely77/distcache/lru"
)

// ============================================
// 性能优化对比测试
// 对比点：原始单锁 vs 256分片锁
// ============================================

// 模拟原始 geecache 的单锁实现
type singleLockCache struct {
	mu    sync.Mutex
	lru   *lru.Cache
	bytes int64
}

func newSingleLockCache(cacheBytes int64) *singleLockCache {
	return &singleLockCache{
		lru:   lru.New(cacheBytes, nil),
		bytes: cacheBytes,
	}
}

func (c *singleLockCache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Add(key, value)
}

func (c *singleLockCache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, found := c.lru.Get(key); found {
		return v.(ByteView), true
	}
	return
}

// ============================================
// 优化1: 并发读写性能对比
// 原始方案：单个全局锁
// 优化方案：256个分片锁
// ============================================

func BenchmarkCache_ConcurrentRead_SingleLock(b *testing.B) {
	cache := newSingleLockCache(2 << 20)

	// 预填充1000个key
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
		cache.add(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("key-%d", rand.Intn(1000))
			cache.get(key)
		}
	})
}

func BenchmarkCache_ConcurrentRead_ShardedLock(b *testing.B) {
	cache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)

	// 预填充1000个key
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
		cache.add(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("key-%d", rand.Intn(1000))
			cache.get(key)
		}
	})
}

// ============================================
// 优化2: 并发写入性能对比
// ============================================

func BenchmarkCache_ConcurrentWrite_SingleLock(b *testing.B) {
	cache := newSingleLockCache(2 << 20)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
			cache.add(key, value)
			i++
		}
	})
}

func BenchmarkCache_ConcurrentWrite_ShardedLock(b *testing.B) {
	cache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
			cache.add(key, value)
			i++
		}
	})
}

// ============================================
// 优化3: 混合读写性能对比 (80%读 20%写)
// 更贴近真实场景
// ============================================

func BenchmarkCache_MixedReadWrite_SingleLock(b *testing.B) {
	cache := newSingleLockCache(2 << 20)

	// 预填充
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
		cache.add(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 1000
		for pb.Next() {
			if rand.Intn(100) < 80 {
				// 80% 读操作
				key := fmt.Sprintf("key-%d", rand.Intn(1000))
				cache.get(key)
			} else {
				// 20% 写操作
				key := fmt.Sprintf("key-%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
				cache.add(key, value)
				i++
			}
		}
	})
}

func BenchmarkCache_MixedReadWrite_ShardedLock(b *testing.B) {
	cache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)

	// 预填充
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
		cache.add(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 1000
		for pb.Next() {
			if rand.Intn(100) < 80 {
				// 80% 读操作
				key := fmt.Sprintf("key-%d", rand.Intn(1000))
				cache.get(key)
			} else {
				// 20% 写操作
				key := fmt.Sprintf("key-%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
				cache.add(key, value)
				i++
			}
		}
	})
}

// ============================================
// 优化4: 不同并发度下的性能对比
// 测试在不同并发度下的扩展性
// ============================================

func BenchmarkCache_Scalability(b *testing.B) {
	concurrencies := []int{1, 2, 4, 8, 16, 32, 64, 128}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("SingleLock-Goroutines-%d", concurrency), func(b *testing.B) {
			cache := newSingleLockCache(2 << 20)

			// 预填充
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key-%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
				cache.add(key, value)
			}

			b.SetParallelism(concurrency)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					key := fmt.Sprintf("key-%d", rand.Intn(1000))
					cache.get(key)
				}
			})
		})

		b.Run(fmt.Sprintf("ShardedLock-Goroutines-%d", concurrency), func(b *testing.B) {
			cache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)

			// 预填充
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key-%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
				cache.add(key, value)
			}

			b.SetParallelism(concurrency)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					key := fmt.Sprintf("key-%d", rand.Intn(1000))
					cache.get(key)
				}
			})
		})
	}
}

// ============================================
// 优化5: 锁竞争测试
// 测量锁的竞争程度
// ============================================

func TestLockContention(t *testing.T) {
	iterations := 100000
	goroutines := 100

	t.Run("SingleLock", func(t *testing.T) {
		cache := newSingleLockCache(2 << 20)

		// 预填充
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
			cache.add(key, value)
		}

		start := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/goroutines; j++ {
					key := fmt.Sprintf("key-%d", rand.Intn(100))
					cache.get(key)
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(start)

		t.Logf("SingleLock: %d goroutines, %d ops, took %v",
			goroutines, iterations, elapsed)
		t.Logf("SingleLock: %.2f ops/sec",
			float64(iterations)/elapsed.Seconds())
	})

	t.Run("ShardedLock", func(t *testing.T) {
		cache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)

		// 预填充
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
			cache.add(key, value)
		}

		start := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations/goroutines; j++ {
					key := fmt.Sprintf("key-%d", rand.Intn(100))
					cache.get(key)
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(start)

		t.Logf("ShardedLock: %d goroutines, %d ops, took %v",
			goroutines, iterations, elapsed)
		t.Logf("ShardedLock: %.2f ops/sec",
			float64(iterations)/elapsed.Seconds())
	})
}

// ============================================
// 优化6: 热点数据访问性能测试
// 测试集中访问少数热点key时的性能
// ============================================

func BenchmarkCache_HotKey_SingleLock(b *testing.B) {
	cache := newSingleLockCache(2 << 20)

	// 预填充
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
		cache.add(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 90%的请求访问前10个key（热点数据）
			var key string
			if rand.Intn(100) < 90 {
				key = fmt.Sprintf("key-%d", rand.Intn(10))
			} else {
				key = fmt.Sprintf("key-%d", rand.Intn(1000))
			}
			cache.get(key)
		}
	})
}

func BenchmarkCache_HotKey_ShardedLock(b *testing.B) {
	cache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)

	// 预填充
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
		cache.add(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 90%的请求访问前10个key（热点数据）
			var key string
			if rand.Intn(100) < 90 {
				key = fmt.Sprintf("key-%d", rand.Intn(10))
			} else {
				key = fmt.Sprintf("key-%d", rand.Intn(1000))
			}
			cache.get(key)
		}
	})
}

// ============================================
// 综合性能报告
// ============================================

func TestPerformanceReport(t *testing.T) {
	t.Log("\n=== 分片缓存性能优化报告 ===\n")

	testCases := []struct {
		name       string
		goroutines int
		operations int
	}{
		{"低并发", 10, 100000},
		{"中并发", 50, 100000},
		{"高并发", 200, 100000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 测试单锁性能
			singleCache := newSingleLockCache(2 << 20)
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key-%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
				singleCache.add(key, value)
			}

			start := time.Now()
			var wg sync.WaitGroup
			for i := 0; i < tc.goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < tc.operations/tc.goroutines; j++ {
						key := fmt.Sprintf("key-%d", rand.Intn(1000))
						singleCache.get(key)
					}
				}()
			}
			wg.Wait()
			singleTime := time.Since(start)

			// 测试分片锁性能
			shardedCache := newCache(2<<20, DefaultHotKeyThreshold, DefaultDecayInterval)
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key-%d", i)
				value := ByteView{b: []byte(fmt.Sprintf("value-%d", i))}
				shardedCache.add(key, value)
			}

			start = time.Now()
			for i := 0; i < tc.goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < tc.operations/tc.goroutines; j++ {
						key := fmt.Sprintf("key-%d", rand.Intn(1000))
						shardedCache.get(key)
					}
				}()
			}
			wg.Wait()
			shardedTime := time.Since(start)

			// 计算性能提升
			improvement := float64(singleTime-shardedTime) / float64(singleTime) * 100
			speedup := float64(singleTime) / float64(shardedTime)

			t.Logf("\n【%s场景】", tc.name)
			t.Logf("并发数: %d goroutines", tc.goroutines)
			t.Logf("操作数: %d ops", tc.operations)
			t.Logf("单锁方案: %v (%.2f ops/sec)",
				singleTime, float64(tc.operations)/singleTime.Seconds())
			t.Logf("分片锁方案: %v (%.2f ops/sec)",
				shardedTime, float64(tc.operations)/shardedTime.Seconds())
			t.Logf("性能提升: %.2f%%", improvement)
			t.Logf("加速比: %.2fx", speedup)
		})
	}
}
