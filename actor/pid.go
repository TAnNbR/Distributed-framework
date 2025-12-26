package actor

import (
	"github.com/zeebo/xxh3"
)

// pidSeparator 是 PID 中地址和 ID 之间的分隔符。
const pidSeparator = "/"

// NewPID 根据给定的地址和 ID 返回一个新的进程 ID。
func NewPID(address, id string) *PID {
	p := &PID{
		Address: address,
		ID:      id,
	}
	return p
}

// String 返回 PID 的字符串表示形式。
func (pid *PID) String() string {
	return pid.Address + pidSeparator + pid.ID
}

// Equals 判断两个 PID 是否相等。
func (pid *PID) Equals(other *PID) bool {
	return pid.Address == other.Address && pid.ID == other.ID
}

// Child 返回当前 PID 的子 PID。
func (pid *PID) Child(id string) *PID {
	childID := pid.ID + pidSeparator + id
	return NewPID(pid.Address, childID)
}

// LookupKey 返回用于查找的哈希键。
func (pid *PID) LookupKey() uint64 {
	key := []byte(pid.Address)
	key = append(key, pid.ID...)
	return xxh3.Hash(key)
}
