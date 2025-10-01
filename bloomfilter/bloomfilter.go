package bloomfilter

import (
	"hash/fnv"
	"sync"
)

type BloomFilter struct {
    bits  []uint64
    k     uint       // hash函数个数
    m     uint       // 位数组长度
    mutex sync.Mutex // 并发安全
}

func NewBloomFilter(size uint, hashes uint) *BloomFilter {
    return &BloomFilter{
        bits: make([]uint64, (size+63)/64),
        k:    hashes,
        m:    size,
    }
}

func (bf *BloomFilter) Add(key string) {
    bf.mutex.Lock()
    defer bf.mutex.Unlock()
    for i := uint(0); i < bf.k; i++ {
        idx := bf.hash(key, i) % bf.m
        bf.bits[idx/64] |= 1 << (idx % 64)
    }
}

func (bf *BloomFilter) Test(key string) bool {
    bf.mutex.Lock()
    defer bf.mutex.Unlock()
    for i := uint(0); i < bf.k; i++ {
        idx := bf.hash(key, i) % bf.m
        if bf.bits[idx/64]&(1<<(idx%64)) == 0 {
            return false
        }
    }
    return true
}

func (bf *BloomFilter) hash(key string, i uint) uint {
    // 使用双重哈希避免聚集
    h1 := fnv.New32a()
    h1.Write([]byte(key))
    hash1 := uint(h1.Sum32())
    
    h2 := fnv.New32()
    h2.Write([]byte(key))
    hash2 := uint(h2.Sum32())
    
    // 确保hash2是奇数，避免周期性
    if hash2%2 == 0 {
        hash2++
    }
    
    return (hash1 + i*hash2) % bf.m
}