package actor

// PIDSet 是一个 PID 集合，支持快速查找和删除。
type PIDSet struct {
	pids   []*PID
	lookup map[pidKey]int
}

// pidKey 是 PID 的键类型，用于快速查找。
type pidKey struct {
	address string
	id      string
}

// key 返回 PID 对应的键。
func (p *PIDSet) key(pid *PID) pidKey {
	return pidKey{address: pid.Address, id: pid.ID}
}

// NewPIDSet 创建一个新的 PID 集合。
func NewPIDSet(pids ...*PID) *PIDSet {
	p := &PIDSet{}
	for _, pid := range pids {
		p.Add(pid)
	}
	return p
}

// ensureInit 确保集合已初始化。
func (p *PIDSet) ensureInit() {
	if p.lookup == nil {
		p.lookup = make(map[pidKey]int)
	}
}

// indexOf 返回 PID 在集合中的索引，不存在则返回 -1。
func (p *PIDSet) indexOf(v *PID) int {
	if idx, ok := p.lookup[p.key(v)]; ok {
		return idx
	}

	return -1
}

// Contains 检查集合是否包含给定的 PID。
func (p *PIDSet) Contains(v *PID) bool {
	_, ok := p.lookup[p.key(v)]
	return ok
}

// Add 向集合添加一个 PID。
func (p *PIDSet) Add(v *PID) {
	p.ensureInit()
	if p.Contains(v) {
		return
	}

	p.pids = append(p.pids, v)
	p.lookup[p.key(v)] = len(p.pids) - 1
}

// Remove 从集合中移除 v，如果元素存在则返回 true。
func (p *PIDSet) Remove(v *PID) bool {
	p.ensureInit()
	i := p.indexOf(v)
	if i == -1 {
		return false
	}

	delete(p.lookup, p.key(v))
	if i < len(p.pids)-1 {
		lastPID := p.pids[len(p.pids)-1]

		p.pids[i] = lastPID
		p.lookup[p.key(lastPID)] = i
	}

	p.pids = p.pids[:len(p.pids)-1]

	return true
}

// Len 返回集合中 PID 的数量。
func (p *PIDSet) Len() int {
	return len(p.pids)
}

// Clear 清空集合。
func (p *PIDSet) Clear() {
	p.pids = p.pids[:0]
	p.lookup = make(map[pidKey]int)
}

// Empty 检查集合是否为空。
func (p *PIDSet) Empty() bool {
	return p.Len() == 0
}

// Values 返回集合中所有的 PID。
func (p *PIDSet) Values() []*PID {
	return p.pids
}

// ForEach 遍历集合中的所有 PID。
func (p *PIDSet) ForEach(f func(i int, pid *PID)) {
	for i, pid := range p.pids {
		f(i, pid)
	}
}

// Get 获取指定索引的 PID。
func (p *PIDSet) Get(index int) *PID {
	return p.pids[index]
}

// Clone 克隆集合。
func (p *PIDSet) Clone() *PIDSet {
	return NewPIDSet(p.pids...)
}
