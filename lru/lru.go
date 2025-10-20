package lru

import "container/list"

type Cache struct {
	// 最大内存
	maxBytes int64
	// 当前内存
	nbytes int64
	// 双向链表
	ll *list.List
	// 键值对映射
	cache map[string]*list.Element
	// 某条记录被移除时的回调函数，可以为 nil
	onEvicted func(key string, val Value)
}

// 双向链表节点的数据类型
type entry struct {
	key   string
	value Value
}

// Value 是缓存值的接口，允许任意类型，只要实现了 Len 方法即可
type Value interface {
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		onEvicted: onEvicted,
	}
}

// Add 添加一个键值对到缓存中
func (c *Cache) Add(key string, val Value) {
	if ele, ok := c.cache[key]; ok {
		// 如果键存在，更新值，并将该节点移到队首
		c.ll.MoveToFront(ele)
		// 通过类型断言获取 entry 对象,通过ele的Value字段获取到的是entry类型的指针
		kv := ele.Value.(*entry)
		// 更新当前使用的内存
		c.nbytes += int64(val.Len()) - int64(kv.value.Len())
		kv.value = val
	} else {
		// 如果键不存在，创建一个新的节点
		ele := c.ll.PushFront(&entry{key, val})
		// 添加到 map 中
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(val.Len())
	}
	// 如果超过了最大内存限制，则移除最久未使用的节点
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// RemoveOldest 移除最久未使用的节点
func (c *Cache) RemoveOldest() {
	// 获取链表尾部节点
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		// 删除 map 中的对应项
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 调用回调函数
		if c.onEvicted != nil {
			c.onEvicted(kv.key, kv.value)
		}
	}
}

// Get 查找键对应的值
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, ok
	}
	return
}

func (c *Cache) Remove(key string) {
	if ele, ok := c.cache[key]; ok {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.onEvicted != nil {
			c.onEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}

// NBytes 返回当前缓存使用的字节数
func (c *Cache) NBytes() int64 {
	return c.nbytes
}
