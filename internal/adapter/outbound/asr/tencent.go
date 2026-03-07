// 这个文件实现腾讯云 ASR 兼容适配器，当前先提供稳定的 mock 转写结果和 provider 标识。

package asr

import "fmt"

type TencentProvider struct{}

func (TencentProvider) Name() string { return "tencent" }

func (TencentProvider) Transcribe(_ []byte, index uint32) string {
	return fmt.Sprintf("tencent asr final %d", index)
}
