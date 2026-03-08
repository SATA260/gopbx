// 这个文件实现默认 TTS 兼容适配器，在未指定 provider 时提供稳定的流式 mock 音频输出。

package tts

import (
	"io"
	"strings"

	"gopbx/pkg/wsproto"
)

type MockProvider struct{}

func (MockProvider) Name() string { return ProviderMock }

func (MockProvider) MetricKey(prefix string) string {
	return prefix + ".tts.mock"
}

func (MockProvider) StartSynthesis(text string, _ *wsproto.SynthesisOption) (Stream, error) {
	data := mockPCM(text)
	return &mockStream{chunks: []Chunk{{Data: data}}}, nil
}

type mockStream struct {
	chunks []Chunk
	closed bool
}

func (s *mockStream) Recv() (Chunk, error) {
	if s.closed {
		return Chunk{}, io.EOF
	}
	if len(s.chunks) == 0 {
		s.closed = true
		return Chunk{}, io.EOF
	}
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]
	return chunk, nil
}

func (s *mockStream) Close() error {
	s.closed = true
	s.chunks = nil
	return nil
}

func mockPCM(text string) []byte {
	if strings.TrimSpace(text) == "" {
		return []byte{0x00, 0x00, 0x00, 0x00}
	}
	payload := make([]byte, 0, len(text)*4)
	for i, r := range text {
		sample := int16(((i % 16) + 1) * 256)
		if r%2 == 0 {
			sample = -sample
		}
		payload = append(payload, byte(sample), byte(sample>>8))
		payload = append(payload, 0x00, 0x00)
	}
	return payload
}
