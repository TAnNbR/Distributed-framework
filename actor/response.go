package actor

import (
	"context"
	"math"
	"math/rand"
	"strconv"
	"time"
)

// Response 表示请求-响应模式中的响应对象。
type Response struct {
	engine  *Engine
	pid     *PID
	result  chan any
	timeout time.Duration
}

// NewResponse 创建一个新的 Response 对象。
func NewResponse(e *Engine, timeout time.Duration) *Response {
	return &Response{
		engine:  e,
		result:  make(chan any, 1),
		timeout: timeout,
		pid:     NewPID(e.address, "response"+pidSeparator+strconv.Itoa(rand.Intn(math.MaxInt32))),
	}
}

// Result 等待并返回响应结果。如果超时，返回错误。
func (r *Response) Result() (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer func() {
		cancel()
		r.engine.Registry.Remove(r.pid)
	}()

	select {
	case resp := <-r.result:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Send 实现 Processer 接口，用于接收响应消息。
func (r *Response) Send(_ *PID, msg any, _ *PID) {
	r.result <- msg
}

// PID 返回 Response 的 PID。
func (r *Response) PID() *PID { return r.pid }

// Shutdown 实现 Processer 接口（空实现）。
func (r *Response) Shutdown() {}

// Start 实现 Processer 接口（空实现）。
func (r *Response) Start() {}

// Invoke 实现 Processer 接口（空实现）。
func (r *Response) Invoke([]Envelope) {}
