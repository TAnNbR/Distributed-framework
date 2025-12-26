package actor

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// Remoter 是一个接口，抽象了与引擎绑定的远程通信模块。
type Remoter interface {
	Address() string
	Send(*PID, any, *PID)
	Start(*Engine) error
	Stop() *sync.WaitGroup
}

// Producer 类型是一个函数类型，它的作用是生产（返回）一个实现了 Receiver 接口的对象。
// 这使得可以用无状态的方式动态生产 Actor 实例，便于 Actor 的创建和管理。
type Producer func() Receiver

// Receiver 是一个可以接收和处理消息的接口。
type Receiver interface {
	// Receive 方法用于接收和处理消息
	Receive(*Context)
}

// Engine 表示 Actor 引擎，是整个 Actor 系统的核心。
type Engine struct {
	Registry    *Registry
	address     string
	remote      Remoter
	eventStream *PID
}

// EngineConfig 保存引擎的配置信息。
type EngineConfig struct {
	remote Remoter // Remoter 是上面在本文件开头定义的接口类型
}

// NewEngineConfig 返回一个新的默认 EngineConfig。
func NewEngineConfig() EngineConfig {
	return EngineConfig{}
}

// WithRemote 设置远程模块，配置引擎使其能够通过网络发送和接收消息。
func (config EngineConfig) WithRemote(remote Remoter) EngineConfig {
	config.remote = remote
	return config
}

// NewEngine 根据给定的 EngineConfig 返回一个新的 Actor 引擎。
func NewEngine(config EngineConfig) (*Engine, error) {
	e := &Engine{}
	e.Registry = newRegistry(e) // 需要初始化注册表，以便我们可以自定义死信处理
	e.address = LocalLookupAddr
	if config.remote != nil {
		e.remote = config.remote
		e.address = config.remote.Address()
		err := config.remote.Start(e)
		if err != nil {
			return nil, fmt.Errorf("启动远程模块失败: %w", err)
		}
	}
	e.eventStream = e.Spawn(newEventStream(), "eventstream")
	return e, nil
}

// Spawn 创建一个由给定 Producer 生产的进程，可以通过 opts 进行配置。
func (e *Engine) Spawn(p Producer, kind string, opts ...OptFunc) *PID {
	// 首先生成默认的 Option 配置，该配置会基于当前的 Producer（即要生成的 actor 实例/工厂）做初始化
	// 调用 DefaultOpts(p)，将 Producer p 封装进 options 中（options.Producer = p 等），
	// 以便后续 process 创建时能通过 Producer 生成具体的 Receiver（即 actor 实例/消息接受者）
	options := DefaultOpts(p)
	options.Kind = kind
	for _, opt := range opts {
		opt(&options)
	}
	// 检查是否有 ID，没有则生成一个
	if len(options.ID) == 0 {
		id := strconv.Itoa(rand.Intn(math.MaxInt))
		options.ID = id
	}
	proc := newProcess(e, options)
	return e.SpawnProc(proc)
}

// SpawnFunc 将给定的函数作为无状态的接收器/actor 来创建进程。
func (e *Engine) SpawnFunc(f func(*Context), kind string, opts ...OptFunc) *PID {
	return e.Spawn(newFuncReceiver(f), kind, opts...)
}

// SpawnProc 创建给定的 Processer。这个函数在处理自定义创建的 Process 时很有用。
// 可以参考 streamWriter 作为示例。
func (e *Engine) SpawnProc(p Processer) *PID {
	e.Registry.add(p)
	return p.PID()
}

// Address 返回 Actor 引擎的地址。当没有配置远程模块时，
// 使用 "local" 地址，否则使用远程的监听地址。
func (e *Engine) Address() string {
	return e.address
}

// Request 将给定的消息作为"请求"发送给给定的 PID，返回一个将来会解析的响应。
// 调用 Response.Result() 将阻塞直到超时或响应被解析。
func (e *Engine) Request(pid *PID, msg any, timeout time.Duration) *Response {
	resp := NewResponse(e, timeout)
	e.Registry.add(resp)

	e.SendWithSender(pid, msg, resp.PID())

	return resp
}

// SendWithSender 将给定的消息和发送者一起发送给给定的 PID。
// 接收此消息的 Receiver 可以通过调用 Context.Sender() 来获取发送者。
func (e *Engine) SendWithSender(pid *PID, msg any, sender *PID) {
	e.send(pid, msg, sender)
}

// Send 将给定的消息发送给给定的 PID。如果由于给定的进程未注册而无法投递消息，
// 消息将被发送到死信进程。
func (e *Engine) Send(pid *PID, msg any) {
	e.send(pid, msg, nil)
}

// BroadcastEvent 将给定的消息广播到事件流，通知所有订阅的 actor。
func (e *Engine) BroadcastEvent(msg any) {
	if e.eventStream != nil {
		e.send(e.eventStream, msg, nil)
	}
}

