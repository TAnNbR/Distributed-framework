package actor

import (
	"context"
	"log/slog"
)

// eventSub 是用于订阅事件流的消息。
type eventSub struct {
	pid *PID
}

// eventUnsub 是用于取消订阅事件流的消息。
type eventUnsub struct {
	pid *PID
}

// eventStream 是事件流 actor。
type eventStream struct {
	subs map[*PID]bool // 订阅者 PID 列表
}

// newEventStream 创建一个新的事件流 Producer。
func newEventStream() Producer {

	return func() Receiver {
		// 在Go中，Producer的类型定义为"func() Receiver"，也就是说该函数返回值的类型必须实现Receiver接口。
		// 这里返回的是"&eventStream{...}"，eventStream类型在本文件中实现了Receive(*Context)方法，因此它实现了Receiver接口。
		// 所以可以直接返回&eventStream{}，其类型会自动被视为Receiver接口类型的值（因为Go是基于方法集的接口实现机制）。
		// eventStream 通过实现 Receive(*Context) 方法作为消息处理函数，实现了 Receiver 接口。
		// 在 Receive 中，根据收到的消息类型，维护订阅者列表（eventSub、eventUnsub），
		// 并将通用事件转发给所有订阅者。例如，订阅消息会把 PID 添加到 subs 字典；
		// 退订消息会从 subs 字典移除 PID。收到其他事件时，会遍历 subs，将消息转发给所有订阅者。
		// 这里是 Producer 的返回值，它的返回类型应该实现 Receiver 接口（即有 Receive(*Context) 方法）。
		// 这里返回的 &eventStream{} 实现了 Receive(*Context) 方法（见下文），
		// Producer 本身并不接受 *Context 参数，只有 Receiver 的 Receive 方法才接受 *Context。
		// 因此这里的写法是正确的，不需要在这里传入 *Context。
		return &eventStream{
			subs: make(map[*PID]bool),
		}
	}
}

// Receive 用于事件流。所有系统级事件都发送到这里。
// 一些事件会被特殊处理，如 eventSub、eventUnsub（用于订阅事件），
// DeadletterSub、DeadletterUnSub 用于订阅 DeadLetterEvent。
// Receive 会在 eventStream 这个 actor 被发送消息时被调用。
// 即，每当有消息（如订阅、取消订阅、或事件消息）投递到 eventStream 这个 actor，由引擎调度器调用其 Receive 方法进行处理。
// 通常在引擎初始化时会自动创建 eventStream actor，
// 其他 actor 通过 Publish/Broadcast 或发送 eventSub、eventUnsub 来与之交互。
// 主要发生在：
//  1. 有 Actor 通过 Subscribe/Unsubscribe 主动订阅或退订系统事件。
//  2. 系统通过 BroadcastEvent 或其他方式投递事件到 eventStream，
//     eventStream 再将消息转发（或广播）给所有订阅方。
//
// 如 Engine.Subscribe(pid)、Engine.Unsubscribe(pid)、Engine.BroadcastEvent(event) 等场景。
func (e *eventStream) Receive(c *Context) {
	switch msg := c.Message().(type) {
	case eventSub:
		e.subs[msg.pid] = true
	case eventUnsub:
		delete(e.subs, msg.pid)
	default:
		// 检查是否应该记录事件，如果是，使用相关的级别、消息和属性记录
		logMsg, ok := c.Message().(EventLogger)
		if ok {
			level, msg, attr := logMsg.Log()
			slog.Log(context.Background(), level, msg, attr...)
		}
		for sub := range e.subs {
			c.Forward(sub)
		}
	}
}
