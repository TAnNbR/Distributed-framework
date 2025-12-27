package cluster

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
	"github.com/grandcat/zeroconf"
)

const (
	serviceName        = "_actor.Actors_"
	domain             = "local."
	memberPingInterval = time.Second * 2
)

// MemberAddr 表示集群中可达的节点。
type MemberAddr struct {
	ListenAddr string
	ID         string
}

type (
	// memberLeave 是成员离开的消息。
	memberLeave struct {
		ListenAddr string
	}
	// memberPing 是成员心跳消息。
	memberPing struct{}
)

// SelfManagedConfig 是自管理提供者的配置。
type SelfManagedConfig struct {
	bootstrapMembers []MemberAddr
}

// NewSelfManagedConfig 返回一个新的自管理配置。
func NewSelfManagedConfig() SelfManagedConfig {
	return SelfManagedConfig{
		bootstrapMembers: make([]MemberAddr, 0),
	}
}

// WithBootstrapMember 添加一个引导成员。
func (c SelfManagedConfig) WithBootstrapMember(member MemberAddr) SelfManagedConfig {
	c.bootstrapMembers = append(c.bootstrapMembers, member)
	return c
}

// SelfManaged 是自管理的集群提供者。
type SelfManaged struct {
	config       SelfManagedConfig
	cluster      *Cluster
	members      *MemberSet
	memberPinger actor.SendRepeater
	eventSubPID  *actor.PID

	pid *actor.PID

	membersAlive *MemberSet

	resolver  *zeroconf.Resolver
	announcer *zeroconf.Server

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSelfManagedProvider 创建一个新的自管理提供者。
func NewSelfManagedProvider(config SelfManagedConfig) Producer {
	return func(c *Cluster) actor.Producer {
		return func() actor.Receiver {
			return &SelfManaged{
				config:       config,
				cluster:      c,
				members:      NewMemberSet(),
				membersAlive: NewMemberSet(),
			}
		}
	}
}

// Receive 处理接收到的消息。
func (s *SelfManaged) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
		s.ctx, s.cancel = context.WithCancel(context.Background())
		s.pid = c.PID()

		s.members.Add(s.cluster.Member())
		s.sendMembersToAgent()

		s.memberPinger = c.SendRepeat(c.PID(), memberPing{}, memberPingInterval)
		s.start(c)
	case actor.Stopped:
		s.memberPinger.Stop()
		s.cluster.engine.Unsubscribe(s.eventSubPID)
		s.announcer.Shutdown()
		s.cancel()
	case *Handshake:
		s.addMembers(msg.Member)
		members := s.members.Slice()
		s.cluster.engine.Send(c.Sender(), &Members{
			Members: members,
		})
	case *Members:
		s.addMembers(msg.Members...)
	case memberPing:
		s.handleMemberPing(c)
	case memberLeave:
		member := s.members.GetByHost(msg.ListenAddr)
		s.removeMember(member)
	case *actor.Ping:
	case actor.Initialized:
		_ = msg
	default:
		slog.Warn("收到未处理的消息", "msg", msg, "t", reflect.TypeOf(msg))
	}
}

// handleMemberPing 处理成员心跳。
func (s *SelfManaged) handleMemberPing(c *actor.Context) {
	s.members.ForEach(func(member *Member) bool {
		if member.Host != s.cluster.agentPID.Address {
			ping := &actor.Ping{
				From: c.PID(),
			}
			c.Send(memberToProviderPID(member), ping)
		}
		return true
	})
}

// addMembers 添加成员。
func (s *SelfManaged) addMembers(members ...*Member) {
	for _, member := range members {
		if !s.members.Contains(member) {
			s.members.Add(member)
		}
	}
	s.sendMembersToAgent()
}

// removeMember 移除成员。
func (s *SelfManaged) removeMember(member *Member) {
	if s.members.Contains(member) {
		s.members.Remove(member)
	}
	s.sendMembersToAgent()
}

// sendMembersToAgent 向本地集群代理发送所有当前成员。
func (s *SelfManaged) sendMembersToAgent() {
	members := &Members{
		Members: s.members.Slice(),
	}
	s.cluster.engine.Send(s.cluster.PID(), members)
}

// start 启动自管理提供者。
func (s *SelfManaged) start(c *actor.Context) {
	s.eventSubPID = c.SpawnChildFunc(s.handleEventStream, "event")
	s.cluster.engine.Subscribe(s.eventSubPID)

	// 如果有引导成员，向所有引导成员发送握手。
	for _, member := range s.config.bootstrapMembers {
		memberPID := actor.NewPID(member.ListenAddr, "provider/"+member.ID)
		s.cluster.engine.SendWithSender(memberPID, &Handshake{
			Member: s.cluster.Member(),
		}, c.PID())
	}

	s.initAutoDiscovery()
	s.startAutoDiscovery()
}

// initAutoDiscovery 初始化自动发现。
func (s *SelfManaged) initAutoDiscovery() {
	resolver, err := zeroconf.NewResolver()
	if err != nil {
		log.Fatal(err)
	}
	s.resolver = resolver

	host, portstr, err := net.SplitHostPort(s.cluster.agentPID.Address)
	if err != nil {
		log.Fatal(err)
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.Fatal(err)
	}

	server, err := zeroconf.RegisterProxy(
		s.cluster.ID(),
		serviceName,
		domain,
		port,
		fmt.Sprintf("member_%s", s.cluster.ID()),
		[]string{host},
		[]string{"txtv=0", "lo=1", "la=2"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	s.announcer = server
}

// startAutoDiscovery 启动自动发现。
func (s *SelfManaged) startAutoDiscovery() {
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			if entry.Instance != s.cluster.ID() {
				host := fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
				hs := &Handshake{
					Member: s.cluster.Member(),
				}
				// 为此成员创建可达的 PID。
				memberPID := actor.NewPID(host, "provider/"+entry.Instance)
				self := actor.NewPID(s.cluster.agentPID.Address, "provider/"+s.cluster.ID())
				s.cluster.engine.SendWithSender(memberPID, hs, self)
			}
		}
		slog.Debug("[CLUSTER] 停止发现", "id", s.cluster.ID())
	}(entries)

	err := s.resolver.Browse(s.ctx, serviceName, domain, entries)
	if err != nil {
		slog.Error("[CLUSTER] 发现失败", "err", err)
		panic(err)
	}
}

// handleEventStream 处理事件流。
func (s *SelfManaged) handleEventStream(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.RemoteUnreachableEvent:
		c.Send(s.pid, memberLeave{ListenAddr: msg.ListenAddr})
	}
}
