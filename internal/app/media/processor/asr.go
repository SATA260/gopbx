// 这个文件实现最小 ASR 处理器，用来把原始二进制音频帧送入会话型识别 provider，并异步转换成协议事件。

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
	mu        sync.Mutex
	trackID   string
	provider  asradapter.Provider
	config    *wsproto.ASRConfig
	session   asradapter.Session
	failed    bool
	index     uint32
	ttfbSent  bool
	silence   int
	emit      func([]protocol.Event) error
	onError   func(error)
	closeOnce sync.Once
}

func NewASR(trackID string, provider asradapter.Provider, cfg *wsproto.ASRConfig, emit func([]protocol.Event) error, onError func(error)) *ASR {
	if provider == nil {
		provider = asradapter.ResolveProvider("")
	}
	return &ASR{trackID: trackID, provider: provider, config: cfg, emit: emit, onError: onError}
}

func (a *ASR) Name() string { return "asr" }

// Process 的职责现在只剩“确保 session 已建立并持续送音频”。
// 识别结果由后台 goroutine 异步消费 provider 回调，然后主动推送成协议事件。
func (a *ASR) Process(packet mediaentity.Packet) []protocol.Event {
	if len(packet.Data) == 0 {
		return nil
	}
	if isSilentPCM16(packet.Data) {
		a.mu.Lock()
		a.silence++
		shouldStop := a.session != nil && a.silence >= 8
		a.mu.Unlock()
		if shouldStop {
			if err := a.stopActiveSession(); err != nil {
				return []protocol.Event{wsproto.NewErrorEvent(a.trackID, "asr", err.Error())}
			}
		}
		return nil
	}
	a.mu.Lock()
	a.silence = 0
	a.mu.Unlock()

	a.mu.Lock()
	session, err := a.ensureSessionLocked()
	a.mu.Unlock()
	if err != nil {
		return []protocol.Event{wsproto.NewErrorEvent(a.trackID, "asr", err.Error())}
	}
	if session == nil {
		return nil
	}
	if err := session.WriteAudio(packet.Data); err != nil {
		return []protocol.Event{wsproto.NewErrorEvent(a.trackID, "asr", err.Error())}
	}
	return nil
}

func (a *ASR) Close() error {
	var closeErr error
	a.closeOnce.Do(func() {
		a.mu.Lock()
		session := a.session
		a.session = nil
		a.mu.Unlock()
		if session != nil {
			closeErr = session.Close()
		}
	})
	return closeErr
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
	go a.forwardResults(session)
	return a.session, nil
}

func (a *ASR) stopActiveSession() error {
	a.mu.Lock()
	session := a.session
	a.session = nil
	a.ttfbSent = false
	a.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

// forwardResults 在后台异步消费 provider 结果。
// 真实阿里云识别是回调式返回，所以这里必须从 session 的结果通道持续取数据，而不能再等下一帧音频时顺带 drain。
func (a *ASR) forwardResults(session asradapter.Session) {
	for {
		select {
		case result, ok := <-session.Results():
			if !ok {
				return
			}
			a.emitResult(result)
		case err, ok := <-session.Errors():
			if !ok {
				return
			}
			if a.onError != nil && err != nil {
				a.onError(err)
			}
		}
	}
}

func (a *ASR) emitResult(result asradapter.Result) {
	now := wsproto.NowMillis()
	a.mu.Lock()
	events := make([]protocol.Event, 0, 2)
	if !a.ttfbSent {
		a.ttfbSent = true
		events = append(events, protocol.Event{
			Event:     compat.EventMetrics,
			Timestamp: now,
			Key:       "ttfb.asr." + a.provider.Name(),
			Duration:  wsproto.Uint64(0),
			Data: map[string]any{
				"trackId":  a.trackID,
				"provider": a.provider.Name(),
			},
		})
	}
	a.index++
	index := a.index
	a.mu.Unlock()

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
		Index:     wsproto.Uint32(index),
		StartTime: wsproto.Int64(startTime),
		EndTime:   wsproto.Int64(endTime),
		Text:      result.Text,
	})
	if a.emit != nil {
		if err := a.emit(events); err != nil && a.onError != nil {
			a.onError(err)
		}
	}
}

func isSilentPCM16(payload []byte) bool {
	if len(payload) < 2 {
		return true
	}
	for i := 0; i+1 < len(payload); i += 2 {
		sample := int16(uint16(payload[i]) | uint16(payload[i+1])<<8)
		if sample > 96 || sample < -96 {
			return false
		}
	}
	return true
}
