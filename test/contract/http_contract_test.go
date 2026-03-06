// 这个文件是 HTTP 合同测试，校验既有管理接口返回结构是否保持稳定。

package contract_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gopbx/internal/app/session"
	"gopbx/internal/bootstrap"
	"gopbx/internal/compat"
	"gopbx/internal/config"
	"gopbx/pkg/wsproto"
)

func TestListCallsRoute(t *testing.T) {
	app := bootstrap.New(config.Default())
	offer := "v=0"
	caller := "1001"
	callee := "1002"
	s := app.Sessions.Create("session-1", session.TypeWebSocket, &wsproto.CallOption{
		Offer:  &offer,
		Caller: &caller,
		Callee: &callee,
	})
	s.CreatedAt = time.Date(2026, 3, 5, 12, 34, 56, 0, time.UTC)

	req := httptest.NewRequest(http.MethodGet, compat.RouteCallLists, nil)
	rec := httptest.NewRecorder()
	app.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	assertJSONEq(t, mustReadFixture(t, "list_calls.json"), rec.Body.Bytes())
}

func TestKillCallRouteAlwaysReturnsTrue(t *testing.T) {
	app := bootstrap.New(config.Default())
	req := httptest.NewRequest(http.MethodPost, "/call/kill/not-found", nil)
	rec := httptest.NewRecorder()
	app.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "true\n" {
		t.Fatalf("unexpected kill response: %s", rec.Body.String())
	}
}

func TestICEServersPasswordShape(t *testing.T) {
	username := "user-1"
	password := "pass-1"
	cfg := config.Default()
	cfg.ICEServers = []wsproto.ICEServer{{
		URLs:     []string{"turn:turn.example.com:3478"},
		Username: &username,
		Password: &password,
	}}
	app := bootstrap.New(cfg)

	req := httptest.NewRequest(http.MethodGet, compat.RouteICEServers, nil)
	rec := httptest.NewRecorder()
	app.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	assertJSONEq(t, mustReadFixture(t, "iceservers_password.json"), rec.Body.Bytes())
}

func TestICEServersCredentialShape(t *testing.T) {
	username := "user-1"
	credential := "cred-1"
	cfg := config.Default()
	cfg.ICEServers = []wsproto.ICEServer{{
		URLs:       []string{"turn:turn.example.com:3478"},
		Username:   &username,
		Credential: &credential,
	}}
	app := bootstrap.New(cfg)

	req := httptest.NewRequest(http.MethodGet, compat.RouteICEServers, nil)
	rec := httptest.NewRecorder()
	app.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	assertJSONEq(t, mustReadFixture(t, "iceservers_credential.json"), rec.Body.Bytes())
}
