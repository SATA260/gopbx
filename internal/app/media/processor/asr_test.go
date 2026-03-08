// 这个文件验证 ASR 处理器会话化后的行为，确保它能建 session、异步输出事件并在关闭时释放资源。

package processor

import (
	"sync"
	"testing"
	"time"

	asradapter "gopbx/internal/adapter/outbound/asr"
	"gopbx/internal/compat"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
	"gopbx/pkg/wsproto"
)

type stubSession struct {
	closed  bool
	results chan asradapter.Result
	errs    chan error
}

func (s *stubSession) WriteAudio(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	s.results <- asradapter.Result{Final: true, Text: "stub final"}
	return nil
}

func (s *stubSession) Results() <-chan asradapter.Result { return s.results }

func (s *stubSession) Errors() <-chan error { return s.errs }

func (s *stubSession) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.results)
	close(s.errs)
	return nil
}

type stubProvider struct {
	name    string
	session *stubSession
}

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) NewSession(_ *wsproto.ASRConfig) (asradapter.Session, error) {
	p.session = &stubSession{results: make(chan asradapter.Result, 4), errs: make(chan error, 1)}
	return p.session, nil
}

func TestASRProcessorUsesSessionAndClosesIt(t *testing.T) {
	provider := &stubProvider{name: "stub"}
	var (
		mu     sync.Mutex
		events []protocol.Event
	)
	processor := NewASR(
		"track-1",
		provider,
		nil,
		func(got []protocol.Event) error {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, got...)
			return nil
		},
		nil,
	)
	returned := processor.Process(mediaentity.Packet{TrackID: "track-1", Data: []byte{0x01, 0x02}})
	if len(returned) != 0 {
		t.Fatalf("expected async processor to return no direct events, got %d", len(returned))
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		count := len(events)
		mu.Unlock()
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for async asr events, got %d", count)
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
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
