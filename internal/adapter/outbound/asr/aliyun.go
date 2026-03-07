// 这个文件实现阿里云 ASR 兼容适配器，当前先提供稳定的 mock 转写结果和 provider 标识。

package asr

import "fmt"

type AliyunProvider struct{}

func (AliyunProvider) Name() string { return "aliyun" }

func (AliyunProvider) Transcribe(_ []byte, index uint32) string {
	return fmt.Sprintf("aliyun asr final %d", index)
}
