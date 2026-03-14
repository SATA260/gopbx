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
	requireEventName(t, requireEventNameEventually(t, conn, compat.EventHangup, 2*time.Second), compat.EventHangup)
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

func TestProviderSpecificASRAndTTSMetrics(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCall, "provider-session")

	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": "v=0",
			"asr": map[string]any{
				"provider": "aliyun",
			},
			"tts": map[string]any{
				"provider": "aliyun",
			},
		},
	}); err != nil {
		t.Fatalf("send provider invite: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventAnswer)

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02}); err != nil {
		t.Fatalf("send provider binary audio: %v", err)
	}
	asrMetrics := readEvent(t, conn)
	requireEventName(t, asrMetrics, compat.EventMetrics)
	requireEventField(t, asrMetrics, "key", "ttfb.asr.aliyun")
	asrFinal := readEvent(t, conn)
	requireEventName(t, asrFinal, compat.EventASRFinal)
	requireEventField(t, asrFinal, "text", "aliyun asr final 1")

	if err := conn.WriteJSON(map[string]any{
		"command": "tts",
		"text":    "hello",
		"option": map[string]any{
			"provider": "aliyun",
		},
	}); err != nil {
		t.Fatalf("send provider tts: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventTrackStart)
	ttsMetrics := readEvent(t, conn)
	requireEventName(t, ttsMetrics, compat.EventMetrics)
	requireEventField(t, ttsMetrics, "key", "ttfb.tts.aliyun")
	completedMetrics := readEvent(t, conn)
	requireEventName(t, completedMetrics, compat.EventMetrics)
	requireEventField(t, completedMetrics, "key", "completed.tts.aliyun")
	requireEventName(t, readEvent(t, conn), compat.EventTrackEnd)

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, requireEventNameEventually(t, conn, compat.EventHangup, 2*time.Second), compat.EventHangup)
	expectClose(t, conn)
}

func TestBinaryAudioWithHybridVADProducesSpeechAndEOU(t *testing.T) {
	vadServer := newTestVADServer(t)

	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCall, "vad-session")

	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": "v=0",
			"vad": map[string]any{
				"type":           "hybrid",
				"samplerate":     16000,
				"speechPadding":  20,
				"silencePadding": 60,
				"ratio":          1.5,
				"voiceThreshold": 0.6,
				"endpoint":       vadServer.URL,
			},
		},
	}); err != nil {
		t.Fatalf("send hybrid vad invite: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventAnswer)

	voice := makePCMFrame(1400)
	for i := 0; i < 3; i++ {
		if err := conn.WriteMessage(websocket.BinaryMessage, voice); err != nil {
			t.Fatalf("send voiced frame %d: %v", i, err)
		}
	}

	var sawSpeaking bool
	var sawMetrics bool
	var sawFinal bool
	deadline := time.Now().Add(4 * time.Second)
	for !(sawSpeaking && sawMetrics && sawFinal) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for hybrid vad start events, speaking=%v metrics=%v final=%v", sawSpeaking, sawMetrics, sawFinal)
		}
		event := readEvent(t, conn)
		switch event["event"] {
		case compat.EventSpeaking:
			sawSpeaking = true
		case compat.EventMetrics:
			sawMetrics = true
		case compat.EventASRFinal:
			sawFinal = true
		}
	}

	silence := makePCMFrame(0)
	for i := 0; i < 5; i++ {
		if err := conn.WriteMessage(websocket.BinaryMessage, silence); err != nil {
			t.Fatalf("send silence frame %d: %v", i, err)
		}
	}

	var sawSilence bool
	var sawEOU bool
	deadline = time.Now().Add(4 * time.Second)
	for !(sawSilence && sawEOU) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for hybrid vad end events, silence=%v eou=%v", sawSilence, sawEOU)
		}
		event := readEvent(t, conn)
		switch event["event"] {
		case compat.EventSilence:
			sawSilence = true
		case compat.EventEOU:
			sawEOU = true
		}
	}

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, requireEventNameEventually(t, conn, compat.EventHangup, 2*time.Second), compat.EventHangup)
	expectClose(t, conn)
}
