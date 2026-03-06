// 这个文件处理 ICE 查询接口，向客户端返回可用的打洞服务器信息。

package httpinbound

import "github.com/labstack/echo/v4"

func (h *Handlers) HandleICEServers(c echo.Context) error {
	return c.JSON(200, h.Config.ICEServers)
}
