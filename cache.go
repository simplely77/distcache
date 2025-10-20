package distcache

import (
	"hash/fnv"
	"sync"
	"time"

	"github.com/simplely77/distcache/lru"
)

const shardCount = 256

type cacheShard struct {
	mu  sync.Mutex
	lru *lru.Cache
}

type cache struct {
	shards      [shardCount]*cacheShard
	cacheBytes  int64
	hotDetector *HotKeyDetector
}

func newCache(cacheBytes int64, hotThreshold uint64, decayInterval time.Duration) *cache {
	c := &cache{
		cacheBytes:  cacheBytes,
		hotDetector: NewHotKeyDetector(hotThreshold, decayInterval),
	}

	perBytes := cacheBytes / shardCount
	for i := 0; i < shardCount; i++ {
		c.shards[i] = &cacheShard{lru: lru.New(perBytes, nil)}
	}

	return c
}

func (c *cache) getShard(key string) *cacheShard {
	h := fnv.New32()
	h.Write([]byte(key))
	idx := h.Sum32() % shardCount
	return c.shards[idx]
}

// add 将一个键值对添加到缓存中，就是在 lru 的基础上加了锁
func (c *cache) add(key string, value ByteView) {
	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.lru.Add(key, value)

	c.hotDetector.RecordKey(key, value)
}

// get 从缓存中获取一个键对应的值
func (c *cache) get(key string) (value ByteView, ok bool) {
	// 先检查是否为热点key
	if v, found := c.hotDetector.GetHot(key); found {
		if IsMetricsEnabled() {
			GetMetrics().RecordHit("hot")
			incrementTotalHits()
			incrementHotKeyHits()
		}
		return v, true
	}

	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if shard.lru == nil {
		return
	}
	if v, found := shard.lru.Get(key); found {
		value = v.(ByteView)
		ok = true
		go c.hotDetector.RecordKey(key, value)
		if IsMetricsEnabled() {
			GetMetrics().RecordHit("local")
			incrementTotalHits()
		}
	}
	return
}

func (c *cache) delete(key string) {
	// 删除分片
	shard := c.getShard(key)
	shard.mu.Lock()
	if shard.lru != nil {
		shard.lru.Remove(key)
	}
	shard.mu.Unlock()

	// 删除热点
	c.hotDetector.hotKeys.Delete(key)
}
