// 这个文件实现最小 ASR 处理器，用来把原始二进制音频帧转换成兼容协议的 mock ASR 事件。

package processor

import (
	"fmt"
	"sync"

	"gopbx/internal/compat"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
	"gopbx/pkg/wsproto"
)

type ASR struct {
	mu      sync.Mutex
	trackID string
	index   uint32
	started bool
}

func NewASR(trackID string) *ASR {
	return &ASR{trackID: trackID}
}

func (a *ASR) Name() string { return "asr" }

// Process 这里先用 mock 事件复刻调用链节奏：首个音频包产出 ttfb.asr.mock，随后每个非空包产出 asrFinal。
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
			Key:       "ttfb.asr.mock",
			Duration:  wsproto.Uint64(0),
			Data: map[string]any{
				"trackId": packet.TrackID,
				"bytes":   len(packet.Data),
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
		Text:      fmt.Sprintf("mock asr final %d", index),
	})
	return events
}
