// 这个文件处理 ICE 查询接口，向客户端返回兼容既有协议的 ICE 服务器结构。

package httpinbound

import (
	"gopbx/pkg/wsproto"

	"github.com/labstack/echo/v4"
)

func (h *Handlers) HandleICEServers(c echo.Context) error {
	if len(h.Config.ICEServers) > 0 {
		return c.JSON(200, h.Config.ICEServers)
	}
	return c.JSON(200, []wsproto.ICEServer{{
		URLs: []string{"stun:restsend.com:3478"},
	}})
}
