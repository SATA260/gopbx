// 这个文件验证真实 WebRTC 建链能力，重点覆盖 /call/webrtc 的真实 SDP answer 和 trickle candidate 处理。

package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gopbx/internal/compat"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func TestWebRTCCallNegotiatesRealAnswerAndAcceptsCandidates(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCallWebRTC, "webrtc-session")

	peer, _, err := newWebRTCPeer(false)
	if err != nil {
		t.Fatalf("create client peer connection: %v", err)
	}
	defer peer.Close()

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
		t.Fatalf("create offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}

	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": offer.SDP,
		},
	}); err != nil {
		t.Fatalf("send webrtc invite: %v", err)
	}

	answer := readEvent(t, conn)
	requireEventName(t, answer, compat.EventAnswer)
	sdp, ok := answer["sdp"].(string)
	if !ok || sdp == "" {
		t.Fatalf("expected non-empty answer sdp, payload=%v", answer)
	}
	if sdp == offer.SDP {
		t.Fatalf("expected real answer instead of offer echo, got identical sdp")
	}
	if !strings.Contains(sdp, "a=fingerprint:") || !strings.Contains(sdp, "a=ice-ufrag:") {
		t.Fatalf("expected real webrtc answer with fingerprint and ice ufrag, got: %s", sdp)
	}

	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}); err != nil {
		t.Fatalf("set remote answer: %v", err)
	}

	var candidateJSON string
	select {
	case candidateJSON = <-candidateCh:
	case <-time.After(5 * time.Second):
		t.Fatal("expected at least one local ICE candidate for trickle signaling")
	}

	if err := conn.WriteJSON(map[string]any{
		"command":    "candidate",
		"candidates": []string{candidateJSON},
	}); err != nil {
		t.Fatalf("send candidate command: %v", err)
	}

	waitPeerConnectionConnected(t, peer)

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)
}

func TestWebRTCAudioProducesASRFinal(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCallWebRTC, "webrtc-audio-session")

	peer, localTrack, err := newWebRTCPeer(true)
	if err != nil {
		t.Fatalf("create audio peer connection: %v", err)
	}
	defer peer.Close()

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
		t.Fatalf("create audio offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}
	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": offer.SDP,
		},
	}); err != nil {
		t.Fatalf("send webrtc invite: %v", err)
	}
	answer := readEvent(t, conn)
	requireEventName(t, answer, compat.EventAnswer)
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
		t.Fatal("expected candidate for audio webrtc peer")
	}
	waitPeerConnectionConnected(t, peer)

	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    111,
			SequenceNumber: 1,
			Timestamp:      1,
			SSRC:           1,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}
	if err := localTrack.WriteRTP(packet); err != nil {
		t.Fatalf("write local RTP packet: %v", err)
	}

	metrics := readEvent(t, conn)
	requireEventName(t, metrics, compat.EventMetrics)
	requireEventField(t, metrics, "key", "ttfb.asr.mock")
	asrFinal := readEvent(t, conn)
	requireEventName(t, asrFinal, compat.EventASRFinal)
	requireEventField(t, asrFinal, "trackId", "webrtc-audio-session")

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)
}

func TestWebRTCTTSProducesOutboundAudio(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCallWebRTC, "webrtc-tts-session")

	peer, _, err := newWebRTCPeer(false)
	if err != nil {
		t.Fatalf("create tts peer connection: %v", err)
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
		t.Fatalf("create tts offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}
	if err := conn.WriteJSON(map[string]any{
		"command": "invite",
		"option": map[string]any{
			"offer": offer.SDP,
			"codec": "pcmu",
		},
	}); err != nil {
		t.Fatalf("send webrtc tts invite: %v", err)
	}
	answer := readEvent(t, conn)
	requireEventName(t, answer, compat.EventAnswer)
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
		t.Fatal("expected candidate for tts webrtc peer")
	}
	waitPeerConnectionConnected(t, peer)

	if err := conn.WriteJSON(map[string]any{
		"command": "tts",
		"text":    "hello from webrtc",
		"option": map[string]any{
			"provider": "mock",
		},
	}); err != nil {
		t.Fatalf("send tts command: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventTrackStart)
	requireEventName(t, readEvent(t, conn), compat.EventMetrics)
	requireEventName(t, readEvent(t, conn), compat.EventMetrics)
	requireEventName(t, readEvent(t, conn), compat.EventTrackEnd)

	var remoteTrack *webrtc.TrackRemote
	select {
	case remoteTrack = <-remoteTrackCh:
	case <-time.After(5 * time.Second):
		t.Fatal("expected outbound webrtc audio track from server tts")
	}
	if _, _, err := remoteTrack.ReadRTP(); err != nil {
		t.Fatalf("read outbound tts RTP packet: %v", err)
	}

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)
}

func newWebRTCPeer(withAudioTrack bool) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticRTP, error) {
	peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, nil, err
	}
	if withAudioTrack {
		track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "pion")
		if err != nil {
			peer.Close()
			return nil, nil, err
		}
		if _, err := peer.AddTrack(track); err != nil {
			peer.Close()
			return nil, nil, err
		}
		return peer, track, nil
	}
	if _, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		peer.Close()
		return nil, nil, err
	}
	return peer, nil, nil
}

func waitPeerConnectionConnected(t *testing.T, peer *webrtc.PeerConnection) {
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
