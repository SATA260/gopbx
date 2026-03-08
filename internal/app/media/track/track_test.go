// 这个文件验证各类媒体音轨实现，确保 trackId、metrics 数据和 WebRTC 协商行为稳定。

package track

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func TestWebRTCTrackBuildAnswerAndCandidates(t *testing.T) {
	peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("create client peer connection: %v", err)
	}
	defer peer.Close()
	if _, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		t.Fatalf("add client transceiver: %v", err)
	}
	candidateCh := make(chan string, 4)
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		data, err := json.Marshal(candidate.ToJSON())
		if err == nil {
			candidateCh <- string(data)
		}
	})
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}

	track := NewWebRTCTrack("webrtc-1", offer.SDP, "g722", nil, nil, nil, nil)
	answer, err := track.BuildAnswer()
	if err != nil {
		t.Fatalf("build answer: %v", err)
	}
	if !strings.Contains(answer, "a=fingerprint:") {
		t.Fatalf("unexpected webrtc answer: %s", answer)
	}
	select {
	case candidate := <-candidateCh:
		if err := track.AddCandidates([]string{candidate}); err != nil {
			t.Fatalf("add candidate: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("expected client candidate for webrtc track")
	}
	if len(track.Candidates()) != 1 {
		t.Fatalf("unexpected candidate count: %v", track.Candidates())
	}
	if err := track.Close(); err != nil {
		t.Fatalf("close webrtc track: %v", err)
	}
}

func TestTTSTrackTrackIDAndMetricsData(t *testing.T) {
	speaker := "alice"
	playID := "tts-1"
	track := NewTTSTrack("fallback", "hello", &speaker, &playID)
	if track.TrackID() != playID {
		t.Fatalf("unexpected tts trackId: %s", track.TrackID())
	}
	metrics := track.MetricsData(true, false)
	if metrics["speaker"] != speaker || metrics["playId"] != playID {
		t.Fatalf("unexpected tts metrics: %v", metrics)
	}
}

func TestFileTrackTrackIDAndMetricsData(t *testing.T) {
	playID := "play-1"
	track := NewFileTrack("fallback", "https://example.com/demo.wav", &playID)
	if track.TrackID() != playID {
		t.Fatalf("unexpected file trackId: %s", track.TrackID())
	}
	metrics := track.MetricsData()
	if metrics["url"] != "https://example.com/demo.wav" {
		t.Fatalf("unexpected file metrics: %v", metrics)
	}
}
