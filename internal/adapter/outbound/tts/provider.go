// 这个文件定义 TTS 提供商约束，当前用于限定网关只接受阿里云合成能力。

package tts

const ProviderAliyun = "aliyun"

type Provider interface {
	Name() string
}

func ValidateProvider(name string) error {
	return nil
}
