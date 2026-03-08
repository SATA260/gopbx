//go:build livealiyun

// 这个文件用于真实联调阿里云 ASR 的端到端链路，验证网关入口、音频链和事件回推能串起来。

package e2e_test

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	ttsadapter "gopbx/internal/adapter/outbound/tts"
	"gopbx/internal/bootstrap"
	"gopbx/internal/compat"
	"gopbx/internal/config"
	"gopbx/pkg/wsproto"

	"github.com/gorilla/websocket"
)

func TestLiveAliyunASREndToEnd(t *testing.T) {
	token := os.Getenv("ALIYUN_TOKEN")
	appKey := os.Getenv("ALIYUN_APPKEY")
	if token == "" || appKey == "" {
		t.Fatal("ALIYUN_TOKEN or ALIYUN_APPKEY is empty")
	}

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
	query.Set("id", "aliyun-live-e2e")
	parsed.RawQuery = query.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"asr": map[string]any{
				"provider":   "aliyun",
				"appId":      appKey,
				"samplerate": 16000,
				"extra": map[string]any{
					"token": token,
				},
			},
		},
	}); err != nil {
		t.Fatalf("send live invite: %v", err)
	}
	requireEvent(t, conn, compat.EventAnswer)

	pcmCodec := "pcm"
	sampleRate := int32(16000)
	ttsStream, err := ttsadapter.AliyunProvider{}.StartSynthesis("你好，今天天气怎么样。", &wsproto.SynthesisOption{
		AppID:      &appKey,
		Codec:      &pcmCodec,
		Samplerate: &sampleRate,
		Extra: map[string]string{
			"token": token,
		},
	})
	if err != nil {
		t.Fatalf("start live aliyun tts source: %v", err)
	}
	defer ttsStream.Close()

	chunkCount := 0
	for {
		chunk, recvErr := ttsStream.Recv()
		if recvErr != nil {
			break
		}
		if len(chunk.Data) == 0 {
			continue
		}
		chunkCount++
		if err := conn.WriteMessage(websocket.BinaryMessage, chunk.Data); err != nil {
			t.Fatalf("send live pcm chunk: %v", err)
		}
		time.Sleep(15 * time.Millisecond)
	}
	if chunkCount == 0 {
		t.Fatal("expected synthesized pcm chunks for live e2e asr test")
	}
	// 实时识别的最终结果通常会在语音结束并观察到一小段静音后才稳定产生。
	// 这里补一段 16k PCM 静音帧，既模拟真实通话结束时的静音尾巴，也给当前处理链一个继续 drain 回调结果的机会。
	silence := make([]byte, 640)
	for i := 0; i < 25; i++ {
		if err := conn.WriteMessage(websocket.BinaryMessage, silence); err != nil {
			t.Fatalf("send silence tail: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	var metrics map[string]any
	var final map[string]any
	timeout := time.After(12 * time.Second)
	for metrics == nil || final == nil {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for live asr events, metrics=%v final=%v", metrics, final)
		default:
		}
		event := requireEventWithTimeout(t, conn, "", 8*time.Second)
		name, _ := event["event"].(string)
		switch name {
		case compat.EventMetrics:
			if key, _ := event["key"].(string); key == "ttfb.asr.aliyun" && metrics == nil {
				metrics = event
			}
		case compat.EventASRFinal:
			if final == nil {
				final = event
			}
		}
	}

	t.Logf("live e2e metrics: %v", metrics)
	t.Logf("live e2e final: %v", final)
	if key, _ := metrics["key"].(string); key != "ttfb.asr.aliyun" {
		t.Fatalf("unexpected live metrics key: %v", metrics)
	}
	if text, _ := final["text"].(string); strings.TrimSpace(text) == "" {
		t.Fatalf("expected non-empty live asr final text: %v", final)
	}

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	requireEvent(t, conn, compat.EventHangup)
}

func requireEventWithTimeout(t *testing.T, conn *websocket.Conn, want string, timeout time.Duration) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
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
	if want != "" && got != want {
		t.Fatalf("unexpected event, want=%s got=%s payload=%v", want, got, event)
	}
	return event
}
