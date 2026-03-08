// 这个文件实现阿里云 ASR 兼容适配器，当前先提供会话型 mock 识别结果，后续替换成真实实时识别客户端。

package asr

import "gopbx/pkg/wsproto"

type AliyunProvider struct{}

func (AliyunProvider) Name() string { return ProviderAliyun }

func (AliyunProvider) NewSession(_ *wsproto.ASRConfig) (Session, error) {
	return &mockSession{provider: ProviderAliyun}, nil
}
