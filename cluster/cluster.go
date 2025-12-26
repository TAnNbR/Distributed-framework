package cluster

import (
	fmt "fmt"
	"log/slog"
	"math"
	"math/rand"
	"reflect"
	"slices"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
	"github.com/TAnNbR/Distributed-framework/remote"
)

// 选择一个合理的超时时间，以便长距离网络的节点也能正常工作。
var defaultRequestTimeout = time.Second

// Producer 是一个函数，给定 *cluster.Cluster 返回一个 actor.Producer。
// 简单但强大的工具，用于构建依赖于 Cluster 的接收器。
type Producer func(c *Cluster) actor.Producer

// Config 保存集群配置。
type Config struct {
	listenAddr     string
	id             string
	region         string
	engine         *actor.Engine
	provider       Producer
	requestTimeout time.Duration
}

// NewConfig 返回一个用默认值初始化的 Config。
func NewConfig() Config {
	return Config{
		listenAddr:     getRandomListenAddr(),
		id:             fmt.Sprintf("%d", rand.Intn(math.MaxInt)),
		region:         "default",
		provider:       NewSelfManagedProvider(NewSelfManagedConfig()),
		requestTimeout: defaultRequestTimeout,
	}
}

// WithRequestTimeout 设置集群成员之间请求的最大持续时间。
// 默认为 1 秒，以支持不同区域节点之间的通信。
func (config Config) WithRequestTimeout(d time.Duration) Config {
	config.requestTimeout = d
	return config
}

// WithProvider 设置集群提供者。
// 默认为 SelfManagedProvider。
func (config Config) WithProvider(p Producer) Config {
	config.provider = p
	return config
}

// WithEngine 设置将用于驱动节点上运行的 actor 的内部 actor 引擎。
// 如果没有给定引擎，集群将实例化一个新的引擎和远程模块。
func (config Config) WithEngine(e *actor.Engine) Config {
	config.engine = e
	return config
}

// WithListenAddr 设置底层远程模块的监听地址。
// 默认为随机端口号。
func (config Config) WithListenAddr(addr string) Config {
	config.listenAddr = addr
	return config
}

// WithID 设置此节点的 ID。
// 默认为随机生成的 ID。
func (config Config) WithID(id string) Config {
	config.id = id
	return config
}

// WithRegion 设置成员将托管的区域。
// 默认为 "default"。
func (config Config) WithRegion(region string) Config {
	config.region = region
	return config
}

// Cluster 允许你编写分布式 actor。它结合了 Engine、Remote 和 Provider，
// 使集群成员能够在自发现环境中相互发送消息。
type Cluster struct {
	config      Config
	engine      *actor.Engine
	agentPID    *actor.PID
	providerPID *actor.PID
	isStarted   bool
	kinds       []kind
}

// New 根据给定的 Config 返回一个新的集群。
func New(config Config) (*Cluster, error) {
	if config.engine == nil {
		remote := remote.New(config.listenAddr, remote.NewConfig())
		e, err := actor.NewEngine(actor.NewEngineConfig().WithRemote(remote))
		if err != nil {
			return nil, err
		}
		config.engine = e
	}
	c := &Cluster{
		config: config,
		engine: config.engine,
		kinds:  make([]kind, 0),
	}
	return c, nil
}

// Start 启动集群。
func (c *Cluster) Start() {
	c.agentPID = c.engine.Spawn(NewAgent(c), "cluster", actor.WithID(c.config.id))
	c.providerPID = c.engine.Spawn(c.config.provider(c), "provider", actor.WithID(c.config.id))
	c.isStarted = true
}

// Stop 将关闭集群，毒化其所有 actor。
func (c *Cluster) Stop() {
	<-c.engine.Poison(c.agentPID).Done()
	<-c.engine.Poison(c.providerPID).Done()
}

// Spawn 在本地节点上创建一个具有集群感知能力的 actor。
func (c *Cluster) Spawn(p actor.Producer, id string, opts ...actor.OptFunc) *actor.PID {
	pid := c.engine.Spawn(p, id, opts...)
	members := c.Members()
	for _, member := range members {
		c.engine.Send(member.PID(), &Activation{
			PID: pid,
		})
	}
	return pid
}

// Activate 根据给定的配置在集群中激活已注册的 kind。
// 即使 actor 没有在本地成员上注册，只要至少有一个成员注册了该 kind，actor 就可以被激活。
//
//	playerPID := cluster.Activate("player", cluster.NewActivationConfig())
func (c *Cluster) Activate(kind string, config ActivationConfig) *actor.PID {
	msg := activate{
		kind:   kind,
		config: config,
	}
	resp, err := c.engine.Request(c.agentPID, msg, c.config.requestTimeout).Result()
	if err != nil {
		slog.Error("激活失败", "err", err)
		return nil
	}
	pid, ok := resp.(*actor.PID)
	if !ok {
		slog.Warn("激活期望响应类型为 *actor.PID", "got", reflect.TypeOf(resp))
		return nil
	}
	return pid
}

