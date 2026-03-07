// 这个文件验证一个最小的端到端兼容链路：invite -> answer -> asrFinal -> tts -> hangup。

package e2e_test

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gopbx/internal/bootstrap"
	"gopbx/internal/compat"
	"gopbx/internal/config"

	"github.com/gorilla/websocket"
)

func TestGoZeroVoicechatCompat(t *testing.T) {
	cfg := config.Default()
	cfg.RecorderPath = t.TempDir()
	app := bootstrap.New(cfg)
	server := httptest.NewServer(app.Echo)
	defer server.Close()

	parsed, err := url.Parse("ws" + strings.TrimPrefix(server.URL, "http") + compat.RouteCall)
	if err != nil {
		t.Fatalf("parse ws url: %v", err)
	}
	query := parsed.Query()
	query.Set("id", "e2e-session")
	parsed.RawQuery = query.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": "v=0",
		},
	}); err != nil {
		t.Fatalf("send invite: %v", err)
	}
	requireEvent(t, conn, compat.EventAnswer)

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("send binary audio: %v", err)
	}
	requireEvent(t, conn, compat.EventMetrics)
	requireEvent(t, conn, compat.EventASRFinal)

	if err := conn.WriteJSON(map[string]any{
		"command": "tts",
		"text":    "hello",
		"playId":  "tts-1",
	}); err != nil {
		t.Fatalf("send tts: %v", err)
	}
	requireEvent(t, conn, compat.EventTrackStart)
	requireEvent(t, conn, compat.EventMetrics)
	requireEvent(t, conn, compat.EventMetrics)
	requireEvent(t, conn, compat.EventTrackEnd)

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	requireEvent(t, conn, compat.EventHangup)
}

func requireEvent(t *testing.T, conn *websocket.Conn, want string) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws event: %v", err)
	}
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("decode ws event: %v", err)
	}
	got, _ := event["event"].(string)
	if got != want {
		t.Fatalf("unexpected event, want=%s got=%s payload=%v", want, got, event)
	}
	return event
}
