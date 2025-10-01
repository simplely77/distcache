# DistCache 使用示例

## 场景 1: 单机模式（本地缓存）

直接使用 `Group`，无需分布式通信：

```go
package main

import (
    "fmt"
    "log"
    "github.com/simplely77/distcache"
)

func main() {
    // 1. 定义数据源（通常是数据库查询）
    db := map[string]string{
        "Tom":  "630",
        "Jack": "589",
        "Sam":  "567",
    }

    // 2. 创建 Getter（缓存未命中时调用）
    getter := distcache.GetterFunc(func(key string) ([]byte, error) {
        log.Printf("从数据源加载: %s", key)
        if v, ok := db[key]; ok {
            return []byte(v), nil
        }
        return nil, fmt.Errorf("key not found")
    })

    // 3. 创建缓存组（2KB 大小限制）
    group := distcache.NewGroup("scores", 2<<10, getter)

    // 4. 使用缓存
    // 第一次 Get：缓存未命中，调用 getter
    view, err := group.Get("Tom")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Tom's score: %s\n", view.String()) // 输出: 630

    // 第二次 Get：缓存命中，不调用 getter
    view, _ = group.Get("Tom")
    fmt.Printf("Tom's score (cached): %s\n", view.String())

    // 删除缓存
    group.Delete("Tom")
    fmt.Println("缓存已删除")

    // 第三次 Get：缓存被删除，重新调用 getter
    view, _ = group.Get("Tom")
    fmt.Printf("Tom's score (reloaded): %s\n", view.String())
}
```

**运行输出：**
```
从数据源加载: Tom
Tom's score: 630
Tom's score (cached): 630
缓存已删除
从数据源加载: Tom
Tom's score (reloaded): 630
```

**特点：**
- ✅ 简单、轻量，适合单体应用
- ✅ 零网络开销，性能最优
- ✅ 256分片锁，支持高并发
- ❌ 仅本地缓存，不支持分布式

---

## 场景 2: 分布式模式（多节点集群）

使用 gRPC 搭建分布式缓存集群：

### 节点 1 - 监听 8001 端口

```go
package main

import (
    "fmt"
    "log"
    "github.com/simplely77/distcache"
)

func main() {
    // 1. 定义数据源（每个节点可以有不同的数据源）
    db := map[string]string{
        "Tom":  "630",
        "Jack": "589",
        "Sam":  "567",
    }

    getter := distcache.GetterFunc(func(key string) ([]byte, error) {
        log.Printf("[Node 8001] 从数据源加载: %s", key)
        if v, ok := db[key]; ok {
            return []byte(v), nil
        }
        return nil, fmt.Errorf("key not found")
    })

    // 2. 创建缓存组
    group := distcache.NewGroup("scores", 2<<10, getter)

    // 3. 创建 gRPC 节点（指定当前节点地址）
    addr := "localhost:8001"
    pool := distcache.NewGRPCPool(addr)

    // 4. 配置所有节点（包括自己）
    pool.SetPeers(
        "localhost:8001",  // 自己
        "localhost:8002",
        "localhost:8003",
    )

    // 5. 注册到缓存组
    group.RegisterPeers(pool)

    // 6. 启动 gRPC 服务器（阻塞）
    log.Printf("节点启动: %s", addr)
    if err := pool.Serve(addr); err != nil {
        log.Fatalf("服务启动失败: %v", err)
    }
}
```

### 节点 2 - 监听 8002 端口

```go
package main

import (
    "fmt"
    "log"
    "github.com/simplely77/distcache"
)

func main() {
    db := map[string]string{
        "Tom":  "630",
        "Jack": "589",
        "Sam":  "567",
    }

    getter := distcache.GetterFunc(func(key string) ([]byte, error) {
        log.Printf("[Node 8002] 从数据源加载: %s", key)
        if v, ok := db[key]; ok {
            return []byte(v), nil
        }
        return nil, fmt.Errorf("key not found")
    })

    group := distcache.NewGroup("scores", 2<<10, getter)

    addr := "localhost:8002"
    pool := distcache.NewGRPCPool(addr)
    
    // 配置相同的节点列表
    pool.SetPeers(
        "localhost:8001",
        "localhost:8002",  // 自己
        "localhost:8003",
    )
    
    group.RegisterPeers(pool)

    log.Printf("节点启动: %s", addr)
    if err := pool.Serve(addr); err != nil {
        log.Fatalf("服务启动失败: %v", err)
    }
}
```

