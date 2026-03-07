// 这个文件实现默认 TTS 兼容适配器，在未指定 provider 时提供稳定的指标命名和 provider 标识。

package tts

type MockProvider struct{}

func (MockProvider) Name() string { return "mock" }

func (MockProvider) MetricKey(prefix string) string {
	return prefix + ".tts.mock"
}
