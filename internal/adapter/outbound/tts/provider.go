// 这个文件定义 TTS 提供商约束，负责把兼容配置里的 provider 名称解析成可用的流式合成实现。

package tts

import (
	"fmt"
	"strings"

	"gopbx/pkg/wsproto"
)

const ProviderAliyun = "aliyun"
const ProviderMock = "mock"

type Chunk struct {
	Data []byte
}

// Stream 表示一次文本合成任务产生的音频流。
// 对上层来说，它只需要不断取 chunk；底层无论是 mock 还是阿里云 SDK，都通过同一个接口暴露。
type Stream interface {
	Recv() (Chunk, error)
	Close() error
}

type Provider interface {
	Name() string
	MetricKey(prefix string) string
	StartSynthesis(text string, cfg *wsproto.SynthesisOption) (Stream, error)
}

func ValidateProvider(name string) error {
	switch normalizeProvider(name) {
	case "", ProviderAliyun, ProviderMock:
		return nil
	default:
		return fmt.Errorf("unsupported tts provider: %s", name)
	}
}

func ResolveProvider(name string) Provider {
	switch normalizeProvider(name) {
	case ProviderAliyun:
		return AliyunProvider{}
	default:
		return MockProvider{}
	}
}

func normalizeProvider(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
