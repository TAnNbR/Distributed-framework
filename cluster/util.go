package cluster

import (
	"github.com/TAnNbR/Distributed-framework/actor"
)

// memberToProviderPID 从成员创建一个新的 PID，
// 将指向集群的 provider actor。
func memberToProviderPID(m *Member) *actor.PID {
	return actor.NewPID(m.Host, "provider/"+m.ID)
}

// NewCID 返回一个新的集群 ID。
func NewCID(pid *actor.PID, kind, id, region string) *CID {
	return &CID{
		PID:    pid,
		Kind:   kind,
		ID:     id,
		Region: region,
	}
}

// Equals 返回给定的 CID 是否与调用者相等。
func (cid *CID) Equals(other *CID) bool {
	return cid.ID == other.ID && cid.Kind == other.Kind
}

// PID 返回节点代理可达的集群 PID。
func (m *Member) PID() *actor.PID {
	return actor.NewPID(m.Host, "cluster/"+m.ID)
}

// Equals 判断两个成员是否相等。
func (m *Member) Equals(other *Member) bool {
	return m.Host == other.Host && m.ID == other.ID
}

// HasKind 返回成员是否注册了给定的 kind。
// TODO: 可能需要重新定位此函数。
func (m *Member) HasKind(kind string) bool {
	for _, k := range m.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}
