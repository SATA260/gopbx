// 这个文件实现最小 ASR 处理器，用来把原始二进制音频帧送入会话型识别 provider，并转换成协议事件。

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
	started  bool
	provider asradapter.Provider
	config   *wsproto.ASRConfig
	session  asradapter.Session
	failed   bool
	index    uint32
}

func NewASR(trackID string, provider asradapter.Provider, cfg *wsproto.ASRConfig) *ASR {
	if provider == nil {
		provider = asradapter.ResolveProvider("")
	}
	return &ASR{trackID: trackID, provider: provider, config: cfg}
}

func (a *ASR) Name() string { return "asr" }

// Process 会在第一次收到音频时创建识别 session，后续持续写入音频流。
// 这样后面接阿里云实时识别时，只需要替换 provider/session 实现，不需要再改音频链和事件拼装逻辑。
func (a *ASR) Process(packet mediaentity.Packet) []protocol.Event {
	if len(packet.Data) == 0 {
		return nil
	}

	a.mu.Lock()
	firstPacket := !a.started
	a.started = true
	session, err := a.ensureSessionLocked()
	a.mu.Unlock()
	if err != nil {
		return []protocol.Event{wsproto.NewErrorEvent(a.trackID, "asr", err.Error())}
	}
	if session == nil {
		return nil
	}

	results, err := session.WriteAudio(packet.Data)
	if err != nil {
		return []protocol.Event{wsproto.NewErrorEvent(a.trackID, "asr", err.Error())}
	}

	now := wsproto.NowMillis()
	events := make([]protocol.Event, 0, len(results)+1)
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

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, result := range results {
		a.index++
		startTime := result.StartTime
		endTime := result.EndTime
		if startTime == 0 {
			startTime = now
		}
		if endTime == 0 {
			endTime = now
		}
		eventName := compat.EventASRDelta
		if result.Final {
			eventName = compat.EventASRFinal
		}
		events = append(events, protocol.Event{
			Event:     eventName,
			TrackID:   a.trackID,
			Timestamp: now,
			Index:     wsproto.Uint32(a.index),
			StartTime: wsproto.Int64(startTime),
			EndTime:   wsproto.Int64(endTime),
			Text:      result.Text,
		})
	}
	return events
}

func (a *ASR) Close() error {
	a.mu.Lock()
	session := a.session
	a.session = nil
	a.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

func (a *ASR) ensureSessionLocked() (asradapter.Session, error) {
	if a.failed {
		return nil, nil
	}
	if a.session != nil {
		return a.session, nil
	}
	session, err := a.provider.NewSession(a.config)
	if err != nil {
		a.failed = true
		return nil, err
	}
	a.session = session
	return a.session, nil
}
