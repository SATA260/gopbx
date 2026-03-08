//go:build livealiyun

// 这个文件用于真实联调阿里云 TTS 的端到端链路，验证网关能够通过 WebRTC 出站音轨把真实合成音频发给客户端。

package e2e_test

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gopbx/internal/bootstrap"
	"gopbx/internal/compat"
	"gopbx/internal/config"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

func TestLiveAliyunTTSEndToEnd(t *testing.T) {
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

	peer, err := newLiveTTSPeerConnection()
	if err != nil {
		t.Fatalf("create live tts peer connection: %v", err)
	}
	defer peer.Close()

	remoteTrackCh := make(chan *webrtc.TrackRemote, 1)
	peer.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		remoteTrackCh <- remote
	})

	candidateCh := make(chan string, 8)
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
		t.Fatalf("create live tts offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}

	parsed, err := url.Parse("ws" + strings.TrimPrefix(server.URL, "http") + compat.RouteCallWebRTC)
	if err != nil {
		t.Fatalf("parse ws url: %v", err)
	}
	query := parsed.Query()
	query.Set("id", "aliyun-live-tts")
	parsed.RawQuery = query.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": offer.SDP,
			"codec": "pcmu",
		},
	}); err != nil {
		t.Fatalf("send live tts invite: %v", err)
	}
	answer := requireEventWithTimeout(t, conn, compat.EventAnswer, 8*time.Second)
	sdp, _ := answer["sdp"].(string)
	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}); err != nil {
		t.Fatalf("set remote answer: %v", err)
	}
	select {
	case candidateJSON := <-candidateCh:
		if err := conn.WriteJSON(map[string]any{"command": "candidate", "candidates": []string{candidateJSON}}); err != nil {
			t.Fatalf("send candidate command: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("expected candidate for live tts peer")
	}
	waitPeerConnectionConnectedE2E(t, peer)

	if err := conn.WriteJSON(map[string]any{
		"command": "tts",
		"text":    "你好，今天天气怎么样。",
		"option": map[string]any{
			"provider":   "aliyun",
			"appId":      appKey,
			"samplerate": 16000,
			"extra": map[string]any{
				"token": token,
			},
		},
	}); err != nil {
		t.Fatalf("send live tts command: %v", err)
	}
	requireEventWithTimeout(t, conn, compat.EventTrackStart, 8*time.Second)
	metrics1 := requireEventWithTimeout(t, conn, compat.EventMetrics, 8*time.Second)
	metrics2 := requireEventWithTimeout(t, conn, compat.EventMetrics, 8*time.Second)
	requireEventWithTimeout(t, conn, compat.EventTrackEnd, 8*time.Second)

	if key, _ := metrics1["key"].(string); key != "ttfb.tts.aliyun" {
		t.Fatalf("unexpected ttfb metric: %v", metrics1)
	}
	if key, _ := metrics2["key"].(string); key != "completed.tts.aliyun" {
		t.Fatalf("unexpected completed metric: %v", metrics2)
	}

	var remoteTrack *webrtc.TrackRemote
	select {
	case remoteTrack = <-remoteTrackCh:
	case <-time.After(8 * time.Second):
		t.Fatal("expected live outbound webrtc track from tts")
	}

	rtpPackets := 0
	bytesTotal := 0
	deadline := time.Now().Add(8 * time.Second)
	for rtpPackets < 3 && time.Now().Before(deadline) {
		if err := remoteTrack.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set remote track deadline: %v", err)
		}
		packet, _, err := remoteTrack.ReadRTP()
		if err != nil {
			continue
		}
		rtpPackets++
		bytesTotal += len(packet.Payload)
	}
	t.Logf("live tts outbound packets=%d bytes=%d", rtpPackets, bytesTotal)
	if rtpPackets == 0 || bytesTotal == 0 {
		t.Fatalf("expected live tts outbound audio packets, packets=%d bytes=%d", rtpPackets, bytesTotal)
	}

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	requireEventWithTimeout(t, conn, compat.EventHangup, 8*time.Second)
}

func newLiveTTSPeerConnection() (*webrtc.PeerConnection, error) {
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
	peer, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	if err != nil {
		peer.Close()
		return nil, err
	}
	if err := transceiver.SetCodecPreferences([]webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
			PayloadType:        0,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMA, ClockRate: 8000, Channels: 1},
			PayloadType:        8,
		},
	}); err != nil {
		peer.Close()
		return nil, err
	}
	return peer, nil
}

func waitPeerConnectionConnectedE2E(t *testing.T, peer *webrtc.PeerConnection) {
	t.Helper()
	stateCh := make(chan webrtc.PeerConnectionState, 8)
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		stateCh <- state
	})
	if peer.ConnectionState() == webrtc.PeerConnectionStateConnected {
		return
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for {
		select {
		case state := <-stateCh:
			if state == webrtc.PeerConnectionStateConnected {
				return
			}
			if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
				t.Fatalf("unexpected peer connection state: %s", state)
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for peer connection to connect, last=%s", peer.ConnectionState())
		}
	}
}
