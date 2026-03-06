// 这个文件负责路由装配，把兼容接口统一注册到 Echo 实例上。

package bootstrap

import (
	httpinbound "gopbx/internal/adapter/inbound/http"

	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, handlers *httpinbound.Handlers) {
	httpinbound.RegisterRoutes(e, handlers)
}
