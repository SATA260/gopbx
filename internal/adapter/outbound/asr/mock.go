// 这个文件实现默认 ASR 兼容适配器，在未指定 provider 时提供稳定的会话型 mock 识别结果。

package asr

import (
	"fmt"
	"sync"

	"gopbx/pkg/wsproto"
)

type MockProvider struct{}

func (MockProvider) Name() string { return ProviderMock }

func (MockProvider) NewSession(_ *wsproto.ASRConfig) (Session, error) {
	return &mockSession{provider: ProviderMock, results: make(chan Result, 16), errs: make(chan error, 4)}, nil
}

// mockSession 用来在接入真实云服务前模拟“会话持续存在、音频连续写入、结果逐步返回”的调用模式。
type mockSession struct {
	provider string
	index    uint32
	closed   bool
	results  chan Result
	errs     chan error
	mu       sync.Mutex
}

func (s *mockSession) WriteAudio(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(payload) == 0 {
		return nil
	}
	s.index++
	s.results <- Result{
		Final: true,
		Text:  fmt.Sprintf("%s asr final %d", s.provider, s.index),
	}
	return nil
}

func (s *mockSession) Results() <-chan Result { return s.results }

func (s *mockSession) Errors() <-chan error { return s.errs }

func (s *mockSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.results)
	close(s.errs)
	return nil
}
