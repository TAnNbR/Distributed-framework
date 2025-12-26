package cluster

import (
	"log/slog"
	"reflect"
	"strings"

	"github.com/TAnNbR/Distributed-framework/actor"
	"golang.org/x/exp/maps"
)

type (
	// activate 是激活 actor 的请求消息。
	activate struct {
		kind   string
		config ActivationConfig
	}
	// getMembers 是获取成员列表的请求消息。
	getMembers struct{}
	// getKinds 是获取 kind 列表的请求消息。
	getKinds struct{}
	// deactivate 是停用 actor 的请求消息。
	deactivate struct{ pid *actor.PID }
	// getActive 是获取激活的 actor 的请求消息。
	getActive struct {
		id   string
		kind string
	}
)

// Agent 是负责管理集群状态的 actor/接收器。
type Agent struct {
	members    *MemberSet
	cluster    *Cluster
	kinds      map[string]bool
	localKinds map[string]kind
	// 集群范围内可用的所有 actor。
	activated map[string]*actor.PID
}

// NewAgent 创建一个新的 Agent Producer。
func NewAgent(c *Cluster) actor.Producer {
	kinds := make(map[string]bool)
	localKinds := make(map[string]kind)
	for _, kind := range c.kinds {
		kinds[kind.name] = true
		localKinds[kind.name] = kind
	}
	return func() actor.Receiver {
		return &Agent{
			members:    NewMemberSet(),
			cluster:    c,
			kinds:      kinds,
			localKinds: localKinds,
			activated:  make(map[string]*actor.PID),
		}
	}
}

// Receive 处理接收到的消息。
func (a *Agent) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
	case actor.Stopped:
	case *ActorTopology:
		a.handleActorTopology(msg)
	case *Members:
		a.handleMembers(msg.Members)
	case *Activation:
		a.handleActivation(msg)
	case activate:
		pid := a.activate(msg.kind, msg.config)
		c.Respond(pid)
	case deactivate:
		a.bcast(&Deactivation{PID: msg.pid})
	case *Deactivation:
		a.handleDeactivation(msg)
	case *ActivationRequest:
		resp := a.handleActivationRequest(msg)
		c.Respond(resp)
	case getMembers:
		c.Respond(a.members.Slice())
	case getKinds:
		kinds := make([]string, len(a.kinds))
		i := 0
		for kind := range a.kinds {
			kinds[i] = kind
			i++
		}
		c.Respond(kinds)
	case getActive:
		a.handleGetActive(c, msg)
	}
}

// handleGetActive 处理获取激活的 actor 的请求。
func (a *Agent) handleGetActive(c *actor.Context, msg getActive) {
	if len(msg.id) > 0 {
		pid := a.activated[msg.id]
		c.Respond(pid)
	}
	if len(msg.kind) > 0 {
		pids := make([]*actor.PID, 0)
		for id, pid := range a.activated {
			parts := strings.Split(id, "/")
			if len(parts) == 0 {
				break
			}
			kind := parts[0]
			if msg.kind == kind {
				pids = append(pids, pid)
			}
		}
		c.Respond(pids)
	}
}

// handleActorTopology 处理 actor 拓扑消息。
func (a *Agent) handleActorTopology(msg *ActorTopology) {
	for _, actorInfo := range msg.Actors {
		a.addActivated(actorInfo.PID)
	}
}

// handleDeactivation 处理停用消息。
func (a *Agent) handleDeactivation(msg *Deactivation) {
	a.removeActivated(msg.PID)
	a.cluster.engine.Poison(msg.PID)
	a.cluster.engine.BroadcastEvent(DeactivationEvent{PID: msg.PID})
}

// handleActivation 处理激活消息。一个新的 kind 在此集群上被激活。
func (a *Agent) handleActivation(msg *Activation) {
	a.addActivated(msg.PID)
	a.cluster.engine.BroadcastEvent(ActivationEvent{PID: msg.PID})
}

// handleActivationRequest 处理激活请求。
func (a *Agent) handleActivationRequest(msg *ActivationRequest) *ActivationResponse {
	if !a.hasKindLocal(msg.Kind) {
		slog.Error("收到激活请求但 kind 未在本地节点注册", "kind", msg.Kind)
		return &ActivationResponse{Success: false}
	}

	kind := a.localKinds[msg.Kind]
	pid := a.cluster.engine.Spawn(kind.producer, msg.Kind, actor.WithID(msg.ID))
	resp := &ActivationResponse{
		PID:     pid,
		Success: true,
	}
	return resp
}

