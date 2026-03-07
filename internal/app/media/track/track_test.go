// 这个文件验证各类媒体音轨兼容壳，确保 trackId、metrics 数据和 WebRTC 协商占位输出稳定。

package track

import (
	"strings"
	"testing"
)

func TestWebRTCTrackBuildAnswerAndCandidates(t *testing.T) {
	track := NewWebRTCTrack("webrtc-1", "", "g722")
	answer := track.BuildAnswer()
	if !strings.Contains(answer, "G722/16000") {
		t.Fatalf("unexpected webrtc answer: %s", answer)
	}
	track.AddCandidates([]string{"candidate-1", "candidate-2"})
	if len(track.Candidates()) != 2 {
		t.Fatalf("unexpected candidate count: %v", track.Candidates())
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
