package actor

import "context"

// InternalError 表示内部错误。
type InternalError struct {
	From string
	Err  error
}

// poisonPill 是毒丸消息，用于停止进程。
type poisonPill struct {
	cancel   context.CancelFunc
	graceful bool // true 表示优雅停止，false 表示立即停止
}

// Initialized 是初始化完成消息。
type Initialized struct{}

// Started 是启动完成消息。
type Started struct{}

// Stopped 是停止完成消息。
type Stopped struct{}
