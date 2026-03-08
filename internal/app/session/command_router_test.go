// 这个文件验证命令路由器的复杂兼容行为，重点覆盖 autoHangup、interrupt 和 no-op 分支。

package session

import (
	"testing"

	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

func TestRoutePlayAutoHangup(t *testing.T) {
	router := NewCommandRouter()
	s := NewSession("s1", TypeWebSocket, nil)
	autoHangup := true
	url := "https://example.com/demo.wav"

	result := router.Route(s, &wsproto.CommandEnvelope{
		Command:    wsproto.CommandPlay,
		URL:        &url,
		AutoHangup: &autoHangup,
	})

	if len(result.Events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(result.Events))
	}
	if result.Events[0].Event != compat.EventTrackStart {
		t.Fatalf("unexpected first event: %s", result.Events[0].Event)
	}
	if result.Events[1].Event != compat.EventMetrics {
		t.Fatalf("unexpected second event: %s", result.Events[1].Event)
	}
	if result.Events[2].Event != compat.EventTrackEnd {
		t.Fatalf("unexpected third event: %s", result.Events[2].Event)
	}
	if result.Events[3].Event != compat.EventHangup {
		t.Fatalf("unexpected fourth event: %s", result.Events[3].Event)
	}
	if result.Close == nil || result.Close.Reason != "autohangup" || result.Close.Initiator != "system" {
		t.Fatalf("unexpected close info: %+v", result.Close)
	}
}

func TestRouteInterruptUsesCurrentTrack(t *testing.T) {
	router := NewCommandRouter()
	s := NewSession("s2", TypeWebSocket, nil)
	playID := "track-1"
	s.StartTrack("tts", &playID)

	result := router.Route(s, &wsproto.CommandEnvelope{Command: wsproto.CommandInterrupt})
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].Event != compat.EventInterruption {
		t.Fatalf("unexpected event: %s", result.Events[0].Event)
	}
	if result.Events[0].TrackID != playID {
		t.Fatalf("unexpected interrupted trackId: %s", result.Events[0].TrackID)
	}
	if s.CurrentTrackID() != "" {
		t.Fatal("expected current track to be cleared after interrupt")
	}
}

func TestRouteNoopCommand(t *testing.T) {
	router := NewCommandRouter()
	s := NewSession("s3", TypeWebSocket, nil)
	result := router.Route(s, &wsproto.CommandEnvelope{Command: wsproto.CommandPause})
	if len(result.Events) != 0 || result.Close != nil {
		t.Fatalf("expected no-op result, got events=%d close=%+v", len(result.Events), result.Close)
	}
}

func TestRouteTTSConsumesProviderStream(t *testing.T) {
	router := NewCommandRouter()
	provider := "aliyun"
	s := NewSession("s4", TypeWebSocket, &wsproto.CallOption{
		TTS: &wsproto.SynthesisOption{Provider: &provider},
	})
	playID := "tts-1"
	result := router.Route(s, &wsproto.CommandEnvelope{Command: wsproto.CommandTTS, Text: "hello", PlayID: &playID})
	if len(result.Events) != 4 {
		t.Fatalf("expected 4 tts events, got %d", len(result.Events))
	}
	if result.Events[1].Key != "ttfb.tts.aliyun" {
		t.Fatalf("unexpected tts metric key: %s", result.Events[1].Key)
	}
	data, ok := result.Events[1].Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected tts metric data: %#v", result.Events[1].Data)
	}
	audioBytes, _ := data["audioBytes"].(int)
	if audioBytes == 0 {
		t.Fatalf("expected synthesized audio bytes in metrics, data=%v", data)
	}
	if result.Events[3].Event != compat.EventTrackEnd {
		t.Fatalf("unexpected final event: %s", result.Events[3].Event)
	}
}
