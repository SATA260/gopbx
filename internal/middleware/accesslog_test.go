// 这个文件验证访问日志中间件，确保日志里会带 request_id 和 session_id，便于兼容迁移阶段排查问题。

package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAccessLogIncludesRequestAndSessionID(t *testing.T) {
	e := echo.New()
	buf := new(bytes.Buffer)
	e.Use(RequestID(), AccessLogWithOutput(buf))
	e.GET("/call", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/call?id=session-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	line := buf.String()
	if !strings.Contains(line, "request_id=") {
		t.Fatalf("expected request_id in access log: %s", line)
	}
	if !strings.Contains(line, "session_id=session-1") {
		t.Fatalf("expected session_id in access log: %s", line)
	}
	if !strings.Contains(line, "status=200") {
		t.Fatalf("expected status in access log: %s", line)
	}
}
