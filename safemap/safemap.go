package safemap

import "sync"

// SafeMap 是一个线程安全的泛型 Map。
type SafeMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// New 创建一个新的 SafeMap。
func New[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		data: make(map[K]V),
	}
}

// Set 设置键值对。
func (s *SafeMap[K, V]) Set(k K, v V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
}

// Get 获取指定键的值。如果键不存在，返回零值和 false。
func (s *SafeMap[K, V]) Get(k K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[k]
	return val, ok
}

// Delete 删除指定的键。
func (s *SafeMap[K, V]) Delete(k K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
}

// Len 返回 Map 中元素的数量。
func (s *SafeMap[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// ForEach 遍历 Map 中的所有键值对。
func (s *SafeMap[K, V]) ForEach(f func(K, V)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.data {
		f(k, v)
	}
}
