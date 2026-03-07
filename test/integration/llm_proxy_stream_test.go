// 这个文件验证 LLM 流式代理链路，确保请求、查询参数、鉴权和响应流都能按兼容方式透传。

package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopbx/internal/bootstrap"
	"gopbx/internal/config"
)

func TestLLMProxyStreamsUpstreamResponse(t *testing.T) {
	var seenAuth string
	var seenQuery string
	var seenBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenQuery = r.URL.RawQuery
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		seenBody = string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\ndata: two\n\n")
	}))
	defer upstream.Close()

	cfg := config.Default()
	cfg.LLMProxy.Endpoint = upstream.URL
	cfg.LLMProxy.APIKey = "proxy-key"
	app := bootstrap.New(cfg)
	server := httptest.NewServer(app.Echo)
	defer server.Close()

	resp, err := http.Post(server.URL+"/llm/v1/chat/completions?stream=true&model=gpt", "application/json", strings.NewReader(`{"input":"hi"}`))
	if err != nil {
		t.Fatalf("post llm proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected llm proxy status: %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("unexpected X-Accel-Buffering header: %s", resp.Header.Get("X-Accel-Buffering"))
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read proxied body: %v", err)
	}
	if string(body) != "data: one\n\ndata: two\n\n" {
		t.Fatalf("unexpected proxied body: %s", string(body))
	}
	if seenAuth != "Bearer proxy-key" {
		t.Fatalf("unexpected upstream auth header: %s", seenAuth)
	}
	if seenQuery != "stream=true&model=gpt" && seenQuery != "model=gpt&stream=true" {
		t.Fatalf("unexpected upstream query: %s", seenQuery)
	}
	if seenBody != `{"input":"hi"}` {
		t.Fatalf("unexpected upstream body: %s", seenBody)
	}
}
