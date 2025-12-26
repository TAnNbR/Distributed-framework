package cluster

import "github.com/TAnNbR/Distributed-framework/actor"

// KindConfig 保存已注册 kind 的配置。
type KindConfig struct{}

// NewKindConfig 返回默认的 kind 配置。
func NewKindConfig() KindConfig {
	return KindConfig{}
}

// kind 是一种可以从集群中任何成员激活的 actor 类型。
type kind struct {
	config   KindConfig
	name     string
	producer actor.Producer
}

// newKind 返回一个新的 kind。
func newKind(name string, p actor.Producer, config KindConfig) kind {
	return kind{
		name:     name,
		config:   config,
		producer: p,
	}
}