### 节点 3 - 监听 8003 端口

```go
// 同节点 2，修改 addr 为 "localhost:8003"
```

### 客户端 - 访问缓存集群

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    pb "github.com/simplely77/distcache/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // 连接任意一个节点
    conn, err := grpc.Dial(
        "localhost:8001",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewCacheServiceClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 查询缓存
    resp, err := client.Get(ctx, &pb.GetRequest{
        Group: "scores",
        Key:   "Tom",
    })
    if err != nil {
        log.Fatal(err)
    }

    if resp.Found {
        fmt.Printf("Tom's score: %s\n", string(resp.Data))
    } else {
        fmt.Printf("Key not found: %s\n", resp.Err)
    }

    // 删除缓存
    delResp, err := client.Delete(ctx, &pb.DeleteRequest{
        Group: "scores",
        Key:   "Tom",
    })
    if err != nil {
        log.Fatal(err)
    }
    if delResp.Success {
        fmt.Println("缓存删除成功")
    }
}
```

**工作流程：**
```
1. 客户端请求 key="Tom" → 连接到 Node 8001
2. Node 8001 通过一致性哈希计算 → 发现应该由 Node 8002 负责
3. Node 8001 通过 gRPC 转发请求到 Node 8002
4. Node 8002 查询本地缓存：
   - 缓存命中 → 直接返回
   - 缓存未命中 → 调用 getter 加载 → 存入缓存 → 同步到2个副本节点
5. 返回结果给客户端
```

**特点：**
- ✅ 支持多节点分布式
- ✅ 一致性哈希自动路由
- ✅ 2副本机制，高可用
- ✅ gRPC/Protobuf 高性能通信
- ⚠️ 需要部署多个节点

---

## 快速启动脚本

### 方式一：启动3个节点（完整示例）

创建 `main.go`:

```go
package main

import (
    "flag"
    "fmt"
    "log"
    "github.com/simplely77/distcache"
)

var (
    port = flag.Int("port", 8001, "服务端口")
)

func main() {
    flag.Parse()

    // 数据源
    db := map[string]string{
        "Tom":  "630",
        "Jack": "589",
        "Sam":  "567",
    }

    getter := distcache.GetterFunc(func(key string) ([]byte, error) {
        log.Printf("[Node :%d] 从数据源加载: %s", *port, key)
        if v, ok := db[key]; ok {
            return []byte(v), nil
        }
        return nil, fmt.Errorf("key not found")
    })

    group := distcache.NewGroup("scores", 2<<10, getter)

    addr := fmt.Sprintf("localhost:%d", *port)
    pool := distcache.NewGRPCPool(addr)
    
    // 所有节点配置
    pool.SetPeers(
        "localhost:8001",
        "localhost:8002",
        "localhost:8003",
    )
    
    group.RegisterPeers(pool)

    log.Printf("✅ 节点启动: %s", addr)
    if err := pool.Serve(addr); err != nil {
        log.Fatalf("❌ 服务失败: %v", err)
    }
}
```

**启动命令**（开3个终端）:
```bash
# 终端 1
go run main.go -port=8001

# 终端 2
go run main.go -port=8002

