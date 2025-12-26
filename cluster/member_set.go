package cluster

// MemberSet 是成员集合，支持快速查找和操作。
type MemberSet struct {
	members map[string]*Member
}

// NewMemberSet 创建一个新的成员集合。
func NewMemberSet(members ...*Member) *MemberSet {
	m := make(map[string]*Member)
	for _, member := range members {
		m[member.ID] = member
	}
	return &MemberSet{
		members: m,
	}
}

// Len 返回集合中成员的数量。
func (s *MemberSet) Len() int {
	return len(s.members)
}

// GetByHost 根据主机地址获取成员。
func (s *MemberSet) GetByHost(host string) *Member {
	var theMember *Member
	for _, member := range s.members {
		if member.Host == host {
			theMember = member
		}
	}
	return theMember
}

// Add 向集合添加成员。
func (s *MemberSet) Add(member *Member) {
	s.members[member.ID] = member
}

// Contains 检查集合是否包含指定成员。
func (s *MemberSet) Contains(member *Member) bool {
	_, ok := s.members[member.ID]
	return ok
}

// Remove 从集合中移除成员。
func (s *MemberSet) Remove(member *Member) {
	delete(s.members, member.ID)
}

// RemoveByHost 根据主机地址移除成员。
func (s *MemberSet) RemoveByHost(host string) {
	member := s.GetByHost(host)
	if member != nil {
		s.Remove(member)
	}
}

// Slice 返回成员的切片。
func (s *MemberSet) Slice() []*Member {
	members := make([]*Member, len(s.members))
	i := 0
	for _, member := range s.members {
		members[i] = member
		i++
	}
	return members
}

// ForEach 遍历集合中的每个成员。
func (s *MemberSet) ForEach(fun func(m *Member) bool) {
	for _, s := range s.members {
		if !fun(s) {
			break
		}
	}
}

// Except 返回在当前集合中但不在给定切片中的成员。
func (s *MemberSet) Except(members []*Member) []*Member {
	var (
		except = []*Member{}
		m      = make(map[string]*Member)
	)
	for _, member := range members {
		m[member.ID] = member
	}
	for _, member := range s.members {
		if _, ok := m[member.ID]; !ok {
			except = append(except, member)
		}
	}
	return except
}

// FilterByKind 返回具有指定 kind 的成员。
func (s *MemberSet) FilterByKind(kind string) []*Member {
	members := []*Member{}
	for _, member := range s.members {
		if member.HasKind(kind) {
			members = append(members, member)
		}
	}
	return members
}
