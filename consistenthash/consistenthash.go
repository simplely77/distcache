package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func([]byte) uint32

type Map struct {
	hash     Hash
	replicas int
	keys     []int
	hashMap  map[int]string
}

// New creates a Map instance
func New(replicas int, fn Hash) *Map {
	if fn == nil {
		fn = crc32.ChecksumIEEE
	}
	return &Map{
		hash:    fn,
		replicas: replicas,
		hashMap: make(map[int]string),
	}
}

// Add adds some keys to the hash.
func (m *Map) Add(keys ...string){
	for _,key:=range keys{
		for i:=0;i<m.replicas;i++{
			hash := int(m.hash([]byte(strconv.Itoa(i)+key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash]=key
		}
	}
	sort.Ints(m.keys)
}

func (m *Map) Get(key string)string{
	if len(m.keys)==0{
		return ""
	}
	hash := int(m.hash([]byte(key)))
	// binary search for appropriate replica, if none found, idx = len(m.keys)
	idx := sort.Search(len(m.keys),func(i int)bool{
		return m.keys[i]>=hash
	})
	// % len for the case when idx == len(m.keys)
	return m.hashMap[m.keys[idx%len(m.keys)]]
}