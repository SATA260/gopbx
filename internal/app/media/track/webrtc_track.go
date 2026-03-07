// 这个文件实现 WebRTC 音轨兼容壳，当前负责保存 offer/candidate 并生成稳定的 answer 占位内容。

package track

import (
	"fmt"
	"sync"

	"gopbx/internal/app/media/codec"
)

type WebRTCTrack struct {
	mu         sync.Mutex
	ID         string
	Offer      string
	Codec      codec.Type
	candidates []string
}

func NewWebRTCTrack(id, offer, codecName string) *WebRTCTrack {
	return &WebRTCTrack{ID: id, Offer: offer, Codec: codec.Parse(codecName)}
}

func (t *WebRTCTrack) BuildAnswer() string {
	if t == nil {
		return ""
	}
	if t.Offer != "" {
		return t.Offer
	}
	return fmt.Sprintf("v=0\r\ns=gopbx\r\na=rtpmap:0 %s/%d\r\n", t.Codec, codec.New(string(t.Codec)).SampleRate())
}

func (t *WebRTCTrack) AddCandidates(candidates []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.candidates = append(t.candidates, candidates...)
}

func (t *WebRTCTrack) Candidates() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.candidates))
	copy(out, t.candidates)
	return out
}
