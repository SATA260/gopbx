// 这个文件提供跨域中间件，保证浏览器访问管理接口时具备基本跨域能力。

package middleware

import (
	echo "github.com/labstack/echo/v4"
	echoMw "github.com/labstack/echo/v4/middleware"
)

func CORS() echo.MiddlewareFunc {
	return echoMw.CORS()
}
