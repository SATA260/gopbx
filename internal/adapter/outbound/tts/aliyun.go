// 这个文件实现阿里云 TTS 兼容适配器，当前先提供稳定的 provider 标识和指标命名。

package tts

type AliyunProvider struct{}

func (AliyunProvider) Name() string { return "aliyun" }

func (AliyunProvider) MetricKey(prefix string) string {
	return prefix + ".tts.aliyun"
}
