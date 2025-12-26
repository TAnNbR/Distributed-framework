package remote

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/TAnNbR/Distributed-framework/actor"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/drpc/drpcwire"
)

// Config 保存远程配置。
type Config struct {
	TLSConfig *tls.Config
	BuffSize  int
}

// NewConfig 返回一个新的默认远程配置。
func NewConfig() Config {
	return Config{}
}

// WithTLS 设置远程的 TLS 配置，这将把 Remote 的传输设置为 TLS。
func (c Config) WithTLS(tlsconf *tls.Config) Config {
	c.TLSConfig = tlsconf
	return c
}

// WithBufferSize 设置流读取器的缓冲区大小。
// 如果未提供，默认缓冲区大小由 drpc 包定义为 4MB。
func (c Config) WithBufferSize(size int) Config {
	c.BuffSize = size
	return c
}

// Remote 表示远程通信模块。
type Remote struct {
	addr            string
	engine          *actor.Engine
	config          Config
	streamRouterPID *actor.PID
	stopCh          chan struct{} // Stop 关闭此通道以通知远程停止监听。
	stopWg          *sync.WaitGroup
	state           atomic.Uint32
}

const (
	stateInvalid uint32 = iota
	stateInitialized
	stateRunning
	stateStopped
)

// New 根据给定的 Config 创建一个新的 "Remote" 对象。
func New(addr string, config Config) *Remote {
	r := &Remote{
		addr:   addr,
		config: config,
	}
	r.state.Store(stateInitialized)
	return r
}

// Start 启动远程模块。
func (r *Remote) Start(e *actor.Engine) error {
	if r.state.Load() != stateInitialized {
		return fmt.Errorf("远程模块已启动")
	}
	r.state.Store(stateRunning)
	r.engine = e
	var ln net.Listener
	var err error
	switch r.config.TLSConfig {
	case nil:
		ln, err = net.Listen("tcp", r.addr)
	default:
		slog.Debug("远程使用 TLS 进行监听")
		ln, err = tls.Listen("tcp", r.addr, r.config.TLSConfig)
	}
	if err != nil {
		return fmt.Errorf("远程监听失败: %w", err)
	}
	slog.Debug("正在监听", "addr", r.addr)
	mux := drpcmux.New()
	err = DRPCRegisterRemote(mux, newStreamReader(r))
	if err != nil {
		return fmt.Errorf("注册远程失败: %w", err)
	}
	s := drpcserver.NewWithOptions(mux, drpcserver.Options{
		Manager: drpcmanager.Options{
			Reader: drpcwire.ReaderOptions{
				MaximumBufferSize: r.config.BuffSize,
			},
		},
	})

	r.streamRouterPID = r.engine.Spawn(
		newStreamRouter(r.engine, r.config.TLSConfig, r.config.BuffSize),
		"router", actor.WithInboxSize(1024*1024))
	slog.Debug("服务器已启动", "listenAddr", r.addr)
	r.stopWg = &sync.WaitGroup{}
	r.stopWg.Add(1)
	r.stopCh = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer r.stopWg.Done()
		err := s.Serve(ctx, ln)
		if err != nil {
			slog.Error("drpcserver", "err", err)
		} else {
			slog.Debug("drpcserver 已停止")
		}
	}()
	// 等待 stopCh 被关闭
	go func() {
		<-r.stopCh
		cancel()
	}()
	return nil
}

// Stop 将停止远程监听。
func (r *Remote) Stop() *sync.WaitGroup {
	if r.state.Load() != stateRunning {
		slog.Warn("远程已停止但调用了 stop", "state", r.state.Load())
		return &sync.WaitGroup{} // 返回空的 waitgroup 以便调用者仍然可以等待而不会 panic。
	}
	r.state.Store(stateStopped)
	r.stopCh <- struct{}{}
	return r.stopWg
}

// Send 通过网络将给定的消息发送到具有给定 pid 的进程。
// 可选地，可以给出"发送者 PID"以通知接收进程谁发送了消息。
// 即使远程已停止，发送仍然有效。但是，接收将不起作用。
func (r *Remote) Send(pid *actor.PID, msg any, sender *actor.PID) {
	r.engine.Send(r.streamRouterPID, &streamDeliver{
		target: pid,
		sender: sender,
		msg:    msg,
	})
}

// Address 返回远程的监听地址。
func (r *Remote) Address() string {
	return r.addr
}

func init() {
	RegisterType(&actor.PID{})
}
