// 这个文件验证 ASR provider 名称校验，确保兼容配置里的常见 provider 可以通过而非法值会被拦截。

package asr

import "testing"

func TestValidateProvider(t *testing.T) {
	for _, name := range []string{"", ProviderAliyun, ProviderTencent, ProviderMock} {
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
	if provider := ResolveProvider(ProviderTencent); provider.Name() != ProviderTencent {
		t.Fatalf("unexpected tencent provider: %s", provider.Name())
	}
	if provider := ResolveProvider("unknown"); provider.Name() != ProviderMock {
		t.Fatalf("unexpected fallback provider: %s", provider.Name())
	}
}
