// 这个文件集中注册 HTTP 路由，是对外兼容接口的总入口。

package httpinbound

import (
	"gopbx/internal/compat"

	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, h *Handlers) {
	e.GET(compat.RouteCall, h.HandleCallWS)
	e.GET(compat.RouteCallWebRTC, h.HandleWebRTCCallWS)
	e.GET(compat.RouteCallLists, h.HandleListCalls)
	e.POST(compat.RouteCallKill, h.HandleKillCall)
	e.GET(compat.RouteICEServers, h.HandleICEServers)
	e.POST(compat.RouteLLMProxyAny, h.HandleLLMProxy)
}
