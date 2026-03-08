// 这个文件验证 Go SDK 的主要调用能力，确保外部程序可以通过 SDK 调用当前网关的 HTTP 和 WS 接口。

package gosdk_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"gopbx/internal/bootstrap"
	"gopbx/internal/compat"
	"gopbx/internal/config"
	"gopbx/pkg/wsproto"
	gosdk "gopbx/sdk/go"

	"github.com/pion/webrtc/v4"
)

func TestClientCallSessionLifecycle(t *testing.T) {
	client, cleanup := newSDKClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.DialCall(ctx, gosdk.DialOptions{SessionID: "sdk-session"})
	if err != nil {
		t.Fatalf("dial call session: %v", err)
	}
	defer session.Close()

	offer := "v=0"
	if err := session.Invite(&gosdk.CallOption{Offer: &offer}); err != nil {
		t.Fatalf("send invite: %v", err)
	}
	answer, err := session.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("read answer: %v", err)
	}
	if answer.Event != compat.EventAnswer {
		t.Fatalf("unexpected first event: %+v", answer)
	}

	if err := session.SendAudio([]byte{0x01, 0x02, 0x03, 0x04}); err != nil {
		t.Fatalf("send audio: %v", err)
	}
	metrics, err := session.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if metrics.Event != compat.EventMetrics {
		t.Fatalf("unexpected metrics event: %+v", metrics)
	}
	final, err := session.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("read final event: %v", err)
	}
	if final.Event != compat.EventASRFinal {
		t.Fatalf("unexpected asr final event: %+v", final)
	}

	if err := session.Hangup("done", "client"); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	hangup, err := session.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("read hangup: %v", err)
	}
	if hangup.Event != compat.EventHangup {
		t.Fatalf("unexpected hangup event: %+v", hangup)
	}
}

func TestClientAdminAPIs(t *testing.T) {
	client, cleanup := newSDKClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.DialCall(ctx, gosdk.DialOptions{SessionID: "sdk-admin"})
	if err != nil {
		t.Fatalf("dial admin call session: %v", err)
	}
	defer session.Close()

	offer := "v=0"
	if err := session.Invite(&gosdk.CallOption{Offer: &offer}); err != nil {
		t.Fatalf("send invite: %v", err)
	}
	if _, err := session.ReadEvent(ctx); err != nil {
		t.Fatalf("read answer: %v", err)
	}

	lists, err := client.ListCalls(ctx)
	if err != nil {
		t.Fatalf("list calls: %v", err)
	}
	if len(lists.Calls) != 1 || lists.Calls[0].ID != "sdk-admin" {
		t.Fatalf("unexpected list result: %+v", lists)
	}

	ok, err := client.KillCall(ctx, "sdk-admin")
	if err != nil {
		t.Fatalf("kill call: %v", err)
	}
	if !ok {
		t.Fatal("expected kill result to be true")
	}
}

func TestClientICEServers(t *testing.T) {
	username := "user-1"
	credential := "cred-1"
	cfg := config.Default()
	cfg.RecorderPath = t.TempDir()
	cfg.ICEServers = []wsproto.ICEServer{{
		URLs:       []string{"turn:turn.example.com:3478"},
		Username:   &username,
		Credential: &credential,
	}}
	app := bootstrap.New(cfg)
	server := httptest.NewServer(app.Echo)
	defer server.Close()

	client := gosdk.NewClient(gosdk.ClientOptions{HTTPBaseURL: server.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	servers, err := client.GetICEServers(ctx)
	if err != nil {
		t.Fatalf("get ice servers: %v", err)
	}
	if len(servers) != 1 || servers[0].Credential == nil || *servers[0].Credential != credential {
		t.Fatalf("unexpected ice servers: %+v", servers)
	}
}

func TestClientDialWebRTC(t *testing.T) {
	client, cleanup := newSDKClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.DialWebRTC(ctx, gosdk.DialOptions{SessionID: "sdk-webrtc"})
	if err != nil {
		t.Fatalf("dial webrtc session: %v", err)
	}
	defer session.Close()
	peer, offer := newSDKWebRTCOffer(t)
	defer peer.Close()
	codec := "pcmu"
	if err := session.Invite(&gosdk.CallOption{Offer: &offer, Codec: &codec}); err != nil {
		t.Fatalf("send webrtc invite: %v", err)
	}
	answer, err := session.ReadEvent(ctx)
	if err != nil {
		t.Fatalf("read webrtc answer: %v", err)
	}
	if answer.Event != compat.EventAnswer || answer.SDP == "" {
		t.Fatalf("unexpected webrtc answer: %+v", answer)
	}
	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer.SDP}); err != nil {
		t.Fatalf("set remote answer: %v", err)
	}
	if err := session.Hangup("done", "client"); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	if _, err := session.ReadEvent(ctx); err != nil {
		t.Fatalf("read hangup: %v", err)
	}
}

func newSDKClient(t *testing.T) (*gosdk.Client, func()) {
	t.Helper()
	cfg := config.Default()
	cfg.RecorderPath = t.TempDir()
	app := bootstrap.New(cfg)
	server := httptest.NewServer(app.Echo)
	client := gosdk.NewClient(gosdk.ClientOptions{HTTPBaseURL: server.URL})
	return client, server.Close
}

func newSDKWebRTCOffer(t *testing.T) (*webrtc.PeerConnection, string) {
	t.Helper()
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
		PayloadType:        0,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		t.Fatalf("register pcmu codec: %v", err)
	}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMA, ClockRate: 8000, Channels: 1},
		PayloadType:        8,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		t.Fatalf("register pcma codec: %v", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peer, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("create peer connection: %v", err)
	}
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	if err != nil {
		peer.Close()
		t.Fatalf("add transceiver: %v", err)
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
		t.Fatalf("set codec preferences: %v", err)
	}
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		peer.Close()
		t.Fatalf("create offer: %v", err)
	}
	if err := peer.SetLocalDescription(offer); err != nil {
		peer.Close()
		t.Fatalf("set local description: %v", err)
	}
	return peer, offer.SDP
}
