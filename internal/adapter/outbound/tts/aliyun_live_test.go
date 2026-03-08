//go:build livealiyun

// 这个文件用于临时真实联调阿里云 TTS，验证 token + appkey 配置下能够稳定拿到音频流。

package tts

import (
	"errors"
	"io"
	"os"
	"testing"

	"gopbx/pkg/wsproto"
)

func TestLiveAliyunTTSWithToken(t *testing.T) {
	token := os.Getenv("ALIYUN_TOKEN")
	appKey := os.Getenv("ALIYUN_APPKEY")
	if token == "" || appKey == "" {
		t.Fatal("ALIYUN_TOKEN or ALIYUN_APPKEY is empty")
	}
	provider := AliyunProvider{}
	codec := "pcm"
	sampleRate := int32(16000)
	stream, err := provider.StartSynthesis("你好，今天天气怎么样。", &wsproto.SynthesisOption{
		AppID:      &appKey,
		Codec:      &codec,
		Samplerate: &sampleRate,
		Extra: map[string]string{
			"token": token,
		},
	})
	if err != nil {
		t.Fatalf("start live aliyun tts: %v", err)
	}
	defer stream.Close()

	var totalBytes int
	var chunkCount int
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("recv live aliyun tts chunk: %v", err)
		}
		totalBytes += len(chunk.Data)
		if len(chunk.Data) > 0 {
			chunkCount++
		}
	}
	t.Logf("aliyun tts live chunks=%d bytes=%d", chunkCount, totalBytes)
	if chunkCount == 0 || totalBytes == 0 {
		t.Fatalf("expected live aliyun tts to return audio, chunks=%d bytes=%d", chunkCount, totalBytes)
	}
}
