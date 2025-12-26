package remote

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// registry 是类型注册表。
var registry = map[string]VTUnmarshaler{}

// RegisterType 注册一个类型到注册表。
func RegisterType(v VTUnmarshaler) {
	tname := string(proto.MessageName(v))
	registry[tname] = v
}

// registryGetType 从注册表获取类型。
func registryGetType(t string) (VTUnmarshaler, error) {
	if m, ok := registry[t]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("给定类型 (%s) 未注册。你是否忘记使用 remote.RegisterType(&instance{}) 注册你的类型？", t)
}
