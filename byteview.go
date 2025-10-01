package distcache

// ByteView 将lru包中的Value接口实现为只读的字节切片，防止外部修改
type ByteView struct{
	b []byte
}

func (v ByteView) Len() int{
	return len(v.b)
}

func (v ByteView) ByteSlice() []byte{
	return cloneBytes(v.b)
}

func (v ByteView) String()string{
	return string(v.b)
}

func cloneBytes(b []byte)[]byte{
	c := make([]byte,len(b))
	copy(c,b)
	return c
}