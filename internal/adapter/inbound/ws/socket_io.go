// 这个文件封装 WebSocket 读写辅助能力，统一发送事件、二进制和错误消息。

package wsinbound

import (
	"github.com/gorilla/websocket"

	"gopbx/pkg/wsproto"
)

func WriteEvent(conn *websocket.Conn, evt wsproto.EventEnvelope) error {
	data, err := MarshalEvent(evt)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func WriteBinary(conn *websocket.Conn, payload []byte) error {
	return conn.WriteMessage(websocket.BinaryMessage, payload)
}

func WriteError(conn *websocket.Conn, trackID, sender, message string) error {
	return WriteEvent(conn, wsproto.NewErrorEvent(trackID, sender, message))
}
