// 这个文件实现 WebRTC 音轨，负责完成真实 offer/answer 协商、接收后续 candidate，并把远端音频接入处理链。

package track

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	ttsadapter "gopbx/internal/adapter/outbound/tts"
	"gopbx/internal/app/media/codec"
	mediastream "gopbx/internal/app/media/stream"
	"gopbx/pkg/wsproto"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
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
	outbound   *webrtc.TrackLocalStaticSample
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

	outbound, err := webrtc.NewTrackLocalStaticSample(outboundCodecCapability(t.Codec), "tts", t.ID+"-tts")
	if err != nil {
		_ = peer.Close()
		return "", err
	}
	transceiver, err := peer.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendrecv,
	})
	if err != nil {
		_ = peer.Close()
		return "", err
	}
	if err := transceiver.Sender().ReplaceTrack(outbound); err != nil {
		_ = peer.Close()
		return "", err
	}
	go drainSenderRTCP(transceiver.Sender())
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
	t.outbound = outbound
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
	t.outbound = nil
	t.mu.Unlock()
	if peer == nil {
		return nil
	}
	return peer.Close()
}

// PlayTTS 会把合成得到的音频流写入 WebRTC 本地发送轨。
// 当前阶段只为 WebRTC 会话提供真实音频输出，普通 websocket 会话仍然只保留事件兼容行为。
func (t *WebRTCTrack) PlayTTS(trackID string, option *wsproto.SynthesisOption, stream ttsadapter.Stream) (audioBytes int, chunkCount int, err error) {
	if t == nil || stream == nil {
		return 0, 0, errors.New("webrtc tts stream is nil")
	}
	t.mu.Lock()
	outbound := t.outbound
	codecType := t.Codec
	t.mu.Unlock()
	if outbound == nil {
		return 0, 0, errors.New("webrtc outbound track is not ready")
	}
	defer func() {
		closeErr := stream.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return audioBytes, chunkCount, err
			}
			return audioBytes, chunkCount, recvErr
		}
		if len(chunk.Data) == 0 {
			continue
		}
		encoded := encodeTTSChunk(codecType, chunk.Data)
		if len(encoded) == 0 {
			continue
		}
		audioBytes += len(encoded)
		chunkCount++
		if writeErr := outbound.WriteSample(media.Sample{Data: encoded, Duration: sampleDuration(codecType, len(encoded), option)}); writeErr != nil {
			return audioBytes, chunkCount, writeErr
		}
	}
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
		normalized, normErr := normalizeInboundPayload(remote, packet.Payload)
		if normErr != nil {
			if t.onError != nil {
				t.onError(normErr)
			}
			return
		}
		if len(normalized) == 0 {
			continue
		}
		events := t.Stream.Push(mediastream.Packet{TrackID: t.ID, Data: normalized})
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

func outboundCodecCapability(kind codec.Type) webrtc.RTPCodecCapability {
	switch kind {
	case codec.PCMA:
		return webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMA, ClockRate: 8000, Channels: 1}
	default:
		return webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1}
	}
}

func sampleDuration(kind codec.Type, size int, option *wsproto.SynthesisOption) time.Duration {
	if size <= 0 {
		return 20 * time.Millisecond
	}
	sampleRate := codec.New(string(kind)).SampleRate()
	if option != nil && option.Samplerate != nil && *option.Samplerate > 0 {
		sampleRate = int(*option.Samplerate)
	}
	if sampleRate <= 0 {
		sampleRate = 8000
	}
	bytesPerSample := 1
	if kind == codec.PCMU || kind == codec.PCMA {
		bytesPerSample = 1
	} else {
		bytesPerSample = 2
	}
	sampleCount := size / bytesPerSample
	if sampleCount == 0 {
		sampleCount = 1
	}
	return time.Second * time.Duration(sampleCount) / time.Duration(sampleRate)
}

func drainSenderRTCP(sender *webrtc.RTPSender) {
	if sender == nil {
		return
	}
	buf := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buf); err != nil {
			return
		}
	}
}

// encodeTTSChunk 会把 provider 输出的 PCM 数据转成当前 WebRTC 出站 codec 所需的编码。
// 当前只保留 G.711 路径，其他 codec 会被解析层统一回退到 PCMU。
func encodeTTSChunk(kind codec.Type, payload []byte) []byte {
	return codec.New(string(kind)).Encode(payload)
}

// normalizeInboundPayload 会把 WebRTC 入站音频统一成 16k PCM16 little-endian。
// 目前明确支持 PCMU/PCMA；如果对端协商成其他 codec，就尽早报错，避免把错误格式直接喂给 ASR。
func normalizeInboundPayload(remote *webrtc.TrackRemote, payload []byte) ([]byte, error) {
	codecType, ok := codec.FromWebRTCMime(remote.Codec().MimeType)
	if !ok {
		return nil, fmt.Errorf("unsupported inbound webrtc codec: %s", remote.Codec().MimeType)
	}
	decoded := codec.New(string(codecType)).Decode(payload)
	if len(decoded) == 0 {
		return nil, nil
	}
	srcRate := int(remote.Codec().ClockRate)
	if srcRate <= 0 {
		srcRate = codec.New(string(codecType)).SampleRate()
	}
	return codec.ResamplePCM16LE(decoded, srcRate, 16000), nil
}
