// 这个文件验证 WS 主链路中的媒体兼容壳，包括原始二进制音频触发 ASR 和 play 自动挂断流程。

package integration_test

import (
	"testing"
	"time"

	"gopbx/internal/compat"

	"github.com/gorilla/websocket"
)

func TestBinaryAudioProducesASRFinal(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCall, "audio-session")

	requireEventName(t, sendInviteAndReadAnswer(t, conn), compat.EventAnswer)

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02, 0x03, 0x04}); err != nil {
		t.Fatalf("send binary audio: %v", err)
	}

	metrics := readEvent(t, conn)
	requireEventName(t, metrics, compat.EventMetrics)
	requireEventField(t, metrics, "key", "ttfb.asr.mock")

	asrFinal := readEvent(t, conn)
	requireEventName(t, asrFinal, compat.EventASRFinal)
	requireEventField(t, asrFinal, "trackId", "audio-session")

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)
}

func TestPlayAutoHangupClosesSession(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCall, "play-session")

	requireEventName(t, sendInviteAndReadAnswer(t, conn), compat.EventAnswer)

	autoHangup := true
	if err := conn.WriteJSON(map[string]any{
		"command":    "play",
		"url":        "https://example.com/demo.wav",
		"autoHangup": autoHangup,
	}); err != nil {
		t.Fatalf("send play command: %v", err)
	}

	requireEventName(t, readEvent(t, conn), compat.EventTrackStart)
	metrics := readEvent(t, conn)
	requireEventName(t, metrics, compat.EventMetrics)
	requireEventField(t, metrics, "key", "completed.play.mock")
	requireEventName(t, readEvent(t, conn), compat.EventTrackEnd)
	hangup := readEvent(t, conn)
	requireEventName(t, hangup, compat.EventHangup)
	requireEventField(t, hangup, "reason", "autohangup")
	requireEventField(t, hangup, "initiator", "system")

	eventually(t, time.Second, func() bool {
		return len(listCallIDs(t, server.URL)) == 0
	}, "expected auto-hangup session to disappear from /call/lists")

	expectClose(t, conn)
}
