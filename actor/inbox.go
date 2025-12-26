package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/TAnNbR/Distributed-framework/ringbuffer"
)

const (
	defaultThroughput = 300       // 默认吞吐量
	messageBatchSize  = 1024 * 4  // 消息批处理大小
)

const (
	stopped int32 = iota  // 已停止
	starting              // 启动中
	idle                  // 空闲
	running               // 运行中
)

// Scheduler 是调度器接口。
type Scheduler interface {
	Schedule(fn func())
	Throughput() int
}

// goscheduler 是基于 goroutine 的调度器。
type goscheduler int

// Schedule 调度一个函数执行。
func (goscheduler) Schedule(fn func()) {
	go fn()
}

// Throughput 返回调度器的吞吐量。
func (sched goscheduler) Throughput() int {
	return int(sched)
}

// NewScheduler 创建一个新的调度器。
func NewScheduler(throughput int) Scheduler {
	return goscheduler(throughput)
}

// Inboxer 是收件箱接口。
type Inboxer interface {
	Send(Envelope)
	Start(Processer)
	Stop() error
}

// Inbox 是消息收件箱，使用环形缓冲区存储消息。
type Inbox struct {
	rb         *ringbuffer.RingBuffer[Envelope]
	proc       Processer
	scheduler  Scheduler
	procStatus int32
}

// NewInbox 创建一个新的收件箱。
func NewInbox(size int) *Inbox {
	return &Inbox{
		rb:         ringbuffer.New[Envelope](int64(size)),
		scheduler:  NewScheduler(defaultThroughput),
		procStatus: stopped,
	}
}

// Send 向收件箱发送消息。
func (in *Inbox) Send(msg Envelope) {
	in.rb.Push(msg)
	in.schedule()
}

// schedule 调度消息处理。
func (in *Inbox) schedule() {
	if atomic.CompareAndSwapInt32(&in.procStatus, idle, running) {
		in.scheduler.Schedule(in.process)
	}
}

// process 处理消息。
func (in *Inbox) process() {
	in.run()
	if atomic.CompareAndSwapInt32(&in.procStatus, running, idle) && in.rb.Len() > 0 {
		// 消息可能在最后一次 pop 和转换到 idle 状态之间被添加到环形缓冲区。
		// 如果是这种情况，我们应该再次调度。
		in.schedule()
	}
}

// run 运行消息处理循环。
func (in *Inbox) run() {
	i, t := 0, in.scheduler.Throughput()
	for atomic.LoadInt32(&in.procStatus) != stopped {
		if i > t {
			i = 0
			runtime.Gosched()
		}
		i++

		if msgs, ok := in.rb.PopN(messageBatchSize); ok && len(msgs) > 0 {
			in.proc.Invoke(msgs)
		} else {
			return
		}
	}
}

// Start 启动收件箱。
func (in *Inbox) Start(proc Processer) {
	// 转换到 "starting" 然后 "idle" 以确保 in.proc 上没有竞态条件
	if atomic.CompareAndSwapInt32(&in.procStatus, stopped, starting) {
		in.proc = proc
		atomic.SwapInt32(&in.procStatus, idle)
		in.schedule()
	}
}

// Stop 停止收件箱。
func (in *Inbox) Stop() error {
	atomic.StoreInt32(&in.procStatus, stopped)
	return nil
}
