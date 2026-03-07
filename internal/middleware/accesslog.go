// 这个文件提供访问日志中间件，用来记录管理接口、握手请求以及与会话相关的请求标识。

package middleware

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	echo "github.com/labstack/echo/v4"
)

func AccessLog() echo.MiddlewareFunc {
	return AccessLogWithOutput(os.Stdout)
}

// AccessLogWithOutput 额外暴露输出目标，方便测试验证日志里是否带上 request_id 和 session_id。
func AccessLogWithOutput(output io.Writer) echo.MiddlewareFunc {
	if output == nil {
		output = io.Discard
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)
			requestID := c.Response().Header().Get(echo.HeaderXRequestID)
			sessionID := sessionIDFromContext(c)
			line := fmt.Sprintf("method=%s path=%s status=%d latency_ms=%d request_id=%s session_id=%s\n",
				c.Request().Method,
				c.Path(),
				c.Response().Status,
				latency.Milliseconds(),
				requestID,
				sessionID,
			)
			_, _ = io.WriteString(output, line)
			return err
		}
	}
}

func sessionIDFromContext(c echo.Context) string {
	if id := strings.TrimSpace(c.QueryParam("id")); id != "" {
		return id
	}
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		return id
	}
	return ""
}
