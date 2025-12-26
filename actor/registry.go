package actor

import (
	"sync"
)

// LocalLookupAddr 是本地查找地址。
const LocalLookupAddr = "local"

// Registry 是 Actor 注册表，用于管理所有已注册的进程。
type Registry struct {
	mu     sync.RWMutex
	lookup map[string]Processer
	engine *Engine
}

// newRegistry 创建一个新的注册表。
func newRegistry(e *Engine) *Registry {
	return &Registry{
		lookup: make(map[string]Processer, 1024),
		engine: e,
	}
}

// GetPID 返回与给定 kind 和 id 关联的进程 ID。
// 如果未找到进程，返回 nil。
func (r *Registry) GetPID(kind, id string) *PID {
	proc := r.getByID(kind + pidSeparator + id)
	if proc != nil {
		return proc.PID()
	}
	return nil
}

// Remove 从注册表中移除给定的 PID。
func (r *Registry) Remove(pid *PID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.lookup, pid.ID)
}

// get 返回给定 PID 对应的 processer（如果存在）。
// 如果不存在，返回 nil，调用者必须检查这一点，
// 并将消息转发到死信 processer。
func (r *Registry) get(pid *PID) Processer {
	if pid == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if proc, ok := r.lookup[pid.ID]; ok {
		return proc
	}
	return nil // 未找到 processer
}

// getByID 根据 ID 获取 processer。
func (r *Registry) getByID(id string) Processer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lookup[id]
}

// add 向注册表添加一个进程并启动它。
func (r *Registry) add(proc Processer) {
	r.mu.Lock()
	id := proc.PID().ID
	if _, ok := r.lookup[id]; ok {
		r.mu.Unlock()
		r.engine.BroadcastEvent(ActorDuplicateIdEvent{PID: proc.PID()})
		return
	}
	r.lookup[id] = proc
	r.mu.Unlock()
	proc.Start()
}
