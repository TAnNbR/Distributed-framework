package actor

import (
	"log/slog"
	"time"
)

// 这里定义了各种事件。

// EventLogger 是一个接口，各种事件可以选择实现它。
// 如果实现了，事件流将使用 slog 记录这些事件。
type EventLogger interface {
	Log() (slog.Level, string, []any)
}

// ActorStartedEvent 在每次 Receiver (Actor) 被创建并激活时，
// 通过事件流广播。这意味着，在接收到此事件时，
// Receiver (Actor) 已准备好处理消息。
type ActorStartedEvent struct {
	PID       *PID
	Timestamp time.Time
}

func (e ActorStartedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelDebug, "Actor 已启动", []any{"pid", e.PID}
}

// ActorInitializedEvent 在 actor 接收并处理其 started 事件之前，
// 通过事件流广播。
type ActorInitializedEvent struct {
	PID       *PID
	Timestamp time.Time
}

func (e ActorInitializedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelDebug, "Actor 已初始化", []any{"pid", e.PID}
}

// ActorStoppedEvent 在每次进程终止时，通过事件流广播。
type ActorStoppedEvent struct {
	PID       *PID
	Timestamp time.Time
}

func (e ActorStoppedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelDebug, "Actor 已停止", []any{"pid", e.PID}
}

// ActorRestartedEvent 在 actor 崩溃并被重启时广播。
type ActorRestartedEvent struct {
	PID        *PID
	Timestamp  time.Time
	Stacktrace []byte
	Reason     any
	Restarts   int32
}

func (e ActorRestartedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Actor 崩溃并重启",
		[]any{"pid", e.PID.GetID(), "stack", string(e.Stacktrace),
			"reason", e.Reason, "restarts", e.Restarts}
}

// ActorMaxRestartsExceededEvent 在 actor 崩溃次数过多时创建。
type ActorMaxRestartsExceededEvent struct {
	PID       *PID
	Timestamp time.Time
}

func (e ActorMaxRestartsExceededEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Actor 崩溃次数过多", []any{"pid", e.PID.GetID()}
}

// ActorDuplicateIdEvent 在尝试注册相同名称两次时发布。
type ActorDuplicateIdEvent struct {
	PID *PID
}

func (e ActorDuplicateIdEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Actor 名称已被占用", []any{"pid", e.PID.GetID()}
}

// EngineRemoteMissingEvent 在尝试向远程 actor 发送消息但远程系统不可用时发布。
type EngineRemoteMissingEvent struct {
	Target  *PID
	Sender  *PID
	Message any
}

func (e EngineRemoteMissingEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "引擎没有远程模块", []any{"sender", e.Target.GetID()}
}

// RemoteUnreachableEvent 在尝试向不可达的远程发送消息时发布。
// 该事件将在我们重试拨号 N 次后发布。
type RemoteUnreachableEvent struct {
	// 我们尝试拨号的远程的监听地址。
	ListenAddr string
}

// DeadLetterEvent 在消息无法投递到其接收者时，投递到死信 actor。
type DeadLetterEvent struct {
	Target  *PID
	Message any
	Sender  *PID
}
