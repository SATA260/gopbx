// 这个文件实现 WebSocket 原始音频音轨，用来把二进制帧送入媒体流处理链。

package track

import (
	"gopbx/internal/app/media/stream"
	"gopbx/internal/domain/protocol"
)

type WebSocketTrack struct {
	ID     string
	Stream *stream.Stream
}

func NewWebSocketTrack(id string, mediaStream *stream.Stream) *WebSocketTrack {
	return &WebSocketTrack{ID: id, Stream: mediaStream}
}

func (t *WebSocketTrack) HandleBinary(payload []byte) []protocol.Event {
	if t == nil || t.Stream == nil {
		return nil
	}
	return t.Stream.Push(stream.Packet{TrackID: t.ID, Data: payload})
}
