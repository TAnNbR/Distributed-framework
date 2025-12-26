package ringbuffer

import (
	"sync"
	"sync/atomic"
)

// buffer 是环形缓冲区的内部结构。
type buffer[T any] struct {
	items           []T
	head, tail, mod int64
}

// RingBuffer 是一个线程安全的环形缓冲区。
type RingBuffer[T any] struct {
	len     int64
	content *buffer[T]
	mu      sync.Mutex
}

// New 创建一个指定大小的环形缓冲区。
func New[T any](size int64) *RingBuffer[T] {
	return &RingBuffer[T]{
		content: &buffer[T]{
			items: make([]T, size),
			head:  0,
			tail:  0,
			mod:   size,
		},
		len: 0,
	}
}

// Push 向缓冲区添加一个元素。如果缓冲区已满，会自动扩容。
func (rb *RingBuffer[T]) Push(item T) {
	rb.mu.Lock()
	rb.content.tail = (rb.content.tail + 1) % rb.content.mod
	if rb.content.tail == rb.content.head {
		size := rb.content.mod * 2
		newBuff := make([]T, size)
		for i := int64(0); i < rb.content.mod; i++ {
			idx := (rb.content.tail + i) % rb.content.mod
			newBuff[i] = rb.content.items[idx]
		}
		content := &buffer[T]{
			items: newBuff,
			head:  0,
			tail:  rb.content.mod,
			mod:   size,
		}
		rb.content = content
	}
	atomic.AddInt64(&rb.len, 1)
	rb.content.items[rb.content.tail] = item
	rb.mu.Unlock()
}

// Len 返回缓冲区中元素的数量。
func (rb *RingBuffer[T]) Len() int64 {
	return atomic.LoadInt64(&rb.len)
}

// Pop 从缓冲区取出一个元素。如果缓冲区为空，返回零值和 false。
func (rb *RingBuffer[T]) Pop() (T, bool) {
	rb.mu.Lock()
	if rb.len == 0 {
		rb.mu.Unlock()
		var t T
		return t, false
	}
	rb.content.head = (rb.content.head + 1) % rb.content.mod
	item := rb.content.items[rb.content.head]
	var t T
	rb.content.items[rb.content.head] = t
	atomic.AddInt64(&rb.len, -1)
	rb.mu.Unlock()
	return item, true
}

// PopN 从缓冲区取出最多 n 个元素。如果缓冲区为空，返回 nil 和 false。
func (rb *RingBuffer[T]) PopN(n int64) ([]T, bool) {
	rb.mu.Lock()
	if rb.len == 0 {
		rb.mu.Unlock()
		return nil, false
	}
	content := rb.content

	if n >= rb.len {
		n = rb.len
	}
	atomic.AddInt64(&rb.len, -n)

	items := make([]T, n)
	for i := int64(0); i < n; i++ {
		pos := (content.head + 1 + i) % content.mod
		items[i] = content.items[pos]
		var t T
		content.items[pos] = t
	}
	content.head = (content.head + n) % content.mod

	rb.mu.Unlock()
	return items, true
}