// activate 激活指定 kind 的 actor。
func (a *Agent) activate(kind string, config ActivationConfig) *actor.PID {
	// 确保 actor 在整个集群中是唯一的。
	id := kind + "/" + config.id // PID 的 id 部分
	if _, ok := a.activated[id]; ok {
		slog.Warn("激活失败", "err", "集群中存在重复的 actor id", "id", id)
		return nil
	}
	members := a.members.FilterByKind(kind)
	if len(members) == 0 {
		slog.Warn("找不到具有该 kind 的成员", "kind", kind)
		return nil
	}
	if config.selectMember == nil {
		config.selectMember = SelectRandomMember
	}
	memberPID := config.selectMember(ActivationDetails{
		Members: members,
		Region:  config.region,
		Kind:    kind,
	})
	if memberPID == nil {
		slog.Warn("激活器未找到可激活的成员")
		return nil
	}
	req := &ActivationRequest{Kind: kind, ID: config.id}
	activatorPID := actor.NewPID(memberPID.Host, "cluster/"+memberPID.ID)

	var activationResp *ActivationResponse
	// 本地激活
	if memberPID.Host == a.cluster.engine.Address() {
		activationResp = a.handleActivationRequest(req)
	} else {
		// 远程激活
		//
		// TODO: 拓扑哈希
		resp, err := a.cluster.engine.Request(activatorPID, req, a.cluster.config.requestTimeout).Result()
		if err != nil {
			slog.Error("激活请求失败", "err", err)
			return nil
		}
		r, ok := resp.(*ActivationResponse)
		if !ok {
			slog.Error("期望 *ActivationResponse", "msg", reflect.TypeOf(resp))
			return nil
		}
		if !r.Success {
			slog.Error("激活不成功", "msg", r)
			return nil
		}
		activationResp = r
	}

	a.bcast(&Activation{
		PID: activationResp.PID,
	})

	return activationResp.PID
}

// handleMembers 处理成员列表消息。
func (a *Agent) handleMembers(members []*Member) {
	joined := NewMemberSet(members...).Except(a.members.Slice())
	left := a.members.Except(members)

	for _, member := range joined {
		a.memberJoin(member)
	}
	for _, member := range left {
		a.memberLeave(member)
	}
}

// memberJoin 处理成员加入。
func (a *Agent) memberJoin(member *Member) {
	a.members.Add(member)

	// 跟踪集群范围内可用的 kind
	for _, kind := range member.Kinds {
		if _, ok := a.kinds[kind]; !ok {
			a.kinds[kind] = true
		}
	}

	actorInfos := make([]*ActorInfo, 0)
	for _, pid := range a.activated {
		actorInfo := &ActorInfo{
			PID: pid,
		}
		actorInfos = append(actorInfos, actorInfo)
	}

	// 向此成员发送我们的 ActorTopology
	if len(actorInfos) > 0 {
		a.cluster.engine.Send(member.PID(), &ActorTopology{Actors: actorInfos})
	}

	// 广播 MemberJoinEvent
	a.cluster.engine.BroadcastEvent(MemberJoinEvent{
		Member: member,
	})

	slog.Debug("[CLUSTER] 成员加入",
		"id", member.ID,
		"host", member.Host,
		"kinds", member.Kinds,
		"region", member.Region,
		"members", len(a.members.members))
}

// memberLeave 处理成员离开。
func (a *Agent) memberLeave(member *Member) {
	a.members.Remove(member)
	a.rebuildKinds()

	// 移除在离开集群的成员上运行的所有 activeKinds。
	for _, pid := range a.activated {
		if pid.Address == member.Host {
			a.removeActivated(pid)
		}
	}

	a.cluster.engine.BroadcastEvent(MemberLeaveEvent{Member: member})

	slog.Debug("[CLUSTER] 成员离开", "id", member.ID, "host", member.Host, "kinds", member.Kinds)
}

// bcast 向所有成员广播消息。
func (a *Agent) bcast(msg any) {
	a.members.ForEach(func(member *Member) bool {
		a.cluster.engine.Send(member.PID(), msg)
		return true
	})
}

// addActivated 添加激活的 actor。
func (a *Agent) addActivated(pid *actor.PID) {
	if _, ok := a.activated[pid.ID]; !ok {
		a.activated[pid.ID] = pid
		slog.Debug("集群上新 actor 可用", "pid", pid)
	}
}

// removeActivated 移除激活的 actor。
func (a *Agent) removeActivated(pid *actor.PID) {
	delete(a.activated, pid.ID)
	slog.Debug("actor 从集群移除", "pid", pid)
}

// hasKindLocal 检查 kind 是否在本地注册。
func (a *Agent) hasKindLocal(name string) bool {
	_, ok := a.localKinds[name]
	return ok
}

// rebuildKinds 重建 kinds 映射。
func (a *Agent) rebuildKinds() {
	maps.Clear(a.kinds)
	a.members.ForEach(func(m *Member) bool {
		for _, kind := range m.Kinds {
			if _, ok := a.kinds[kind]; !ok {
				a.kinds[kind] = true
			}
		}
		return true
	})
}
