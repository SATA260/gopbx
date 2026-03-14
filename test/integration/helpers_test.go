package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type listCallsResponse struct {
	Calls []struct {
		ID string `json:"id"`
	} `json:"calls"`
}

func newIntegrationServer(t *testing.T) (*bootstrap.App, *httptest.Server) {
	t.Helper()
	cfg := config.Default()
	cfg.RecorderPath = t.TempDir()
	return newIntegrationServerWithConfig(t, cfg)
}

func newIntegrationServerWithConfig(t *testing.T, cfg *config.Config) (*bootstrap.App, *httptest.Server) {
	t.Helper()
	app := bootstrap.New(cfg)
	server := httptest.NewServer(app.Echo)
	t.Cleanup(server.Close)
	return app, server
}

func dialCallWS(t *testing.T, serverURL, path, sessionID string) *websocket.Conn {
	t.Helper()
	parsed, err := url.Parse("ws" + strings.TrimPrefix(serverURL, "http") + path)
	if err != nil {
		t.Fatalf("parse websocket url %s: %v", path, err)
	}
	query := parsed.Query()
	query.Set("id", sessionID)
	parsed.RawQuery = query.Encode()
	wsURL := parsed.String()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial websocket %s failed, status=%d err=%v", wsURL, status, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func sendInviteAndReadAnswer(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": "v=0",
		},
	}); err != nil {
		t.Fatalf("send invite: %v", err)
	}
	return readEvent(t, conn)
}

func readEvent(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode websocket event: %v", err)
	}
	return payload
}

func postKill(t *testing.T, serverURL, sessionID string) {
	t.Helper()
	resp, err := http.Post(serverURL+"/call/kill/"+url.PathEscape(sessionID), "application/json", nil)
	if err != nil {
		t.Fatalf("post kill: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected kill status: %d", resp.StatusCode)
	}
}

func listCallIDs(t *testing.T, serverURL string) []string {
	t.Helper()
	resp, err := http.Get(serverURL + compat.RouteCallLists)
	if err != nil {
		t.Fatalf("get call list: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %d", resp.StatusCode)
	}
	var payload listCallsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode call list: %v", err)
	}
	ids := make([]string, 0, len(payload.Calls))
	for _, call := range payload.Calls {
		ids = append(ids, call.ID)
	}
	return ids
}

func eventually(t *testing.T, timeout time.Duration, check func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(message)
}

func expectClose(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set close read deadline: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected websocket connection to close")
	}
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		if !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "websocket: close") {
			t.Fatalf("expected close error, got %v", err)
		}
	}
	_ = conn.SetReadDeadline(time.Time{})
}

func requireEventName(t *testing.T, evt map[string]any, want string) {
	t.Helper()
	got, _ := evt["event"].(string)
	if got != want {
		t.Fatalf("unexpected event, want=%s got=%s payload=%v", want, got, evt)
	}
}

func requireEventField(t *testing.T, evt map[string]any, field, want string) {
	t.Helper()
	got, _ := evt[field].(string)
	if got != want {
		t.Fatalf("unexpected %s, want=%s got=%s payload=%v", field, want, got, evt)
	}
}

func idsString(ids []string) string {
	return fmt.Sprintf("%v", ids)
}

func requireEventNameEventually(t *testing.T, conn *websocket.Conn, want string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		evt := readEvent(t, conn)
		if got, _ := evt["event"].(string); got == want {
			return evt
		}
	}
	t.Fatalf("timed out waiting for event %s", want)
	return nil
}

func newTestVADServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		payload, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		prob := 0.05
		if hasVoiceEnergy(payload) {
			prob = 0.95
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"speechProb": prob})
	}))
	t.Cleanup(server.Close)
	return server
}

func hasVoiceEnergy(payload []byte) bool {
	for i := 0; i+1 < len(payload); i += 2 {
		sample := int16(uint16(payload[i]) | uint16(payload[i+1])<<8)
		if sample > 200 || sample < -200 {
			return true
		}
	}
	return false
}

func makePCMFrame(amplitude int16) []byte {
	frame := make([]byte, 640)
	for i := 0; i+1 < len(frame); i += 2 {
		frame[i] = byte(amplitude)
		frame[i+1] = byte(uint16(amplitude) >> 8)
	}
	return frame
}

func makePCM8kFrame(amplitude int16) []byte {
	frame := make([]byte, 320)
	for i := 0; i+1 < len(frame); i += 2 {
		frame[i] = byte(amplitude)
		frame[i+1] = byte(uint16(amplitude) >> 8)
	}
	return frame
}
