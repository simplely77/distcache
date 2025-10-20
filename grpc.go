package distcache

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/simplely77/distcache/consistenthash"
	pb "github.com/simplely77/distcache/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultGRPCReplicas     = 50
	defaultReplicaNodeCount = 2
)

type GRPCPool struct {
	self        string
	mu          sync.Mutex
	peers       *consistenthash.Map
	grpcClients map[string]*grpcClient
	// 作为 gRPC 服务器的实例，与http不同的是，grpc 服务器需要注册服务
	server *grpc.Server
	pb.UnimplementedCacheServiceServer
}

func NewGRPCPool(self string) *GRPCPool {
	pool := &GRPCPool{
		self:        self,
		grpcClients: make(map[string]*grpcClient),
	}
	// 创建 gRPC 服务器实例
	pool.server = grpc.NewServer()
	pb.RegisterCacheServiceServer(pool.server, pool)
	return pool
}

func (p *GRPCPool) Log(format string, v ...interface{}) {
	if IsLoggingEnabled() {
		log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
	}
}

// Set 处理gRPC Set请求 - 仅用于副本同步，不对外开放
// 注意：这是内部方法，用于节点间同步副本数据
func (p *GRPCPool) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	p.Log("grpc Set (replica sync) %s %s", req.Group, req.Key)

	group := GetGroup(req.Group)
	if group == nil {
		return &pb.SetResponse{
			Success: false,
			Err:     "no such group: " + req.Group,
		}, nil
	}

	// 直接写入本地缓存，不再触发副本同步（避免循环）
	group.setCache(req.Key, ByteView{b: req.Data})

	return &pb.SetResponse{Success: true}, nil
}

// Delete 删除本地缓存并同步副本
func (p *GRPCPool) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	p.Log("grpc Delete %s %s", req.Group, req.Key)

	group := GetGroup(req.Group)
	if group == nil {
		return &pb.DeleteResponse{
			Success: false,
			Err:     "no such group: " + req.Group,
		}, nil
	}

	// 删除本地缓存
	group.Delete(req.Key)

	// 异步删除副本
	for _, peer := range p.ReplicaPeersForKey(req.Key) {
		go func(pg PeerClient) {
			if err := pg.Delete(req.Group, req.Key); err != nil {
				p.Log("replica Delete error: %v", err)
			}
		}(peer)
	}

	return &pb.DeleteResponse{Success: true}, nil
}

// Get 处理gRPC Get请求
func (p *GRPCPool) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	p.Log("grpc Get %s %s", req.Group, req.Key)

	group := GetGroup(req.Group)
	if group == nil {
		return &pb.GetResponse{
			Found: false,
			Err:   "no such group: " + req.Group,
		}, nil
	}

	view, err := group.Get(req.Key)
	if err != nil {
		return &pb.GetResponse{
			Found: false,
			Err:   err.Error(),
		}, nil
	}

	return &pb.GetResponse{
		Found: true,
		Data:  view.ByteSlice(),
	}, nil
}

// 启动 gRPC 服务器
func (p *GRPCPool) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	p.Log("gRPC server listening on %s", addr)
	return p.server.Serve(lis)
}

// 关闭 gRPC 服务器
func (p *GRPCPool) Stop() {
	if p.server != nil {
		p.server.GracefulStop()
	}
}

func (p *GRPCPool) SetPeers(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultGRPCReplicas, nil)
	p.peers.Add(peers...)
	p.grpcClients = make(map[string]*grpcClient, len(peers))
	for _, peer := range peers {
		p.grpcClients[peer] = &grpcClient{
			addr: peer,
		}
	}
}

// 实现 PeerPicker 接口
func (p *GRPCPool) PickPeer(key string) (PeerClient, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.grpcClients[peer], true
	}
	return nil, false
}

// 实现 PeerPicker 接口
func (p *GRPCPool) ReplicaPeersForKey(key string) []PeerClient {
	var peers []PeerClient
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.peers == nil {
		return peers
	}
	// 获取所有副本节点
	replicaKeys := p.peers.GetN(key, defaultReplicaNodeCount+1)
	for _, peer := range replicaKeys {
		if peer != p.self {
			if client, ok := p.grpcClients[peer]; ok {
				peers = append(peers, client)
			}
		}
	}
	return peers
}

// client字段，用于复用连接，所以需要实现getClient和Close()方法
type grpcClient struct {
	addr   string
	client pb.CacheServiceClient
	conn   *grpc.ClientConn
	// 确保连接的创建是线程安全的
	mu sync.RWMutex
}

func (g *grpcClient) Get(group string, key string) ([]byte, error) {
	client, err := g.getClient()
	if err != nil {
		return nil, err
	}
	// 设置请求的超时时间，防止请求阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetRequest{
		Group: group,
		Key:   key,
	}

	resp, err := client.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	if !resp.Found {
		return nil, fmt.Errorf("key not found: %s", resp.Err)
	}

	return resp.Data, nil
}

// Set 实现PeerClient接口
func (g *grpcClient) Set(group string, key string, value []byte) error {
	client, err := g.getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.SetRequest{
		Group: group,
		Key:   key,
		Data:  value,
	}

	resp, err := client.Set(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("set failed: %s", resp.Err)
	}

	return nil
}

// Delete 实现PeerClient接口
func (g *grpcClient) Delete(group string, key string) error {
	client, err := g.getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.DeleteRequest{
		Group: group,
		Key:   key,
	}

	resp, err := client.Delete(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("delete failed: %s", resp.Err)
	}

	return nil
}

func (g *grpcClient) getClient() (pb.CacheServiceClient, error) {
	// 双重锁机制，第一次读锁检查是否已经建立连接，如果有则直接返回
	g.mu.RLock()
	if g.client != nil {
		defer g.mu.RUnlock()
		return g.client, nil
	}
	g.mu.RUnlock()

	// 第二次写锁，确保只有一个协程创建连接
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.client != nil {
		return g.client, nil
	}

	conn, err := grpc.Dial(
		g.addr,
		// 明文传输
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", g.addr, err)
	}

	g.conn = conn
	g.client = pb.NewCacheServiceClient(conn)

	return g.client, nil
}

// Close 关闭连接，测试时使用
func (g *grpcClient) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}

var _ PeerClient = (*grpcClient)(nil)
var _ PeerPicker = (*GRPCPool)(nil)
