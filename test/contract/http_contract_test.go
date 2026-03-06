// 这个文件是 HTTP 合同测试，校验管理接口路由的兼容性是否保持稳定。

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gopbx/internal/bootstrap"
	"gopbx/internal/compat"
	"gopbx/internal/config"
)

func TestListCallsRoute(t *testing.T) {
	app := bootstrap.New(config.Default())
	req := httptest.NewRequest(http.MethodGet, compat.RouteCallLists, nil)
	rec := httptest.NewRecorder()

	app.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
