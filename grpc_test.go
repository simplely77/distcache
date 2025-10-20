package distcache

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "github.com/simplely77/distcache/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 初始化测试用的缓存组
func init() {
	NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			m := map[string]string{
				"Tom":  "630",
				"Jack": "589",
				"Sam":  "567",
			}
			if v, ok := m[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

// 启动测试用的 gRPC 服务器
func startGRPCServer(t *testing.T, addr string) (*GRPCPool, func()) {
	pool := NewGRPCPool(addr)

	go func() {
		if err := pool.Serve(addr); err != nil {
			t.Logf("grpc server stopped: %v", err)
		}
	}()

	// 给服务器一点时间启动
	time.Sleep(100 * time.Millisecond)

	// 返回关闭函数
	return pool, func() {
		pool.Stop()
		time.Sleep(100 * time.Millisecond)
	}
}

// 创建 gRPC 客户端连接
func newClient(t *testing.T, addr string) (pb.CacheServiceClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial grpc: %v", err)
	}
	return pb.NewCacheServiceClient(conn), conn
}

// 测试基本的 Get 操作
func TestGRPCPool_Get(t *testing.T) {
	addr := "127.0.0.1:50051"
	_, stop := startGRPCServer(t, addr)
	defer stop()

	client, conn := newClient(t, addr)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	t.Run("Get_Existing_Key", func(t *testing.T) {
		resp, err := client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Tom",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Found {
			t.Fatalf("expected key found, got not found: %s", resp.Err)
		}
		if string(resp.Data) != "630" {
			t.Errorf("expected 630, got %s", string(resp.Data))
		}
	})

	t.Run("Get_NonExisting_Key", func(t *testing.T) {
		resp, err := client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Unknown",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Found {
			t.Fatalf("expected not found, got found")
		}
	})

	t.Run("Get_NonExisting_Group", func(t *testing.T) {
		resp, err := client.Get(ctx, &pb.GetRequest{
			Group: "notExist",
			Key:   "Tom",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Found {
			t.Fatalf("expected not found, got found")
		}
		if resp.Err == "" {
			t.Fatalf("expected error message for missing group")
		}
	})
}

// 测试 Get 和缓存行为
func TestGRPCPool_GetAndCache(t *testing.T) {
	addr := "127.0.0.1:50052"
	_, stop := startGRPCServer(t, addr)
	defer stop()

	client, conn := newClient(t, addr)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	t.Run("Get_And_Cache_Hit", func(t *testing.T) {
		// 第一次 Get，应该从 getter 加载
		resp1, err := client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Tom",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp1.Found {
			t.Fatalf("expected key found")
		}
		if string(resp1.Data) != "630" {
			t.Errorf("expected 630, got %s", string(resp1.Data))
		}

		// 第二次 Get 同一个 key，应该从缓存读取（更快）
		resp2, err := client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Tom",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp2.Found {
			t.Fatalf("expected key found in cache")
		}
		if string(resp2.Data) != "630" {
			t.Errorf("expected 630 from cache, got %s", string(resp2.Data))
		}
	})

	t.Run("Get_NonExisting_Group", func(t *testing.T) {
		resp, err := client.Get(ctx, &pb.GetRequest{
			Group: "nonexistent",
			Key:   "Tom",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Found {
			t.Fatal("expected not found for nonexistent group")
		}
	})
}

// 测试 Delete 操作
func TestGRPCPool_Delete(t *testing.T) {
	addr := "127.0.0.1:50053"
	_, stop := startGRPCServer(t, addr)
	defer stop()

	client, conn := newClient(t, addr)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	t.Run("Delete_Cached_Key", func(t *testing.T) {
		// 先 Get 一个 key，使其进入缓存
		_, err := client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Jack",
		})
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}

		// 删除缓存
		delResp, err := client.Delete(ctx, &pb.DeleteRequest{
			Group: "scores",
			Key:   "Jack",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !delResp.Success {
			t.Fatalf("delete failed: %s", delResp.Err)
		}

		// 再次 Get，应该从 getter 重新加载（因为缓存被删除了）
		getResp, err := client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Jack",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !getResp.Found {
			t.Fatalf("key should still exist in getter")
		}
		if string(getResp.Data) != "589" {
			t.Errorf("expected 589, got %s", string(getResp.Data))
		}
	})

	t.Run("Delete_NonExisting_Group", func(t *testing.T) {
		delResp, err := client.Delete(ctx, &pb.DeleteRequest{
			Group: "nonexistent",
			Key:   "Tom",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if delResp.Success {
			t.Fatal("expected delete to fail for nonexistent group")
		}
	})
}

// 测试多节点场景
func TestGRPCPool_MultiNodes(t *testing.T) {
	// 创建三个节点
	addrs := []string{
		"127.0.0.1:50061",
		"127.0.0.1:50062",
		"127.0.0.1:50063",
	}

	pools := make([]*GRPCPool, len(addrs))
	stops := make([]func(), len(addrs))

	// 启动所有节点
	for i, addr := range addrs {
		var stop func()
		pools[i], stop = startGRPCServer(t, addr)
		stops[i] = stop
		defer stops[i]()
	}

	// 设置每个节点的 peers
	for _, pool := range pools {
		pool.SetPeers(addrs...)
	}

	// 获取主节点的客户端
	client, conn := newClient(t, addrs[0])
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	t.Run("Write_And_Read_Across_Nodes", func(t *testing.T) {
		// 从第一个节点写入数据
		_, err := client.Set(ctx, &pb.SetRequest{
			Group: "scores",
			Key:   "Charlie",
			Data:  []byte("610"),
		})
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		// 给副本同步一点时间
		time.Sleep(200 * time.Millisecond)

		// 从每个节点读取数据验证
		for i, addr := range addrs {
			nodeClient, nodeConn := newClient(t, addr)
			defer nodeConn.Close()

			resp, err := nodeClient.Get(ctx, &pb.GetRequest{
				Group: "scores",
				Key:   "Charlie",
			})
			if err != nil {
				t.Fatalf("failed to get from node %d: %v", i, err)
			}
			if !resp.Found {
				t.Logf("key not found on node %d (this may be expected for non-replica nodes)", i)
				continue
			}
			if string(resp.Data) != "610" {
				t.Errorf("node %d: expected 610, got %s", i, string(resp.Data))
			}
		}
	})
}

// 测试并发操作
func TestGRPCPool_ConcurrentOperations(t *testing.T) {
	addr := "127.0.0.1:50071"
	_, stop := startGRPCServer(t, addr)
	defer stop()

	client, conn := newClient(t, addr)
	defer conn.Close()

	t.Run("Concurrent_Get", func(t *testing.T) {
		const n = 50 // 并发数
		errChan := make(chan error, n)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 使用 getter 中已存在的 keys
		keys := []string{"Tom", "Jack", "Sam"}

		for i := 0; i < n; i++ {
			go func(i int) {
				key := keys[i%len(keys)]

				// 并发读取
				resp, err := client.Get(ctx, &pb.GetRequest{
					Group: "scores",
					Key:   key,
				})
				if err != nil {
					errChan <- fmt.Errorf("failed to get key-%s: %v", key, err)
					return
				}
				if !resp.Found {
					errChan <- fmt.Errorf("key-%s: not found", key)
					return
				}
				if len(resp.Data) == 0 {
					errChan <- fmt.Errorf("key-%s: got empty data", key)
					return
				}

				errChan <- nil
			}(i)
		}

		// 等待所有操作完成
		for i := 0; i < n; i++ {
			if err := <-errChan; err != nil {
				t.Error(err)
			}
		}
	})
}

// 测试客户端接口实现
func TestGRPCClient_Interface(t *testing.T) {
	addr := "127.0.0.1:50081"
	pool, stop := startGRPCServer(t, addr)
	defer stop()

	// 设置节点
	pool.SetPeers(addr)

	// 通过 PickPeer 获取客户端
	peerClient, ok := pool.PickPeer("testkey")
	if ok {
		// 测试 PeerClient 接口的方法

		// 测试 Set
		err := peerClient.Set("scores", "test-peer-key", []byte("test-value"))
		if err != nil {
			t.Fatalf("PeerClient.Set failed: %v", err)
		}

		// 给一点时间让数据写入
		time.Sleep(100 * time.Millisecond)

		// 测试 Get
		data, err := peerClient.Get("scores", "test-peer-key")
		if err != nil {
			t.Fatalf("PeerClient.Get failed: %v", err)
		}
		if string(data) != "test-value" {
			t.Errorf("expected 'test-value', got %s", string(data))
		}

		// 测试 Delete
		err = peerClient.Delete("scores", "test-peer-key")
		if err != nil {
			t.Fatalf("PeerClient.Delete failed: %v", err)
		}

		// 验证删除成功
		_, err = peerClient.Get("scores", "test-peer-key")
		if err == nil {
			t.Error("expected error after delete, got nil")
		}
	}
}

// 基准测试
func BenchmarkGRPCPool_Get(b *testing.B) {
	addr := "127.0.0.1:50091"
	_, stop := startGRPCServer(&testing.T{}, addr)
	defer stop()

	client, conn := newClient(&testing.T{}, addr)
	defer conn.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Get(ctx, &pb.GetRequest{
			Group: "scores",
			Key:   "Tom",
		})
	}
}

func BenchmarkGRPCPool_Set(b *testing.B) {
	addr := "127.0.0.1:50092"
	_, stop := startGRPCServer(&testing.T{}, addr)
	defer stop()

	client, conn := newClient(&testing.T{}, addr)
	defer conn.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Set(ctx, &pb.SetRequest{
			Group: "scores",
			Key:   fmt.Sprintf("bench-key-%d", i),
			Data:  []byte("bench-value"),
		})
	}
}
