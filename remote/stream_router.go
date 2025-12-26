package remote

import (
	"crypto/tls"
	"log/slog"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// streamDeliver 是流传递消息。
type streamDeliver struct {
	sender *actor.PID
	target *actor.PID
	msg    any
}

// streamRouter 是流路由器，负责管理到不同远程地址的流写入器。
type streamRouter struct {
	engine *actor.Engine
	// streams 是远程地址到流写入器 pid 的映射。
	streams   map[string]*actor.PID
	pid       *actor.PID
	tlsConfig *tls.Config
	buffSize  int
}

// newStreamRouter 创建一个新的流路由器。
func newStreamRouter(e *actor.Engine, tlsConfig *tls.Config, buffSize int) actor.Producer {
	return func() actor.Receiver {
		return &streamRouter{
			streams:   make(map[string]*actor.PID),
			engine:    e,
			tlsConfig: tlsConfig,
			buffSize:  buffSize,
		}
	}
}

// Receive 处理接收到的消息。
func (s *streamRouter) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		s.pid = ctx.PID()
	case *streamDeliver:
		s.deliverStream(msg)
	case actor.RemoteUnreachableEvent:
		s.handleTerminateStream(msg)
	}
}

// handleTerminateStream 处理流终止事件。
func (s *streamRouter) handleTerminateStream(msg actor.RemoteUnreachableEvent) {
	streamWriterPID := s.streams[msg.ListenAddr]
	delete(s.streams, msg.ListenAddr)
	slog.Debug("流已终止",
		"remote", msg.ListenAddr,
		"pid", streamWriterPID,
	)
}

// deliverStream 将消息传递到对应的流写入器。
func (s *streamRouter) deliverStream(msg *streamDeliver) {
	var (
		swpid   *actor.PID
		ok      bool
		address = msg.target.Address
	)

	swpid, ok = s.streams[address]
	if !ok {
		swpid = s.engine.SpawnProc(newStreamWriter(s.engine, s.pid, address, s.tlsConfig, s.buffSize))
		s.streams[address] = swpid
	}

	s.engine.Send(swpid, msg)
}
