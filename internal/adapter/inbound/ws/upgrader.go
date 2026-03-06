// 这个文件提供 WebSocket 升级器，负责把 HTTP 请求切换成长连接通道。

package wsinbound

import (
	"net/http"

	"github.com/gorilla/websocket"
)

func NewUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
}
