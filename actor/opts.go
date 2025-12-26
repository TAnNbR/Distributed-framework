package actor

import (
	"context"
	"time"
)

const (
	defaultInboxSize   = 1024 // 默认收件箱大小
	defaultMaxRestarts = 3    // 默认最大重启次数
)

var defaultRestartDelay = 500 * time.Millisecond // 默认重启延迟

// ReceiveFunc 是消息接收函数的类型。
type ReceiveFunc = func(*Context)

// MiddlewareFunc 是中间件函数的类型。
type MiddlewareFunc = func(ReceiveFunc) ReceiveFunc

// Opts 包含 Actor 的配置选项。
type Opts struct {
	Producer     Producer          // Actor 生产者
	Kind         string            // Actor 类型
	ID           string            // Actor ID
	MaxRestarts  int32             // 最大重启次数
	RestartDelay time.Duration     // 重启延迟
	InboxSize    int               // 收件箱大小
	Middleware   []MiddlewareFunc  // 中间件列表
	Context      context.Context   // Go 上下文
}

// OptFunc 是配置选项函数的类型。
type OptFunc func(*Opts)

// DefaultOpts 根据给定的 Producer 返回默认选项。
func DefaultOpts(p Producer) Opts {
	return Opts{
		Context:      context.Background(),
		Producer:     p,
		MaxRestarts:  defaultMaxRestarts,
		InboxSize:    defaultInboxSize,
		RestartDelay: defaultRestartDelay,
		Middleware:   []MiddlewareFunc{},
	}
}

// WithContext 设置 Actor 的上下文。
func WithContext(ctx context.Context) OptFunc {
	return func(opts *Opts) {
		opts.Context = ctx
	}
}

// WithMiddleware 添加中间件。
func WithMiddleware(mw ...MiddlewareFunc) OptFunc {
	return func(opts *Opts) {
		opts.Middleware = append(opts.Middleware, mw...)
	}
}

// WithRestartDelay 设置重启延迟时间。
func WithRestartDelay(d time.Duration) OptFunc {
	return func(opts *Opts) {
		opts.RestartDelay = d
	}
}

// WithInboxSize 设置收件箱大小。
func WithInboxSize(size int) OptFunc {
	return func(opts *Opts) {
		opts.InboxSize = size
	}
}

// WithMaxRestarts 设置最大重启次数。
func WithMaxRestarts(n int) OptFunc {
	return func(opts *Opts) {
		opts.MaxRestarts = int32(n)
	}
}

// WithID 设置 Actor 的 ID。
func WithID(id string) OptFunc {
	return func(opts *Opts) {
		opts.ID = id
	}
}
