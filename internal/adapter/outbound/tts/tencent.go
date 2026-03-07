// 这个文件实现腾讯云 TTS 兼容适配器，当前先提供稳定的 provider 标识和指标命名。

package tts

type TencentProvider struct{}

func (TencentProvider) Name() string { return "tencent" }

func (TencentProvider) MetricKey(prefix string) string {
	return prefix + ".tts.tencent"
}
