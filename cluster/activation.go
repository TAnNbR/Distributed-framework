package cluster

import (
	fmt "fmt"
	"math"
	"math/rand"
)

// ActivationConfig 是激活配置。
type ActivationConfig struct {
	id           string
	region       string
	selectMember SelectMemberFunc
}

// NewActivationConfig 返回一个新的默认配置。
func NewActivationConfig() ActivationConfig {
	return ActivationConfig{
		id:           fmt.Sprintf("%d", rand.Intn(math.MaxInt)),
		region:       "default",
		selectMember: SelectRandomMember,
	}
}

// WithSelectMemberFunc 设置在激活过程中将被调用的函数。
// 它将选择 actor 将被激活/创建的成员。
func (config ActivationConfig) WithSelectMemberFunc(fun SelectMemberFunc) ActivationConfig {
	config.selectMember = fun
	return config
}

// WithID 设置将在集群上激活的 actor 的 ID。
// 默认为随机标识符。
func (config ActivationConfig) WithID(id string) ActivationConfig {
	config.id = id
	return config
}

// WithRegion 设置此 actor 应该被创建的区域。
// 默认为 "default"。
func (config ActivationConfig) WithRegion(region string) ActivationConfig {
	config.region = region
	return config
}

// SelectMemberFunc 将在激活过程中被调用。
// 给定 ActivationDetails，actor 将在返回的成员上创建。
type SelectMemberFunc func(ActivationDetails) *Member

// ActivationDetails 保存有关激活的详细信息。
type ActivationDetails struct {
	// actor 应该被激活的区域
	Region string
	// 按需要激活的 actor 的 kind 预过滤的成员切片
	Members []*Member
	// actor 的 kind
	Kind string
}

// SelectRandomMember 选择集群中的随机成员。
func SelectRandomMember(details ActivationDetails) *Member {
	return details.Members[rand.Intn(len(details.Members))]
}
