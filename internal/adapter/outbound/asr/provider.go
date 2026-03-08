// 这个文件定义 ASR 提供商约束，负责把兼容配置里的 provider 名称解析成可用的流式识别实现。

package asr

import (
	"fmt"
	"strings"

	"gopbx/pkg/wsproto"
)

const ProviderAliyun = "aliyun"
const ProviderMock = "mock"

type Result struct {
	Final     bool
	Text      string
	StartTime int64
	EndTime   int64
}

// Session 表示一次会话级的流式识别上下文。
// 实时语音识别需要跨多个音频包维护状态，因此这里不再用“单包输入、单次返回”的接口。
type Session interface {
	WriteAudio(payload []byte) error
	Results() <-chan Result
	Errors() <-chan error
	Close() error
}

type Provider interface {
	Name() string
	NewSession(cfg *wsproto.ASRConfig) (Session, error)
}

func ValidateProvider(name string) error {
	switch normalizeProvider(name) {
	case "", ProviderAliyun, ProviderMock:
		return nil
	default:
		return fmt.Errorf("unsupported asr provider: %s", name)
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
