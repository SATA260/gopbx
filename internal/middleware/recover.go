// 这个文件提供恢复中间件，避免运行时 panic 直接打断语音网关服务。

package middleware

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func Recover() echo.MiddlewareFunc {
	return echomw.Recover()
}
