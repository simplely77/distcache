package distcache

import (
	"fmt"
	"log"
	"sync"
)

type Group struct {
	name      string
	getter    Getter
	mainCache cache
	peers    PeerPicker
}

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

var (
	mu sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string,cacheBytes int64,getter Getter)*Group{
	if getter == nil{
		panic("nil getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:name,
		getter:getter,
		mainCache:cache{cacheBytes: cacheBytes},
	}
	groups[name]=g
	return g
}

func GetGroup(name string)*Group{
	mu.RLock()
	g:=groups[name]
	mu.RUnlock()
	return g
}

// if key exists in mainCache, return it directly
// otherwise, load it from the underlying getter
func (g *Group) Get(key string)(ByteView,error){
	if key == ""{
		return ByteView{},fmt.Errorf("key is required")
	}

	if v,ok:=g.mainCache.get(key);ok{
		log.Println("[DistCache] hit")
		return v,nil
	}
	return g.load(key)
}

// RegisterPeers registers a PeerPicker for choosing remote peers
func (g *Group) RegisterPeers(peers PeerPicker){
	if g.peers!=nil{
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// load the key's value from the underlying getter
func (g *Group) load(key string)(value ByteView,err error){
	if g.peers!=nil{
		if peer,ok:=g.peers.PickPeer(key);ok{
			if value,err:=g.getFromPeer(peer,key);err==nil{
				return value,nil
			}
		}
	}
	return g.getLocally(key)
}

func (g *Group)getFromPeer(peer PeerGetter,key string)(ByteView,error){
	bytes,err:=peer.Get(g.name,key)
	if err!=nil{
		return ByteView{},err
	}
	return ByteView{b:bytes},nil
}

func (g *Group) getLocally(key string)(ByteView,error){
	bytes,err:=g.getter.Get(key)
	if err !=nil{
		return ByteView{},err
	}
	value := ByteView{b:cloneBytes(bytes)}
	g.populateCache(key,value)
	return value,nil
}

func (g *Group) populateCache(key string, value ByteView){
	g.mainCache.add(key,value)
}