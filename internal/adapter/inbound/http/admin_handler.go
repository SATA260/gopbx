// 这个文件处理管理接口，负责查询活跃通话和执行强制挂断。

package httpinbound

import "github.com/labstack/echo/v4"

func (h *Handlers) HandleListCalls(c echo.Context) error {
	return c.JSON(200, map[string]any{"calls": h.Sessions.List()})
}

func (h *Handlers) HandleKillCall(c echo.Context) error {
	id := c.Param("id")
	return c.JSON(200, h.Sessions.Kill(id))
}
