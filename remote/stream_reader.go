package remote

import (
	"context"
	"errors"
	"log/slog"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// streamReader 是流读取器，负责接收远程消息。
type streamReader struct {
	DRPCRemoteUnimplementedServer

	remote       *Remote
	deserializer Deserializer
}

// newStreamReader 创建一个新的流读取器。
func newStreamReader(r *Remote) *streamReader {
	return &streamReader{
		remote:       r,
		deserializer: ProtoSerializer{},
	}
}

// Receive 接收并处理远程消息。
func (r *streamReader) Receive(stream DRPCRemote_ReceiveStream) error {
	defer slog.Debug("流读取器已终止")

	for {
		envelope, err := stream.Recv()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			slog.Error("流读取器接收", "err", err)
			return err
		}

		for _, msg := range envelope.Messages {
			tname := envelope.TypeNames[msg.TypeNameIndex]
			payload, err := r.deserializer.Deserialize(msg.Data, tname)

			if err != nil {
				slog.Error("流读取器反序列化", "err", err)
				return err
			}
			target := envelope.Targets[msg.TargetIndex]
			var sender *actor.PID
			if len(envelope.Senders) > 0 {
				sender = envelope.Senders[msg.SenderIndex]
			}
			r.remote.engine.SendLocal(target, payload, sender)
		}
	}

	return nil
}
