package cluster

import "github.com/TAnNbR/Distributed-framework/actor"

// MemberJoinEvent 在每次有新成员加入集群时触发。
type MemberJoinEvent struct {
	Member *Member
}

// MemberLeaveEvent 在每次有成员离开集群时触发。
type MemberLeaveEvent struct {
	Member *Member
}

// ActivationEvent 在每次有新 actor 在集群某处被激活时触发。
type ActivationEvent struct {
	PID *actor.PID
}

// DeactivationEvent 在每次有 actor 在集群某处被停用时触发。
type DeactivationEvent struct {
	PID *actor.PID
}
