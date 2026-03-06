// 这个文件是阿里云 ASR 适配器占位，后续接入实时语音识别能力。

package asr

type AliyunProvider struct{}

func (AliyunProvider) Name() string { return "aliyun" }
