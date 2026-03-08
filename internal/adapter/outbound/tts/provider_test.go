// 这个文件验证 TTS provider 名称校验和流式接口，确保兼容配置里的常见 provider 可以正常建流并输出音频块。

package tts

import (
	"errors"
	"io"
	"testing"
)

func TestValidateProvider(t *testing.T) {
	for _, name := range []string{"", ProviderAliyun, ProviderMock} {
		if err := ValidateProvider(name); err != nil {
			t.Fatalf("expected provider %q to be valid: %v", name, err)
		}
	}
	if err := ValidateProvider("unknown"); err == nil {
		t.Fatal("expected unknown provider to fail validation")
	}
}

func TestResolveProvider(t *testing.T) {
	if provider := ResolveProvider(ProviderAliyun); provider.MetricKey("ttfb") != "ttfb.tts.aliyun" {
		t.Fatalf("unexpected aliyun metric key: %s", provider.MetricKey("ttfb"))
	}
	if provider := ResolveProvider("unknown"); provider.Name() != ProviderMock {
		t.Fatalf("unexpected fallback provider: %s", provider.Name())
	}
}

func TestProviderStreamLifecycle(t *testing.T) {
	providers := []Provider{ResolveProvider(ProviderAliyun), ResolveProvider(ProviderMock)}
	for _, provider := range providers {
		stream, err := provider.StartSynthesis("hello", nil)
		if err != nil {
			t.Fatalf("start synthesis for %s: %v", provider.Name(), err)
		}
		chunk, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv first chunk for %s: %v", provider.Name(), err)
		}
		if len(chunk.Data) == 0 {
			t.Fatalf("expected non-empty chunk for %s", provider.Name())
		}
		_, err = stream.Recv()
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected EOF for %s, got %v", provider.Name(), err)
		}
		if err := stream.Close(); err != nil {
			t.Fatalf("close stream for %s: %v", provider.Name(), err)
		}
	}
}
