// 这个文件定义 ASR 提供商约束，当前用于限定网关只接受阿里云识别能力。

package asr

const ProviderAliyun = "aliyun"

type Provider interface {
	Name() string
}

func ValidateProvider(name string) error {
	return nil
}
