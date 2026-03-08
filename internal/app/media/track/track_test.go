// 这个文件验证各类媒体音轨实现，确保 trackId、metrics 数据和 WebRTC 协商行为稳定。

package track

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	ttsadapter "gopbx/internal/adapter/outbound/tts"
	"gopbx/pkg/wsproto"

	"github.com/pion/webrtc/v4"
)

func TestWebRTCTrackBuildAnswerAndCandidates(t *testing.T) {
	peer, err := newTestPeerConnection()
	if err != nil {
		t.Fatalf("create client peer connection: %v", err)
	}
	defer peer.Close()
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	if err != nil {
		t.Fatalf("add client transceiver: %v", err)
	}
	if err := transceiver.SetCodecPreferences(testCodecPreferences()); err != nil {
		t.Fatalf("set client codec preferences: %v", err)
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

	track := NewWebRTCTrack("webrtc-1", offer.SDP, "pcmu", nil, nil, nil, nil)
	answer, err := track.BuildAnswer()
	if err != nil {
		t.Fatalf("build answer: %v", err)
	}
	if !strings.Contains(answer, "a=fingerprint:") {
		t.Fatalf("unexpected webrtc answer: %s", answer)
	}
	lower := strings.ToLower(answer)
	if !strings.Contains(lower, "pcmu/8000") || !strings.Contains(lower, "pcma/8000") {
		t.Fatalf("expected answer to contain g711 codecs: %s", answer)
	}
	if strings.Contains(lower, "opus/48000") || strings.Contains(lower, "g722") {
		t.Fatalf("expected answer to exclude unsupported codecs: %s", answer)
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

func TestWebRTCTrackBuildAnswerAllowsRequestedPCMA(t *testing.T) {
	peer, err := newTestPeerConnection()
	if err != nil {
		t.Fatalf("create client peer connection: %v", err)
	}
	defer peer.Close()
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	if err != nil {
		t.Fatalf("add client transceiver: %v", err)
	}
	if err := transceiver.SetCodecPreferences(testCodecPreferences()); err != nil {
		t.Fatalf("set client codec preferences: %v", err)
	}
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}
	track := NewWebRTCTrack("webrtc-2", offer.SDP, "pcma", nil, nil, nil, nil)
	answer, err := track.BuildAnswer()
	if err != nil {
		t.Fatalf("build answer: %v", err)
	}
	lower := strings.ToLower(answer)
	pcmaIndex := strings.Index(lower, "a=rtpmap:8 pcma/8000")
	pcmuIndex := strings.Index(lower, "a=rtpmap:0 pcmu/8000")
	if pcmaIndex == -1 || pcmuIndex == -1 {
		t.Fatalf("expected both pcma and pcmu in answer: %s", answer)
	}
	_ = track.Close()
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

func TestWebRTCTrackPlayTTS(t *testing.T) {
	peer, err := newTestPeerConnection()
	if err != nil {
		t.Fatalf("create client peer: %v", err)
	}
	defer peer.Close()
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	if err != nil {
		t.Fatalf("add client transceiver: %v", err)
	}
	if err := transceiver.SetCodecPreferences(testCodecPreferences()); err != nil {
		t.Fatalf("set client codec preferences: %v", err)
	}
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}
	track := NewWebRTCTrack("webrtc-tts", offer.SDP, "pcmu", nil, nil, nil, nil)
	answer, err := track.BuildAnswer()
	if err != nil {
		t.Fatalf("build answer: %v", err)
	}
	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatalf("set remote answer: %v", err)
	}

	stream := &stubTTSStream{chunks: []ttsadapter.Chunk{{Data: []byte{0x01, 0x02, 0x03}}}}
	audioBytes, chunkCount, err := track.PlayTTS("track-1", &wsproto.SynthesisOption{}, stream)
	if err != nil {
		t.Fatalf("play tts: %v", err)
	}
	if audioBytes == 0 || chunkCount != 1 {
		t.Fatalf("unexpected audio stats: bytes=%d chunks=%d", audioBytes, chunkCount)
	}
}

type stubTTSStream struct {
	chunks []ttsadapter.Chunk
}

func (s *stubTTSStream) Recv() (ttsadapter.Chunk, error) {
	if len(s.chunks) == 0 {
		return ttsadapter.Chunk{}, io.EOF
	}
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]
	return chunk, nil
}

func (s *stubTTSStream) Close() error { return nil }

func newTestPeerConnection() (*webrtc.PeerConnection, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
		PayloadType:        0,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMA, ClockRate: 8000, Channels: 1},
		PayloadType:        8,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	return api.NewPeerConnection(webrtc.Configuration{})
}

func testCodecPreferences() []webrtc.RTPCodecParameters {
	return []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
			PayloadType:        0,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMA, ClockRate: 8000, Channels: 1},
			PayloadType:        8,
		},
	}
}
