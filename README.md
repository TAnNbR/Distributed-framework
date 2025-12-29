# 🎬 Go 高性能分布式框架

通过消息驱动实现高量级并发处理，支持多节点自动发现与协作。

---

## ✨ 核心特性

| 特性 | 说明 |
|-----|------|
| 🚀 **高性能** | 无锁消息队列、批量处理、VTProtobuf 序列化 |
| 🔄 **容错设计** | 自动崩溃重启、消息缓冲、可配置重启策略 |
| 🌐 **分布式** | 跨节点通信、服务自动发现（mDNS/Consul） |
| 🎯 **简洁 API** | 函数式选项、链式配置、开箱即用 |
| 🔒 **安全通信** | TLS 加密传输支持 |

---

## 🏗️ 架构概览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Distributed Framework                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │   Cluster   │    │   Remote    │    │    Actor    │                 │
│  │  分布式集群  │    │  远程通信   │    │   核心引擎   │                 │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                 │
│         │                  │                  │                         │
│         └──────────────────┼──────────────────┘                         │
│                            │                                            │
│                     ┌──────┴──────┐                                     │
│                     │   Engine    │                                     │
│                     │  Actor 引擎  │                                     │
│                     └─────────────┘                                     │
│                                                                         │
│  ┌─────────────┐    ┌─────────────┐                                    │
│  │ RingBuffer  │    │   SafeMap   │                                    │
│  │  环形队列   │    │  线程安全Map │                                    │
│  └─────────────┘    └─────────────┘                                    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 📦 模块说明

| 模块 | 路径 | 说明 |
|-----|------|------|
| **actor** | `actor/` | 核心引擎：Engine、Process、Inbox、Context |
| **remote** | `remote/` | 远程通信：dRPC、流路由、序列化 |
| **cluster** | `cluster/` | 分布式集群：Agent、Provider、成员管理 |
| **ringbuffer** | `ringbuffer/` | 泛型环形队列：自动扩容、线程安全 |
| **safemap** | `safemap/` | 泛型线程安全 Map：读写分离锁 |

---

## 🚀 快速开始

### 安装

```bash
go get github.com/TAnNbR/Distributed-framework
```

### 基础示例：创建 Actor

```go
package main

import (
    "fmt"
    "github.com/TAnNbR/Distributed-framework/actor"
)

// 定义消息
type Hello struct{ Name string }

// 定义 Actor
type Greeter struct{}

func (g *Greeter) Receive(ctx *actor.Context) {
    switch msg := ctx.Message().(type) {
    case actor.Started:
        fmt.Println("Actor 启动")
    case Hello:
        fmt.Printf("Hello, %s!\n", msg.Name)
    }
}

func NewGreeter() actor.Producer {
    return func() actor.Receiver {
        return &Greeter{}
    }
}

func main() {
    // 创建引擎
    engine, _ := actor.NewEngine(actor.NewEngineConfig())
    
    // 创建 Actor
    pid := engine.Spawn(NewGreeter(), "greeter")
    
    // 发送消息
    engine.Send(pid, Hello{Name: "World"})
}
```

### 远程通信

```go
package main

import (
    "github.com/TAnNbR/Distributed-framework/actor"
    "github.com/TAnNbR/Distributed-framework/remote"
)

func main() {
    // 节点 A
    remoteA := remote.New("127.0.0.1:4000", remote.NewConfig())
    engineA, _ := actor.NewEngine(actor.NewEngineConfig().WithRemote(remoteA))
    
    pidA := engineA.Spawn(NewGreeter(), "greeter", actor.WithID("1"))
    // PID: 127.0.0.1:4000/greeter/1
    
    // 节点 B 可以直接发送消息到节点 A
    remoteB := remote.New("127.0.0.1:5000", remote.NewConfig())
    engineB, _ := actor.NewEngine(actor.NewEngineConfig().WithRemote(remoteB))
    
    // 跨节点发送
    engineB.Send(pidA, Hello{Name: "from Node B"})
}
```

### 分布式集群

```go
package main

import (
    "github.com/TAnNbR/Distributed-framework/actor"
    "github.com/TAnNbR/Distributed-framework/cluster"
)

func main() {
    // 创建集群
    c, _ := cluster.New(cluster.NewConfig().
        WithID("node-1").
        WithListenAddr("127.0.0.1:4000").
        WithRegion("us-east"))
    
    // 注册可激活的 Actor 类型
    c.RegisterKind("player", NewPlayer, cluster.NewKindConfig())
    
    // 启动集群
    c.Start()
    
    // 在集群中激活 Actor（自动选择节点）
    playerPID := c.Activate("player", cluster.NewActivationConfig().WithID("player-1"))
    
    // 发送消息
    c.Engine().Send(playerPID, GameMessage{...})
}
```

---

## 🔧 配置选项

### Actor 配置

```go
engine.Spawn(producer, "kind",
    actor.WithID("custom-id"),           // 自定义 ID
    actor.WithMaxRestarts(5),            // 最大重启次数
    actor.WithRestartDelay(time.Second), // 重启延迟
    actor.WithInboxSize(1024),           // 收件箱大小
    actor.WithMiddleware(LoggingMW),     // 中间件
)
```

### Remote 配置

```go
remote.New(addr, remote.NewConfig().
    WithTLS(tlsConfig).                  // TLS 加密
    WithBufferSize(4*1024*1024),         // 缓冲区大小
)
```

### Cluster 配置

```go
cluster.New(cluster.NewConfig().
    WithID("node-1").                    // 节点 ID
    WithListenAddr("0.0.0.0:4000").      // 监听地址
    WithRegion("us-east").               // 区域
    WithProvider(consulProvider).         // 服务发现提供者
    WithRequestTimeout(5*time.Second),   // 请求超时
)
```

---

## 📊 性能设计

| 组件 | 技术 | 优势 |
|-----|------|------|
| 消息队列 | RingBuffer + 自动扩容 | 无锁 Push、批量 Pop |
| 调度 | CAS 状态机 | 避免锁竞争 |
| 序列化 | VTProtobuf | 无反射、5x 性能提升 |
| 网络 | dRPC | 比 gRPC 低延迟 |
| 并发控制 | RWMutex + Atomic | 读多写少优化 |

---

## 🌐 服务发现

### SelfManaged (mDNS)

适用于本地开发和同一局域网：

```go
config := cluster.NewConfig().
    WithProvider(cluster.NewSelfManagedProvider(
        cluster.NewSelfManagedConfig(),
    ))
```

### Consul

适用于生产环境：

```go
config := cluster.NewConfig().
    WithProvider(cluster.NewConsulProvider(
        cluster.NewConsulProviderConfig().
            WithAddress("consul:8500"),
    ))
```

---

## 📁 项目结构

```
hollywood/
├── actor/           # 核心 Actor 引擎
│   ├── engine.go    # Engine 实现
│   ├── process.go   # Process 生命周期
│   ├── inbox.go     # 消息队列
│   ├── context.go   # Actor 上下文
│   └── ...
├── remote/          # 远程通信模块
│   ├── remote.go    # Remote 入口
│   ├── stream_*.go  # 流管理
│   ├── serialize.go # 序列化
│   └── ...
├── cluster/         # 分布式集群
│   ├── cluster.go   # Cluster 主体
│   ├── agent.go     # Agent Actor
│   ├── selfmanaged.go # mDNS 发现
│   ├── consul_provider.go # Consul 发现
│   └── ...
├── ringbuffer/      # 环形缓冲区
├── safemap/         # 线程安全 Map
└── examples/        # 示例代码
```



