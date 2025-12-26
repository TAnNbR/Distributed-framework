package actor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/DataDog/gostackparse"
)

// Envelope 是消息信封，包含消息内容和发送者信息。
type Envelope struct {
	Msg    any
	Sender *PID
}

// Processer 是一个接口，抽象了进程的行为方式。
type Processer interface {
	Start()
	PID() *PID
	Send(*PID, any, *PID)
	Invoke([]Envelope)
	Shutdown()
}

// process 表示一个 Actor 进程。
type process struct {
	Opts

	inbox    Inboxer
	context  *Context
	pid      *PID
	restarts int32
	mbuffer  []Envelope
}

// newProcess 创建一个新的进程。
func newProcess(e *Engine, opts Opts) *process {
	pid := NewPID(e.address, opts.Kind+pidSeparator+opts.ID)
	ctx := newContext(opts.Context, e, pid)
	p := &process{
		pid:     pid,
		inbox:   NewInbox(opts.InboxSize),
		Opts:    opts,
		context: ctx,
		mbuffer: nil,
	}
	return p
}

// applyMiddleware 应用中间件到接收函数。
func applyMiddleware(rcv ReceiveFunc, middleware ...MiddlewareFunc) ReceiveFunc {
	for i := len(middleware) - 1; i >= 0; i-- {
		rcv = middleware[i](rcv)
	}
	return rcv
}

// Invoke 处理消息列表。
func (p *process) Invoke(msgs []Envelope) {
	var (
		// 需要处理的消息数量
		nmsg = len(msgs)
		// 已处理的消息数量
		nproc = 0
		// 修复: 我们可以在这里使用 nproc，但由于某种原因，将 nproc++ 放在
		// 函数底部会冻结某些测试。因此，我创建了一个新的计数器用于记录。
		processed = 0
	)
	defer func() {
		// 如果我们恢复了，我们将缓冲所有无法处理的消息，
		// 以便在下次重启时重试。
		if v := recover(); v != nil {
			p.context.message = Stopped{}
			p.context.receiver.Receive(p.context)

			p.mbuffer = make([]Envelope, nmsg-nproc)
			for i := 0; i < nmsg-nproc; i++ {
				p.mbuffer[i] = msgs[i+nproc]
			}
			p.tryRestart(v)
		}
	}()

	for i := 0; i < len(msgs); i++ {
		nproc++
		msg := msgs[i]
		if pill, ok := msg.Msg.(poisonPill); ok {
			// 如果需要优雅停止，我们处理收件箱中的所有消息，
			// 否则我们忽略并清理。
			if pill.graceful {
				msgsToProcess := msgs[processed:]
				for _, m := range msgsToProcess {
					p.invokeMsg(m)
				}
			}
			p.cleanup(pill.cancel)
			return
		}
		p.invokeMsg(msg)
		processed++
	}
}

// invokeMsg 处理单条消息。
func (p *process) invokeMsg(msg Envelope) {
	// 在这里过滤 poisonPill 消息。它们是 actor 引擎私有的。
	if _, ok := msg.Msg.(poisonPill); ok {
		return
	}
	p.context.message = msg.Msg
	p.context.sender = msg.Sender
	recv := p.context.receiver
	if len(p.Opts.Middleware) > 0 {
		applyMiddleware(recv.Receive, p.Opts.Middleware...)(p.context)
	} else {
		recv.Receive(p.context)
	}
}

// Start 启动进程。
func (p *process) Start() {
	recv := p.Producer()
	p.context.receiver = recv
	defer func() {
		if v := recover(); v != nil {
			p.context.message = Stopped{}
			p.context.receiver.Receive(p.context)
			p.tryRestart(v)
		}
	}()
	p.context.message = Initialized{}
	applyMiddleware(recv.Receive, p.Opts.Middleware...)(p.context)
	p.context.engine.BroadcastEvent(ActorInitializedEvent{PID: p.pid, Timestamp: time.Now()})

	p.context.message = Started{}
	applyMiddleware(recv.Receive, p.Opts.Middleware...)(p.context)
	p.context.engine.BroadcastEvent(ActorStartedEvent{PID: p.pid, Timestamp: time.Now()})
	// 如果缓冲区中有消息，调用它们。
	if len(p.mbuffer) > 0 {
		p.Invoke(p.mbuffer)
		p.mbuffer = nil
	}

	p.inbox.Start(p)
}

// tryRestart 尝试重启进程。
func (p *process) tryRestart(v any) {
	// InternalError 不考虑最大重启次数。
	// 目前，InternalError 在我们拨号远程节点时触发。通过这样做，
	// 我们可以持续拨号直到它恢复上线。
	// 注意：不确定这是否是最佳选择。如果该节点永远不再上线怎么办？
	if msg, ok := v.(*InternalError); ok {
		slog.Error(msg.From, "err", msg.Err)
		time.Sleep(p.Opts.RestartDelay)
		p.Start()
		return
	}
	stackTrace := cleanTrace(debug.Stack())
	// 如果达到最大重启次数，我们关闭收件箱并清理一切。
	if p.restarts == p.MaxRestarts {
		p.context.engine.BroadcastEvent(ActorMaxRestartsExceededEvent{
			PID:       p.pid,
			Timestamp: time.Now(),
		})
		p.cleanup(nil)
		return
	}

	p.restarts++
	// 在重启延迟后重启进程
	p.context.engine.BroadcastEvent(ActorRestartedEvent{
		PID:        p.pid,
		Timestamp:  time.Now(),
		Stacktrace: stackTrace,
		Reason:     v,
		Restarts:   p.restarts,
	})
	time.Sleep(p.Opts.RestartDelay)
	p.Start()
}

// cleanup 清理进程资源。
func (p *process) cleanup(cancel context.CancelFunc) {
	defer cancel()

	if p.context.parentCtx != nil {
		p.context.parentCtx.children.Delete(p.pid.ID)
	}

	if p.context.children.Len() > 0 {
		children := p.context.Children()
		for _, pid := range children {
			<-p.context.engine.Poison(pid).Done()
		}
	}

	p.inbox.Stop()
	p.context.engine.Registry.Remove(p.pid)
	p.context.message = Stopped{}
	applyMiddleware(p.context.receiver.Receive, p.Opts.Middleware...)(p.context)

	p.context.engine.BroadcastEvent(ActorStoppedEvent{PID: p.pid, Timestamp: time.Now()})
}

// PID 返回进程的 PID。
func (p *process) PID() *PID { return p.pid }

// Send 向进程发送消息。
func (p *process) Send(_ *PID, msg any, sender *PID) {
	p.inbox.Send(Envelope{Msg: msg, Sender: sender})
}

// Shutdown 关闭进程。
func (p *process) Shutdown() {
	p.cleanup(nil)
}

// cleanTrace 清理堆栈跟踪信息。
func cleanTrace(stack []byte) []byte {
	goros, err := gostackparse.Parse(bytes.NewReader(stack))
	if err != nil {
		slog.Error("解析堆栈跟踪失败", "err", err)
		return stack
	}
	if len(goros) != 1 {
		slog.Error("预期只有一个 goroutine", "goroutines", len(goros))
		return stack
	}
	// 跳过前几帧
	goros[0].Stack = goros[0].Stack[4:]
	buf := bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(buf, "goroutine %d [%s]\n", goros[0].ID, goros[0].State)
	for _, frame := range goros[0].Stack {
		_, _ = fmt.Fprintf(buf, "%s\n", frame.Func)
		_, _ = fmt.Fprint(buf, "\t", frame.File, ":", frame.Line, "\n")
	}
	return buf.Bytes()
}
