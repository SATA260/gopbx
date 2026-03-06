// 这个文件提供请求 ID 中间件，方便日志关联和问题排查。

package middleware

import (
	echo "github.com/labstack/echo/v4"
	echoMw "github.com/labstack/echo/v4/middleware"
)

func RequestID() echo.MiddlewareFunc {
	return echoMw.RequestID()
}
