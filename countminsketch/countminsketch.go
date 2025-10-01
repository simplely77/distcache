package countminsketch

import (
	"hash/fnv"
	"math"
	"sync/atomic"
)

type CountMinSketch struct {
	width  uint
	depth  uint
	table  [][]uint64
	hashes []func(string) uint32
}

func NewCountMinSketch(epsilon float64, delta float64) *CountMinSketch {
	width := uint(math.Ceil(math.E / epsilon))
	depth := uint(math.Ceil(math.Log(1 / delta)))
	table := make([][]uint64, depth)
	for i := range table {
		table[i] = make([]uint64, width)
	}
	hashes := make([]func(string) uint32, depth)
	for i := uint(0); i < depth; i++ {
		idx := i
		hashes[i] = func(key string) uint32 {
			return doubleHash(key, uint32(idx)) % uint32(width)
		}
	}
	return &CountMinSketch{
		width:  width,
		depth:  depth,
		table:  table,
		hashes: hashes,
	}
}

func (cms *CountMinSketch) Add(key string, count uint64) {
	for i := uint(0); i < cms.depth; i++ {
		idx := cms.hashes[i](key) % uint32(cms.width)
		atomic.AddUint64(&cms.table[i][idx], count)
	}
}

func (cms *CountMinSketch) Count(key string) uint64 {
	min := uint64(math.MaxUint64)
	for i := uint(0); i < cms.depth; i++ {
		idx := cms.hashes[i](key) % uint32(cms.width)
		v := atomic.LoadUint64(&cms.table[i][idx])
		if v < min {
			min = v
		}
	}
	return min
}

// Decay 将所有计数器的值减半，用于定期衰减
func (cms *CountMinSketch) Decay() {
	for i := uint(0); i < cms.depth; i++ {
		for j := uint(0); j < cms.width; j++ {
			old := atomic.LoadUint64(&cms.table[i][j])
			atomic.StoreUint64(&cms.table[i][j], old/2)
		}
	}
}

// doubleHash 使用双重哈希生成多个哈希值
func doubleHash(key string, i uint32) uint32 {
	h1 := fnv.New32a()
	h1.Write([]byte(key))
	sum1 := h1.Sum32()

	h2 := fnv.New32()
	h2.Write([]byte(key))
	sum2 := h2.Sum32()
	if sum2%2 == 0 { // 确保奇数，避免周期性
		sum2++
	}
	return sum1 + i*sum2
}
