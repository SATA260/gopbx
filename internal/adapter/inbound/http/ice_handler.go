// 这个文件处理 ICE 查询接口，向客户端返回兼容既有协议的 ICE 服务器结构。

package httpinbound

import "github.com/labstack/echo/v4"

func (h *Handlers) HandleICEServers(c echo.Context) error {
	if len(h.Config.ICEServers) > 0 {
		return c.JSON(200, h.Config.ICEServers)
	}
	if h.iceProvider != nil {
		servers, err := h.iceProvider.Get(c.Request().Context())
		if err != nil {
			return c.JSON(200, nil)
		}
		if servers == nil {
			return c.JSON(200, nil)
		}
		return c.JSON(200, servers)
	}
	return c.JSON(200, []map[string]any{{
		"urls": []string{"stun:restsend.com:3478"},
	}})
}
