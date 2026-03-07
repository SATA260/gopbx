// 这个文件定义 ASR 提供商约束，负责在迁移阶段统一校验兼容配置里出现的 provider 名称。

package asr

import "fmt"

const ProviderAliyun = "aliyun"
const ProviderTencent = "tencent"
const ProviderMock = "mock"

type Provider interface {
	Name() string
}

func ValidateProvider(name string) error {
	switch name {
	case "", ProviderAliyun, ProviderTencent, ProviderMock:
		return nil
	default:
		return fmt.Errorf("unsupported asr provider: %s", name)
	}
}
