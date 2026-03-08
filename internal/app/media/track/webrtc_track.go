// 这个文件实现 WebRTC 音轨，负责完成真实 offer/answer 协商、接收后续 candidate，并把远端音频接入处理链。

package track

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"gopbx/internal/app/media/codec"
	mediastream "gopbx/internal/app/media/stream"
	"gopbx/pkg/wsproto"

	"github.com/pion/webrtc/v4"
)

type WebRTCTrack struct {
	mu         sync.Mutex
	ID         string
	Offer      string
	Codec      codec.Type
	ICEServers []wsproto.ICEServer
	Stream     *mediastream.Stream
	onEvents   func([]wsproto.EventEnvelope) error
	onError    func(error)

	peer       *webrtc.PeerConnection
	answer     string
	candidates []string
}

// NewWebRTCTrack 只做配置装配，不在构造函数里立刻建连。
// 这样可以把“创建对象”和“真正执行 SDP 协商”拆开，便于上层在失败时统一走会话错误收敛。
func NewWebRTCTrack(id, offer, codecName string, iceServers []wsproto.ICEServer, mediaStream *mediastream.Stream, onEvents func([]wsproto.EventEnvelope) error, onError func(error)) *WebRTCTrack {
	return &WebRTCTrack{
		ID:         id,
		Offer:      offer,
		Codec:      codec.Parse(codecName),
		ICEServers: iceServers,
		Stream:     mediaStream,
		onEvents:   onEvents,
		onError:    onError,
	}
}

// BuildAnswer 会基于远端 offer 创建真实的 PeerConnection，并返回包含本端 candidate 的 answer SDP。
// 这里会等待一小段时间让 ICE 收集尽量完成，这样现有客户端即使不依赖服务端 trickle candidate 事件，也能直接连通。
func (t *WebRTCTrack) BuildAnswer() (string, error) {
	if t == nil {
		return "", errors.New("webrtc track is nil")
	}
	if strings.TrimSpace(t.Offer) == "" {
		return "", errors.New("webrtc offer is empty")
	}
	if t.answer != "" {
		return t.answer, nil
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return "", err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peer, err := api.NewPeerConnection(webrtc.Configuration{ICEServers: convertICEServers(t.ICEServers)})
	if err != nil {
		return "", err
	}

	// 当前阶段先以接收音频为主，后续真实 TTS 出站时再把发送轨补到同一个 PeerConnection 上。
	if _, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		_ = peer.Close()
		return "", err
	}
	peer.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		go t.consumeRemoteTrack(remote)
	})

	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: t.Offer}
	if err := peer.SetRemoteDescription(offer); err != nil {
		_ = peer.Close()
		return "", err
	}
	answer, err := peer.CreateAnswer(nil)
	if err != nil {
		_ = peer.Close()
		return "", err
	}
	if err := peer.SetLocalDescription(answer); err != nil {
		_ = peer.Close()
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peer)
	select {
	case <-gatherComplete:
	case <-time.After(3 * time.Second):
	}

	local := peer.LocalDescription()
	if local == nil {
		_ = peer.Close()
		return "", errors.New("webrtc local description is empty")
	}
	t.mu.Lock()
	t.peer = peer
	t.answer = local.SDP
	t.mu.Unlock()
	return t.answer, nil
}

// AddCandidates 兼容两种输入形态：
// 1. 纯 candidate 行字符串；2. JSON 序列化后的 ICECandidateInit。
// 这样既能兼容传统信令透传，也能兼容更完整的 WebRTC trickle candidate 用法。
func (t *WebRTCTrack) AddCandidates(candidates []string) error {
	if t == nil {
		return errors.New("webrtc track is nil")
	}
	t.mu.Lock()
	peer := t.peer
	t.mu.Unlock()
	if peer == nil {
		return errors.New("webrtc peer connection is not ready")
	}
	for _, raw := range candidates {
		candidate, err := parseCandidate(raw)
		if err != nil {
			return err
		}
		if err := peer.AddICECandidate(candidate); err != nil {
			return err
		}
		t.mu.Lock()
		t.candidates = append(t.candidates, raw)
		t.mu.Unlock()
	}
	return nil
}

func (t *WebRTCTrack) Candidates() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.candidates))
	copy(out, t.candidates)
	return out
}

func (t *WebRTCTrack) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	peer := t.peer
	t.peer = nil
	t.mu.Unlock()
	if peer == nil {
		return nil
	}
	return peer.Close()
}

// consumeRemoteTrack 会持续读取远端 RTP 包，并把 payload 作为上行音频送入处理链。
// 当前处理链里的 ASR 还是 mock backend，但这条链路已经是真实 WebRTC 媒体输入，而不是伪造事件。
func (t *WebRTCTrack) consumeRemoteTrack(remote *webrtc.TrackRemote) {
	if t == nil || remote == nil || t.Stream == nil || t.onEvents == nil {
		return
	}
	for {
		packet, _, err := remote.ReadRTP()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if t.onError != nil {
				t.onError(err)
			}
			return
		}
		if len(packet.Payload) == 0 {
			continue
		}
		events := t.Stream.Push(mediastream.Packet{TrackID: t.ID, Data: packet.Payload})
		if len(events) == 0 {
			continue
		}
		if err := t.onEvents(events); err != nil {
			if t.onError != nil {
				t.onError(err)
			}
			return
		}
	}
}

func convertICEServers(servers []wsproto.ICEServer) []webrtc.ICEServer {
	out := make([]webrtc.ICEServer, 0, len(servers))
	for _, server := range servers {
		item := webrtc.ICEServer{URLs: server.URLs}
		if server.Username != nil {
			item.Username = *server.Username
		}
		if server.Credential != nil {
			item.Credential = *server.Credential
		} else if server.Password != nil {
			item.Credential = *server.Password
		}
		out = append(out, item)
	}
	return out
}

func parseCandidate(raw string) (webrtc.ICECandidateInit, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return webrtc.ICECandidateInit{}, errors.New("candidate is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal([]byte(trimmed), &candidate); err != nil {
			return webrtc.ICECandidateInit{}, err
		}
		return candidate, nil
	}
	mid := "0"
	line := uint16(0)
	return webrtc.ICECandidateInit{Candidate: trimmed, SDPMid: &mid, SDPMLineIndex: &line}, nil
}
