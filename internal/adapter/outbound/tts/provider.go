// 这个文件定义 TTS 提供商约束，负责把兼容配置里的 provider 名称解析成可用的合成实现。

package tts

import (
	"fmt"
	"strings"
)

const ProviderAliyun = "aliyun"
const ProviderTencent = "tencent"
const ProviderMock = "mock"

type Provider interface {
	Name() string
	MetricKey(prefix string) string
}

func ValidateProvider(name string) error {
	switch normalizeProvider(name) {
	case "", ProviderAliyun, ProviderTencent, ProviderMock:
		return nil
	default:
		return fmt.Errorf("unsupported tts provider: %s", name)
	}
}

func ResolveProvider(name string) Provider {
	switch normalizeProvider(name) {
	case ProviderAliyun:
		return AliyunProvider{}
	case ProviderTencent:
		return TencentProvider{}
	default:
		return MockProvider{}
	}
}

func normalizeProvider(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
