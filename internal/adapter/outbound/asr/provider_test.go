// 这个文件验证 ASR provider 名称校验和 session 式接口，确保兼容配置里的常见 provider 可以正常建会和产出结果。

package asr

import "testing"

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
	if provider := ResolveProvider(ProviderAliyun); provider.Name() != ProviderAliyun {
		t.Fatalf("unexpected aliyun provider: %s", provider.Name())
	}
	if provider := ResolveProvider("unknown"); provider.Name() != ProviderMock {
		t.Fatalf("unexpected fallback provider: %s", provider.Name())
	}
}

func TestProviderSessionLifecycle(t *testing.T) {
	providers := []Provider{ResolveProvider(ProviderAliyun), ResolveProvider(ProviderMock)}
	for _, provider := range providers {
		session, err := provider.NewSession(nil)
		if err != nil {
			t.Fatalf("create session for %s: %v", provider.Name(), err)
		}
		results, err := session.WriteAudio([]byte{0x01, 0x02})
		if err != nil {
			t.Fatalf("write audio for %s: %v", provider.Name(), err)
		}
		if len(results) != 1 || !results[0].Final {
			t.Fatalf("unexpected results for %s: %+v", provider.Name(), results)
		}
		if results[0].Text == "" {
			t.Fatalf("expected non-empty result text for %s", provider.Name())
		}
		if err := session.Close(); err != nil {
			t.Fatalf("close session for %s: %v", provider.Name(), err)
		}
	}
}