// Deactivate 停用给定的 PID。
func (c *Cluster) Deactivate(pid *actor.PID) {
	c.engine.Send(c.agentPID, deactivate{pid: pid})
}

// RegisterKind 注册一个可以从集群中任何成员激活的新 actor。
//
//	cluster.Register("player", NewPlayer, NewKindConfig())
//
// 注意：kind 只能在集群启动之前注册。
func (c *Cluster) RegisterKind(kind string, producer actor.Producer, config KindConfig) {
	if c.isStarted {
		slog.Warn("注册 kind 失败", "reason", "集群已启动", "kind", kind)
		return
	}
	c.kinds = append(c.kinds, newKind(kind, producer, config))
}

// HasKindLocal 返回集群节点是否在本地注册了该 kind。
func (c *Cluster) HasKindLocal(name string) bool {
	for _, kind := range c.kinds {
		if kind.name == name {
			return true
		}
	}
	return false
}

// Members 返回集群中的所有成员。
func (c *Cluster) Members() []*Member {
	resp, err := c.engine.Request(c.agentPID, getMembers{}, c.config.requestTimeout).Result()
	if err != nil {
		return []*Member{}
	}
	if res, ok := resp.([]*Member); ok {
		return res
	}
	return nil
}

// HasKind 返回给定的 kind 是否可在集群上激活。
func (c *Cluster) HasKind(name string) bool {
	resp, err := c.engine.Request(c.agentPID, getKinds{}, c.config.requestTimeout).Result()
	if err != nil {
		return false
	}
	if kinds, ok := resp.([]string); ok {
		return slices.Contains(kinds, name)
	}
	return false
}

// GetActiveByKind 返回集群中按给定 kind 激活的所有 actor PID。
// 如果在集群上找不到该 kind，将返回一个包含 nil pid 的切片。
//
// 为什么？
// 这保证了函数将返回一个至少包含 1 个元素的切片。
// nil pid 可以盲目地用于发送任何消息，这些消息最终会进入死信。
// 高可用且正确的代码来处理完全容错系统应该如下所示：
//
// playerPids := cluster.GetActiveByKind("player")
//
//	for _, pid := range playerPids {
//	 engine.Send(pid, theMessage)
//	}
//
// engine.Send(playerPid)
//
// playerPids := c.GetActiveByKind("player")
// [127.0.0.1:34364/player/1 127.0.0.1:34365/player/2]
func (c *Cluster) GetActiveByKind(kind string) []*actor.PID {
	resp, err := c.engine.Request(c.agentPID, getActive{kind: kind}, c.config.requestTimeout).Result()
	if err != nil {
		return []*actor.PID{nil}
	}
	if res, ok := resp.([]*actor.PID); ok {
		if len(res) == 0 || res == nil {
			return []*actor.PID{nil}
		}
		return res
	}
	return []*actor.PID{nil}
}

// GetActiveByID 通过给定的 ID 返回完整的 PID。
//
//	playerPid := c.GetActiveByID("player/1")
//	// 127.0.0.1:34364/player/1
func (c *Cluster) GetActiveByID(id string) *actor.PID {
	resp, err := c.engine.Request(c.agentPID, getActive{id: id}, c.config.requestTimeout).Result()
	if err != nil {
		return nil
	}
	if res, ok := resp.(*actor.PID); ok {
		return res
	}
	return nil
}

// Member 返回此节点的成员信息。
func (c *Cluster) Member() *Member {
	kinds := make([]string, len(c.kinds))
	for i := 0; i < len(c.kinds); i++ {
		kinds[i] = c.kinds[i].name
	}
	m := &Member{
		ID:     c.config.id,
		Host:   c.engine.Address(),
		Kinds:  kinds,
		Region: c.config.region,
	}
	return m
}

// Engine 返回 actor 引擎。
func (c *Cluster) Engine() *actor.Engine {
	return c.engine
}

// Region 返回集群的区域。
func (c *Cluster) Region() string {
	return c.config.region
}

// ID 返回集群的 ID。
func (c *Cluster) ID() string {
	return c.config.id
}

// Address 返回集群的主机/地址。
func (c *Cluster) Address() string {
	return c.agentPID.Address
}

// PID 返回可达的 actor 进程 ID，即 Agent actor。
func (c *Cluster) PID() *actor.PID {
	return c.agentPID
}

// kindsToString 将 kinds 转换为字符串切片。
func (c *Cluster) kindsToString() []string {
	items := make([]string, len(c.kinds))
	for i := range len(c.kinds) {
		items[i] = c.kinds[i].name
	}
	return items
}

// getRandomListenAddr 返回一个随机的监听地址。
func getRandomListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", rand.Intn(50000)+10000)
}
