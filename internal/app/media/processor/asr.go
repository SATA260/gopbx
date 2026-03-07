// 这个文件实现最小 ASR 处理器，用来把原始二进制音频帧转换成兼容协议的 mock ASR 事件。

package processor

import (
	"sync"

	asradapter "gopbx/internal/adapter/outbound/asr"
	"gopbx/internal/compat"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
	"gopbx/pkg/wsproto"
)

type ASR struct {
	mu       sync.Mutex
	trackID  string
	index    uint32
	started  bool
	provider asradapter.Provider
}

func NewASR(trackID string, provider asradapter.Provider) *ASR {
	if provider == nil {
		provider = asradapter.ResolveProvider("")
	}
	return &ASR{trackID: trackID, provider: provider}
}

func (a *ASR) Name() string { return "asr" }

// Process 会按 provider 生成兼容风格的 ASR 事件；当前仍是 mock 结果，但指标名和文本来源已经按 provider 分流。
func (a *ASR) Process(packet mediaentity.Packet) []protocol.Event {
	if len(packet.Data) == 0 {
		return nil
	}

	a.mu.Lock()
	a.index++
	index := a.index
	firstPacket := !a.started
	a.started = true
	a.mu.Unlock()

	now := wsproto.NowMillis()
	events := make([]protocol.Event, 0, 2)
	if firstPacket {
		events = append(events, protocol.Event{
			Event:     compat.EventMetrics,
			Timestamp: now,
			Key:       "ttfb.asr." + a.provider.Name(),
			Duration:  wsproto.Uint64(0),
			Data: map[string]any{
				"trackId":  packet.TrackID,
				"provider": a.provider.Name(),
				"bytes":    len(packet.Data),
			},
		})
	}
	events = append(events, protocol.Event{
		Event:     compat.EventASRFinal,
		TrackID:   a.trackID,
		Timestamp: now,
		Index:     wsproto.Uint32(index),
		StartTime: wsproto.Int64(now),
		EndTime:   wsproto.Int64(now),
		Text:      a.provider.Transcribe(packet.Data, index),
	})
	return events
}
