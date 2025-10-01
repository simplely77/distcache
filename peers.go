package distcache

// PeerPicker 选择远程节点的接口，提供了根据键选择节点的方法
type PeerPicker interface {
	PickPeer(key string)(peer PeerClient,ok bool)
	ReplicaPeersForKey(key string)[]PeerClient
}

// PeerClient 获取远程节点数据的接口，副本相关的方法也放在这里
type PeerClient interface {
	Get(group string,key string)([]byte,error)
	Set(group string, key string, value []byte) error
	Delete(group string,key string)error
}