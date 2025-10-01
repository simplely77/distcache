package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func([]byte) uint32

// 一致性哈希算法的实现，用于分布式缓存中的节点选择
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

func (m *Map) GetN(key string, n int) []string {
	if len(m.keys) == 0 || n <= 0 {
		return nil
	}

	// 安全限制：n 不能超过实际节点数
	totalNodes := len(m.hashMap)
	if n > totalNodes {
		n = totalNodes
	}

	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	result := make([]string, 0, n)
	// 由于每个节点对应多个物理节点，所以需要用一个集合来去重
	seen := make(map[string]struct{}, n)

	// 遍历 m.keys 一圈即可，避免无限循环
	for i := 0; len(result) < n && i < len(m.keys); i++ {
		node := m.hashMap[m.keys[(idx+i)%len(m.keys)]]
		if _, exists := seen[node]; !exists {
			result = append(result, node)
			seen[node] = struct{}{}
		}
	}

	return result
}