func (e *Engine) send(pid *PID, msg any, sender *PID) {
	// TODO: 我们可能需要在这里记录日志。还没决定什么是合理的。
	// 发送到死信还是作为事件？死信可能更合理，因为目标不可达。
	if pid == nil {
		return
	}
	if e.isLocalMessage(pid) {
		e.SendLocal(pid, msg, sender)
		return
	}
	if e.remote == nil {
		e.BroadcastEvent(EngineRemoteMissingEvent{Target: pid, Sender: sender, Message: msg})
		return
	}
	e.remote.Send(pid, msg, sender)
}

// SendRepeater 是一个结构体，用于向给定的 PID 发送重复消息。
// 如果你需要让一个 actor 定期唤醒，可以使用 SendRepeater。
// 它通过 SendRepeat 方法启动，通过其 Stop() 方法停止。
type SendRepeater struct {
	engine   *Engine
	self     *PID
	target   *PID
	msg      any
	interval time.Duration
	cancelch chan struct{}
}

func (sr SendRepeater) start() {
	ticker := time.NewTicker(sr.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				sr.engine.SendWithSender(sr.target, sr.msg, sr.self)
			case <-sr.cancelch:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop 停止重复发送消息。
func (sr SendRepeater) Stop() {
	close(sr.cancelch)
}

// SendRepeat 将给定的消息以给定的间隔发送给给定的 PID。
// 返回一个 SendRepeater 结构体，可以通过调用 Stop() 来停止重复发送。
func (e *Engine) SendRepeat(pid *PID, msg any, interval time.Duration) SendRepeater {
	clonedPID := *pid.CloneVT()
	sr := SendRepeater{
		engine:   e,
		self:     nil,
		target:   &clonedPID,
		interval: interval,
		msg:      msg,
		cancelch: make(chan struct{}, 1),
	}
	sr.start()
	return sr
}

// Stop 向与给定 PID 关联的进程发送非优雅的 poisonPill 消息。
// 进程将立即关闭。返回一个 context，可用于阻塞/等待直到进程停止。
func (e *Engine) Stop(pid *PID) context.Context {
	return e.sendPoisonPill(context.Background(), false, pid)
}

// Poison 向与给定 PID 关联的进程发送优雅的 poisonPill 消息。
// 进程将在处理完收件箱中的所有消息后优雅关闭。
// 返回一个 context，可用于阻塞/等待直到进程停止。
func (e *Engine) Poison(pid *PID) context.Context {
	return e.sendPoisonPill(context.Background(), true, pid)
}

// PoisonCtx 行为与 Poison 完全相同，唯一的区别是它接受一个 context 作为第一个参数。
// 该 context 可用于自定义超时和手动取消。
func (e *Engine) PoisonCtx(ctx context.Context, pid *PID) context.Context {
	return e.sendPoisonPill(ctx, true, pid)
}

func (e *Engine) sendPoisonPill(ctx context.Context, graceful bool, pid *PID) context.Context {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	pill := poisonPill{
		cancel:   cancel,
		graceful: graceful,
	}
	// 死信 - 如果我们没有找到进程，我们将广播一个 DeadletterEvent
	if e.Registry.get(pid) == nil {
		e.BroadcastEvent(DeadLetterEvent{
			Target:  pid,
			Message: pill,
			Sender:  nil,
		})
		cancel()
		return ctx
	}
	e.SendLocal(pid, pill, nil)
	return ctx
}

// SendLocal 将给定的消息发送给给定的 PID。如果在注册表中找不到接收者，
// 消息将被发送到死信进程。如果没有注册死信进程，函数将 panic。
func (e *Engine) SendLocal(pid *PID, msg any, sender *PID) {
	proc := e.Registry.get(pid)
	if proc == nil {
		// 广播死信消息
		e.BroadcastEvent(DeadLetterEvent{
			Target:  pid,
			Message: msg,
			Sender:  sender,
		})
		return
	}
	proc.Send(pid, msg, sender)
}

// Subscribe 将给定的 PID 订阅到事件流。
func (e *Engine) Subscribe(pid *PID) {
	e.Send(e.eventStream, eventSub{pid: pid})
}

// Unsubscribe 将给定的 PID 从事件流取消订阅。
func (e *Engine) Unsubscribe(pid *PID) {
	e.Send(e.eventStream, eventUnsub{pid: pid})
}

func (e *Engine) isLocalMessage(pid *PID) bool {
	if pid == nil {
		return false
	}
	return e.address == pid.Address
}

type funcReceiver struct {
	f func(*Context)
}

func newFuncReceiver(f func(*Context)) Producer {
	return func() Receiver {
		return &funcReceiver{
			f: f,
		}
	}
}

// Receive 实现了 Receiver 接口的方法。当 Engine 调用该 actor 的 Receive 方法时，
// 实际上会执行外部传递进来的函数 f，将当前的 Context 作为参数传递进去，
// 实现了无状态函数式 actor 的处理机制。
func (r *funcReceiver) Receive(c *Context) {
	// 调用函数型 actor 的消息处理逻辑
	r.f(c)
}
