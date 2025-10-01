package distcache

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/simplely77/distcache/singleflight"
)

// 热点缓存默认配置
const (
	// 默认热点检测阈值：访问次数达到10次认为是热点
	DefaultHotKeyThreshold = 10
	// 默认衰减周期：每5分钟进行一次频率衰减
	DefaultDecayInterval = 5 * time.Minute
)

// Group 是缓存的核心数据结构，负责与用户交互
type Group struct {
	name string
	// 用于缓存未命中时获取源数据
	getter Getter
	// 本地缓存
	mainCache *cache
	// 用于选择远程节点
	peers PeerPicker
	// 使每个 key 并发状况下只被请求一次
	loader *singleflight.Group
}

// Getter 用于获取源数据，可以是本地文件、数据库，或远程 API
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 函数式接口实现 Getter 接口，方便用户传入函数
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// 全局变量，存储所有创建的 Group，这是所有的本地的group的集合，不同节点的group是不同的
var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: newCache(cacheBytes, DefaultHotKeyThreshold, DefaultDecayInterval),
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// NewGroupWithHotKeyConfig 创建一个带有自定义热点缓存配置的Group
func NewGroupWithHotKeyConfig(name string, cacheBytes int64, getter Getter, hotThreshold uint64, decayInterval time.Duration) *Group {
	if getter == nil {
		panic("nil getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: newCache(cacheBytes, hotThreshold, decayInterval),
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// if key exists in mainCache, return it directly
// otherwise, load it from the underlying getter
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok {
		log.Println("[DistCache] hit")
		return v, nil
	}
	return g.load(key)
}

// set 是内部方法，用于设置缓存并同步到副本节点
// 只在从底层数据源加载数据时调用，不对外暴露
func (g *Group) set(key string, value ByteView) {
	g.mainCache.add(key, value)

	if g.peers == nil {
		return
	}
	for _, peer := range g.peers.ReplicaPeersForKey(key) {
		// 异步添加副本
		go func(p PeerClient) {
			p.Set(g.name, key, value.ByteSlice())
		}(peer)
	}
}

func (g *Group) Delete(key string) {
	g.mainCache.delete(key)

	if g.peers == nil {
		return
	}
	for _, peer := range g.peers.ReplicaPeersForKey(key) {
		// 异步删除副本
		go func(p PeerClient) {
			p.Delete(g.name, key)
		}(peer)
	}
}

// setCache 直接设置缓存，用于副本同步，不触发进一步的副本同步
func (g *Group) setCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// RegisterPeers registers a PeerPicker for choosing remote peers
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// load the key's value from the underlying getter
func (g *Group) load(key string) (value ByteView, err error) {
	view, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err := g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				// 主节点失败，读取副节点
				for _, peer := range g.peers.ReplicaPeersForKey(key) {
					if value, err := g.getFromPeer(peer, key); err == nil {
						return value, nil
					}
				}
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		return view.(ByteView), nil
	}
	return
}

func (g *Group) getFromPeer(peer PeerClient, key string) (ByteView, error) {
	// 通过 peer 获取数据
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}

func (g *Group) getLocally(key string) (ByteView, error) {
	// 从本地数据源获取数据
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	// 克隆一份数据，避免外部数据源持有对底层数组的引用
	value := ByteView{b: cloneBytes(bytes)}
	g.set(key, value)
	return value, nil
}
