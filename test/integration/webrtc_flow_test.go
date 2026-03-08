// 这个文件验证真实 WebRTC 建链能力，重点覆盖 /call/webrtc 的真实 SDP answer 和 trickle candidate 处理。

package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gopbx/internal/compat"

	"github.com/pion/webrtc/v4"
)

func TestWebRTCCallNegotiatesRealAnswerAndAcceptsCandidates(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCallWebRTC, "webrtc-session")

	peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("create client peer connection: %v", err)
	}
	defer peer.Close()
	if _, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		t.Fatalf("add audio transceiver: %v", err)
	}

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
