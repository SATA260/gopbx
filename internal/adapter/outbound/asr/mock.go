// 这个文件实现默认 ASR 兼容适配器，在未指定 provider 时提供稳定的会话型 mock 识别结果。

package asr

import (
	"fmt"

	"gopbx/pkg/wsproto"
)

type MockProvider struct{}

func (MockProvider) Name() string { return ProviderMock }

func (MockProvider) NewSession(_ *wsproto.ASRConfig) (Session, error) {
	return &mockSession{provider: ProviderMock}, nil
}

// mockSession 用来在接入真实云服务前模拟“会话持续存在、音频连续写入、结果逐步返回”的调用模式。
type mockSession struct {
	provider string
	index    uint32
	closed   bool
}

func (s *mockSession) WriteAudio(payload []byte) ([]Result, error) {
	if s.closed || len(payload) == 0 {
		return nil, nil
	}
	s.index++
	return []Result{{
		Final: true,
		Text:  fmt.Sprintf("%s asr final %d", s.provider, s.index),
	}}, nil
}

func (s *mockSession) Close() error {
	s.closed = true
	return nil
}
