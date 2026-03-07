// 这个文件定义 ASR 提供商约束，负责把兼容配置里的 provider 名称解析成可用的转写实现。

package asr

import (
	"fmt"
	"strings"
)

const ProviderAliyun = "aliyun"
const ProviderTencent = "tencent"
const ProviderMock = "mock"

type Provider interface {
	Name() string
	Transcribe(payload []byte, index uint32) string
}

func ValidateProvider(name string) error {
	switch normalizeProvider(name) {
	case "", ProviderAliyun, ProviderTencent, ProviderMock:
		return nil
	default:
		return fmt.Errorf("unsupported asr provider: %s", name)
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
