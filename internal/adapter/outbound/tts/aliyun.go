// 这个文件是阿里云 TTS 适配器占位，后续接入流式语音合成能力。

package tts

type AliyunProvider struct{}

func (AliyunProvider) Name() string { return "aliyun" }
