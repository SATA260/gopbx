// 这个文件验证编解码工厂和轻量兼容壳，确保 codec 名称解析、采样率和字节透传行为稳定。

package codec

import "testing"

func TestParseAndNewCodec(t *testing.T) {
	tests := []struct {
		name       string
		wantType   Type
		wantRate   int
		wantEncode []byte
	}{
		{name: "pcmu", wantType: PCMU, wantRate: 8000, wantEncode: []byte{1, 2}},
		{name: "pcma", wantType: PCMA, wantRate: 8000, wantEncode: []byte{3, 4}},
		{name: "g722", wantType: G722, wantRate: 16000, wantEncode: []byte{5, 6}},
	}

	for _, test := range tests {
		codec := New(test.name)
		if codec.Type() != test.wantType {
			t.Fatalf("unexpected codec type for %s: %s", test.name, codec.Type())
		}
		if codec.SampleRate() != test.wantRate {
			t.Fatalf("unexpected codec sample rate for %s: %d", test.name, codec.SampleRate())
		}
		encoded := codec.Encode(test.wantEncode)
		decoded := codec.Decode(encoded)
		if string(decoded) != string(test.wantEncode) {
			t.Fatalf("unexpected roundtrip payload for %s: %v", test.name, decoded)
		}
	}
}
