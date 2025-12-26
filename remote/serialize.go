package remote

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Serializer 是序列化器接口。
type Serializer interface {
	Serialize(msg any) ([]byte, error)
	TypeName(any) string
}

// Deserializer 是反序列化器接口。
type Deserializer interface {
	Deserialize([]byte, string) (any, error)
}

// VTMarshaler 是 vtproto 序列化接口。
type VTMarshaler interface {
	proto.Message
	MarshalVT() ([]byte, error)
}

// VTUnmarshaler 是 vtproto 反序列化接口。
type VTUnmarshaler interface {
	proto.Message
	UnmarshalVT([]byte) error
}

// TODO: 删除这个或说明为什么不删除。
// type DefaultSerializer struct{}

// func (DefaultSerializer) Serialize(msg any) ([]byte, error) {
// 	switch msg.(type) {
// 	case VTMarshaler:
// 		return VTProtoSerializer{}.Serialize(msg)
// 	case proto.Message:
// 		return ProtoSerializer{}.Serialize(msg)
// 	default:
// 		return nil, fmt.Errorf("不支持的消息类型 (%v) 用于序列化", reflect.TypeOf(msg))
// 	}
// }

// func (DefaultSerializer) Deserialize(data []byte, mtype string) (any, error) {
// 	switch msg.(type) {
// 	case VTMarshaler:
// 		return VTProtoSerializer{}.Serialize(msg)
// 	case proto.Message:
// 		return ProtoSerializer{}.Serialize(msg)
// 	default:
// 		return nil, fmt.Errorf("不支持的消息类型 (%v) 用于序列化", reflect.TypeOf(msg))
// 	}
// }

// ProtoSerializer 是 protobuf 序列化器。
type ProtoSerializer struct{}

// Serialize 序列化消息。
func (ProtoSerializer) Serialize(msg any) ([]byte, error) {
	return proto.Marshal(msg.(proto.Message))
}

// Deserialize 反序列化消息。
func (ProtoSerializer) Deserialize(data []byte, tname string) (any, error) {
	pname := protoreflect.FullName(tname)
	n, err := protoregistry.GlobalTypes.FindMessageByName(pname)
	if err != nil {
		return nil, err
	}
	pm := n.New().Interface()
	err = proto.Unmarshal(data, pm)
	return pm, err
}

// TypeName 返回消息的类型名称。
func (ProtoSerializer) TypeName(msg any) string {
	return string(proto.MessageName(msg.(proto.Message)))
}

// VTProtoSerializer 是 vtproto 序列化器。
type VTProtoSerializer struct{}

// TypeName 返回消息的类型名称。
func (VTProtoSerializer) TypeName(msg any) string {
	return string(proto.MessageName(msg.(proto.Message)))
}

// Serialize 序列化消息。
func (VTProtoSerializer) Serialize(msg any) ([]byte, error) {
	return msg.(VTMarshaler).MarshalVT()
}

// Deserialize 反序列化消息。
func (VTProtoSerializer) Deserialize(data []byte, mtype string) (any, error) {
	v, err := registryGetType(mtype)
	if err != nil {
		return nil, err
	}
	err = v.UnmarshalVT(data)
	return v, err
}
