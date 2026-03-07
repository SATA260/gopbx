// 这个文件实现默认 ASR 兼容适配器，在未指定 provider 时提供稳定的 mock 转写结果。

package asr

import "fmt"

type MockProvider struct{}

func (MockProvider) Name() string { return "mock" }

func (MockProvider) Transcribe(_ []byte, index uint32) string {
	return fmt.Sprintf("mock asr final %d", index)
}
