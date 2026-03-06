// 这个文件提供访问日志中间件，用来记录管理接口和握手请求的访问情况。

package middleware

import (
	echo "github.com/labstack/echo/v4"
	echoMw "github.com/labstack/echo/v4/middleware"
)

func AccessLog() echo.MiddlewareFunc {
	return echoMw.Logger()
}
