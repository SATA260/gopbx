// 这个文件验证 ASR 处理器会话化后的行为，确保它能建 session、输出事件并在关闭时释放资源。

package processor

import (
	"testing"

	asradapter "gopbx/internal/adapter/outbound/asr"
	"gopbx/internal/compat"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/pkg/wsproto"
)

type stubSession struct {
	closed bool
}

func (s *stubSession) WriteAudio(payload []byte) ([]asradapter.Result, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	return []asradapter.Result{{Final: true, Text: "stub final"}}, nil
}

func (s *stubSession) Close() error {
	s.closed = true
	return nil
}

type stubProvider struct {
	name    string
	session *stubSession
}

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) NewSession(_ *wsproto.ASRConfig) (asradapter.Session, error) {
	p.session = &stubSession{}
	return p.session, nil
}

func TestASRProcessorUsesSessionAndClosesIt(t *testing.T) {
	provider := &stubProvider{name: "stub"}
	processor := NewASR("track-1", provider, nil)
	events := processor.Process(mediaentity.Packet{TrackID: "track-1", Data: []byte{0x01, 0x02}})
	if len(events) != 2 {
		t.Fatalf("expected metrics + final events, got %d", len(events))
	}
	if events[0].Event != compat.EventMetrics {
		t.Fatalf("unexpected first event: %s", events[0].Event)
	}
	if events[1].Event != compat.EventASRFinal || events[1].Text != "stub final" {
		t.Fatalf("unexpected final event: %+v", events[1])
	}
	if err := processor.Close(); err != nil {
		t.Fatalf("close processor: %v", err)
	}
	if provider.session == nil || !provider.session.closed {
		t.Fatal("expected underlying session to be closed")
	}
}
