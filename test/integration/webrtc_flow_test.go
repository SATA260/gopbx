// 这个文件验证真实 WebRTC 建链能力，重点覆盖 /call/webrtc 的真实 SDP answer 和 trickle candidate 处理。

package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	codecutil "gopbx/internal/app/media/codec"
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
	setClientAudioCodecPreferences(t, peer)

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
	if !strings.Contains(strings.ToLower(sdp), "pcmu/8000") || !strings.Contains(strings.ToLower(sdp), "pcma/8000") {
		t.Fatalf("expected answer to advertise g711 codecs, got: %s", sdp)
	}
	if strings.Contains(strings.ToLower(sdp), "opus/48000") || strings.Contains(strings.ToLower(sdp), "g722") {
		t.Fatalf("expected answer to exclude unsupported codecs, got: %s", sdp)
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
	setClientAudioCodecPreferences(t, peer)

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

	encoded := codecutil.PCMUCodec{}.Encode([]byte{0x00, 0x10, 0x00, 0x20})
	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    0,
			SequenceNumber: 1,
			Timestamp:      1,
			SSRC:           1,
		},
		Payload: encoded,
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
	setClientAudioCodecPreferences(t, peer)

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

func TestWebRTCCallAllowsRequestedPCMACodec(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCallWebRTC, "webrtc-pcma-session")

	peer, _, err := newWebRTCPeer(false)
	if err != nil {
		t.Fatalf("create client peer connection: %v", err)
	}
	defer peer.Close()
	setClientAudioCodecPreferences(t, peer)

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
			"codec": "pcma",
		},
	}); err != nil {
		t.Fatalf("send pcma invite: %v", err)
	}
	answer := readEvent(t, conn)
	requireEventName(t, answer, compat.EventAnswer)
	sdp, _ := answer["sdp"].(string)
	lower := strings.ToLower(sdp)
	pcmaIndex := strings.Index(lower, "a=rtpmap:8 pcma/8000")
	pcmuIndex := strings.Index(lower, "a=rtpmap:0 pcmu/8000")
	if pcmaIndex == -1 || pcmuIndex == -1 {
		t.Fatalf("expected both pcma and pcmu in answer, got: %s", sdp)
	}

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)
}

func newWebRTCPeer(withAudioTrack bool) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticRTP, error) {
	peer, err := newPeerConnectionWithG711()
	if err != nil {
		return nil, nil, err
	}
	if withAudioTrack {
		track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1}, "audio", "pion")
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
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	if err != nil {
		peer.Close()
		return nil, nil, err
	}
	if err := transceiver.SetCodecPreferences(clientAudioCodecPreferences()); err != nil {
		peer.Close()
		return nil, nil, err
	}
	return peer, nil, nil
}

func setClientAudioCodecPreferences(t *testing.T, peer *webrtc.PeerConnection) {
	t.Helper()
	for _, transceiver := range peer.GetTransceivers() {
		if transceiver.Kind() != webrtc.RTPCodecTypeAudio {
			continue
		}
		if err := transceiver.SetCodecPreferences(clientAudioCodecPreferences()); err != nil {
			t.Fatalf("set client codec preferences: %v", err)
		}
	}
}

func clientAudioCodecPreferences() []webrtc.RTPCodecParameters {
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

func newPeerConnectionWithG711() (*webrtc.PeerConnection, error) {
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
