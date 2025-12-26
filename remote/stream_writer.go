package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcwire"
)

const (
	connIdleTimeout       = time.Minute * 10 // 连接空闲超时时间
	streamWriterBatchSize = 1024             // 流写入器批处理大小
)

// streamWriter 是流写入器，负责向远程发送消息。
type streamWriter struct {
	writeToAddr string
	rawconn     net.Conn
	conn        *drpcconn.Conn
	stream      DRPCRemote_ReceiveStream
	engine      *actor.Engine
	routerPID   *actor.PID
	pid         *actor.PID
	inbox       actor.Inboxer
	serializer  Serializer
	tlsConfig   *tls.Config
	buffSize    int
}

// newStreamWriter 创建一个新的流写入器。
func newStreamWriter(e *actor.Engine, rpid *actor.PID, address string, tlsConfig *tls.Config, buffSize int) actor.Processer {
	return &streamWriter{
		writeToAddr: address,
		engine:      e,
		routerPID:   rpid,
		inbox:       actor.NewInbox(streamWriterBatchSize),
		pid:         actor.NewPID(e.Address(), "stream"+"/"+address),
		serializer:  ProtoSerializer{},
		tlsConfig:   tlsConfig,
		buffSize:    buffSize,
	}
}

// PID 返回流写入器的 PID。
func (s *streamWriter) PID() *actor.PID { return s.pid }

// Send 向流写入器发送消息。
func (s *streamWriter) Send(_ *actor.PID, msg any, sender *actor.PID) {
	s.inbox.Send(actor.Envelope{Msg: msg, Sender: sender})
}

// Invoke 批量处理消息并发送到远程。
func (s *streamWriter) Invoke(msgs []actor.Envelope) {
	var (
		typeLookup   = make(map[string]int32)
		typeNames    = make([]string, 0)
		senderLookup = make(map[uint64]int32)
		senders      = make([]*actor.PID, 0)
		targetLookup = make(map[uint64]int32)
		targets      = make([]*actor.PID, 0)
		messages     = make([]*Message, len(msgs))
	)

	for i := 0; i < len(msgs); i++ {
		var (
			stream   = msgs[i].Msg.(*streamDeliver)
			typeID   int32
			senderID int32
			targetID int32
		)
		typeID, typeNames = lookupTypeName(typeLookup, s.serializer.TypeName(stream.msg), typeNames)
		senderID, senders = lookupPIDs(senderLookup, stream.sender, senders)
		targetID, targets = lookupPIDs(targetLookup, stream.target, targets)

		b, err := s.serializer.Serialize(stream.msg)
		if err != nil {
			slog.Error("序列化", "err", err)
			continue
		}

		messages[i] = &Message{
			Data:          b,
			TypeNameIndex: typeID,
			SenderIndex:   senderID,
			TargetIndex:   targetID,
		}
	}

	env := &Envelope{
		Senders:   senders,
		Targets:   targets,
		TypeNames: typeNames,
		Messages:  messages,
	}

	if err := s.stream.Send(env); err != nil {
		if errors.Is(err, io.EOF) {
			_ = s.conn.Close()
			return
		}
		slog.Error("流写入器发送消息失败",
			"err", err,
		)
	}
	// 刷新连接超时时间。
	err := s.rawconn.SetDeadline(time.Now().Add(connIdleTimeout))
	if err != nil {
		slog.Error("设置上下文超时失败", "err", err)
	}
}

// init 初始化流写入器，建立到远程的连接。
func (s *streamWriter) init() {
	var (
		rawconn    net.Conn
		err        error
		delay      time.Duration = time.Millisecond * 500
		maxRetries               = 3
	)
	for i := 0; i < maxRetries; i++ {
		// 这里我们尝试连接到远程地址。
		switch s.tlsConfig {
		case nil:
			rawconn, err = net.Dial("tcp", s.writeToAddr)
			if err != nil {
				d := time.Duration(delay * time.Duration(i*2))
				slog.Error("net.Dial", "err", err, "remote", s.writeToAddr, "retry", i, "max", maxRetries, "delay", d)
				time.Sleep(d)
				continue
			}
		default:
			slog.Debug("远程使用 TLS 进行写入")
			rawconn, err = tls.Dial("tcp", s.writeToAddr, s.tlsConfig)
			if err != nil {
				d := time.Duration(delay * time.Duration(i*2))
				slog.Error("tls.Dial", "err", err, "remote", s.writeToAddr, "retry", i, "max", maxRetries, "delay", d)
				time.Sleep(d)
				continue
			}
		}
		break
	}
	// 重试 N 次后仍无法连接到远程。因此，关闭流写入器
	// 并通知 RemoteUnreachableEvent。
	if rawconn == nil {
		s.Shutdown()
		return
	}

	s.rawconn = rawconn
	err = rawconn.SetDeadline(time.Now().Add(connIdleTimeout))
	if err != nil {
		slog.Error("设置原始连接超时失败", "err", err)
		return
	}

	conn := drpcconn.NewWithOptions(rawconn, drpcconn.Options{
		Manager: drpcmanager.Options{
			Reader: drpcwire.ReaderOptions{
				MaximumBufferSize: s.buffSize,
			},
		},
	})
	client := NewDRPCRemoteClient(conn)

	stream, err := client.Receive(context.Background())
	if err != nil {
		slog.Error("接收", "err", err, "remote", s.writeToAddr)
		s.Shutdown()
		return
	}

	s.stream = stream
	s.conn = conn

	slog.Debug("已连接",
		"remote", s.writeToAddr,
	)

	go func() {
		<-s.conn.Closed()
		slog.Debug("连接丢失",
			"remote", s.writeToAddr,
		)
		s.Shutdown()
	}()
}

// Shutdown 关闭流写入器。
// TODO: 有没有办法让流路由器监听事件流而不是自己发送事件？
func (s *streamWriter) Shutdown() {
	evt := actor.RemoteUnreachableEvent{ListenAddr: s.writeToAddr}
	s.engine.Send(s.routerPID, evt)
	s.engine.BroadcastEvent(evt)
	if s.stream != nil {
		s.stream.Close()
	}
	s.inbox.Stop()
	s.engine.Registry.Remove(s.PID())
}

// Start 启动流写入器。
func (s *streamWriter) Start() {
	s.inbox.Start(s)
	s.init()
}

// lookupPIDs 查找或添加 PID 到查找表。
func lookupPIDs(m map[uint64]int32, pid *actor.PID, pids []*actor.PID) (int32, []*actor.PID) {
	if pid == nil {
		return 0, pids
	}
	max := int32(len(m))
	key := pid.LookupKey()
	id, ok := m[key]
	if !ok {
		m[key] = max
		id = max
		pids = append(pids, pid)
	}
	return id, pids

}

// lookupTypeName 查找或添加类型名称到查找表。
func lookupTypeName(m map[string]int32, name string, types []string) (int32, []string) {
	max := int32(len(m))
	id, ok := m[name]
	if !ok {
		m[name] = max
		id = max
		types = append(types, name)
	}
	return id, types
}
