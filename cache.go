package distcache

import (
	"sync"
	"github.com/simplely77/distcache/lru"
)

// cache is a thread-safe in-memory cache
type cache struct{
	mu sync.Mutex
	lru *lru.Cache
	cacheBytes int64
}

func (c *cache) add(key string,value ByteView){
	c.mu.Lock()
	defer c.mu.Unlock()
	// lazy init
	if c.lru == nil{
		c.lru = lru.New(c.cacheBytes,nil)
	}
	c.lru.Add(key,value)
}

func (c *cache) get(key string)(value ByteView,ok bool){
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil{
		return
	}
	if v,ok:=c.lru.Get(key);ok{
		return v.(ByteView),ok
	}
	return
}