// 这个文件定义处理器返回结果，支持同时产出协议事件和下游继续消费的媒体包。

package processor

import (
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
)

type Result struct {
	Packets []mediaentity.Packet
	Events  []protocol.Event
}

func passthrough(packet mediaentity.Packet) Result {
	return Result{Packets: []mediaentity.Packet{packet}}
}