# 终端 3
go run main.go -port=8003
```

### 方式二：测试客户端

创建 `client.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    pb "github.com/simplely77/distcache/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // 连接节点
    conn, err := grpc.Dial(
        "localhost:8001",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewCacheServiceClient(conn)

    // 测试 Get
    fmt.Println("=== 测试 Get ===")
    for _, key := range []string{"Tom", "Jack", "Sam"} {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        resp, err := client.Get(ctx, &pb.GetRequest{
            Group: "scores",
            Key:   key,
        })
        cancel()

        if err != nil {
            log.Printf("❌ Get(%s) 失败: %v", key, err)
            continue
        }

        if resp.Found {
            fmt.Printf("✅ %s: %s\n", key, string(resp.Data))
        } else {
            fmt.Printf("❌ %s: 未找到 (%s)\n", key, resp.Err)
        }
    }

    // 测试 Delete
    fmt.Println("\n=== 测试 Delete ===")
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    delResp, err := client.Delete(ctx, &pb.DeleteRequest{
        Group: "scores",
        Key:   "Tom",
    })
    cancel()

    if err != nil {
        log.Printf("❌ Delete 失败: %v", err)
    } else if delResp.Success {
        fmt.Println("✅ 删除 Tom 成功")
    } else {
        fmt.Printf("❌ 删除失败: %s\n", delResp.Err)
    }

    // 验证删除后重新加载
    fmt.Println("\n=== 验证重新加载 ===")
    time.Sleep(100 * time.Millisecond) // 等待删除同步
    ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
    resp, err := client.Get(ctx, &pb.GetRequest{
        Group: "scores",
        Key:   "Tom",
    })
    cancel()

    if err != nil {
        log.Printf("❌ Get(Tom) 失败: %v", err)
    } else if resp.Found {
        fmt.Printf("✅ Tom 重新加载: %s (从数据源)\n", string(resp.Data))
    }
}
```

**运行客户端**:
```bash
go run client.go
```

**预期输出**:
```
=== 测试 Get ===
✅ Tom: 630
✅ Jack: 589
✅ Sam: 567

=== 测试 Delete ===
✅ 删除 Tom 成功

=== 验证重新加载 ===
✅ Tom 重新加载: 630 (从数据源)
```

---

## 对比总结

| 特性 | 单机模式 | 分布式模式 |
|------|----------|------------|
| **使用方式** | 直接用 Group | Group + GRPCPool + gRPC客户端 |
| **复杂度** | ⭐ 简单 | ⭐⭐⭐ 复杂 |
| **性能** | ⚡ 最高（无网络） | 🚀 较高（gRPC优化） |
| **扩展性** | ❌ 单节点 | ✅ 横向扩展 |
| **容错性** | ❌ 无 | ✅ 2副本容错 |
| **数据一致性** | ✅ 强一致 | ⚠️ 最终一致 |
| **适用场景** | 单体应用、开发测试 | 微服务、大规模部署 |

---

## 架构对比图

### 单机模式
```
┌─────────────┐
│   用户请求   │
└──────┬──────┘
       │
       v
┌─────────────┐     命中 → 返回
│    Group    │ ───────────────→
│  (256分片)  │
└──────┬──────┘
       │ 未命中
       v
┌─────────────┐
│   Getter    │
│ (数据源查询) │
└─────────────┘
```

### 分布式模式
```
┌─────────────┐
│ gRPC Client │ (用户)
└──────┬──────┘
       │
       v
┌─────────────┐
│  Node 8001  │ ──[一致性哈希]──> Node 8002 (负责该key)
│   GRPCPool  │                   │
└─────────────┘                   v
                            ┌─────────────┐
                            │    Group    │ → 查缓存 → 返回
                            │  (256分片)  │
                            └──────┬──────┘
                                   │ 未命中
                                   v
                            ┌─────────────┐
                            │   Getter    │
                            │  + 副本同步  │ → Node 8001, Node 8003
                            └─────────────┘
```

---

## 核心设计理念

1. **Group 是核心**：无论单机还是分布式，都基于 Group
2. **GRPCPool 是扩展层**：可选，用于分布式通信
3. **渐进式架构**：从单机到分布式，只需添加几行代码
4. **高性能优先**：256分片锁 + gRPC/Protobuf
