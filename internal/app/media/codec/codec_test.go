// 这个文件验证编解码工厂和真实 G.711 编解码行为，确保 codec 解析、采样率和 PCM 往返转换稳定。

package codec

import (
	"encoding/binary"
	"testing"
)

func TestParseAndNewCodec(t *testing.T) {
	tests := []struct {
		name     string
		wantType Type
		wantRate int
	}{
		{name: "pcmu", wantType: PCMU, wantRate: 8000},
		{name: "pcma", wantType: PCMA, wantRate: 8000},
	}

	for _, test := range tests {
		codec := New(test.name)
		if codec.Type() != test.wantType {
			t.Fatalf("unexpected codec type for %s: %s", test.name, codec.Type())
		}
		if codec.SampleRate() != test.wantRate {
			t.Fatalf("unexpected codec sample rate for %s: %d", test.name, codec.SampleRate())
		}
	}
}

func TestFromWebRTCMime(t *testing.T) {
	if kind, ok := FromWebRTCMime("audio/PCMU"); !ok || kind != PCMU {
		t.Fatalf("unexpected pcmu mime parse result: ok=%v kind=%s", ok, kind)
	}
	if kind, ok := FromWebRTCMime("audio/pcma"); !ok || kind != PCMA {
		t.Fatalf("unexpected pcma mime parse result: ok=%v kind=%s", ok, kind)
	}
	if _, ok := FromWebRTCMime("audio/opus"); ok {
		t.Fatal("expected opus mime to be unsupported")
	}
}

func TestPCMUCodecEncodeDecode(t *testing.T) {
	codec := PCMUCodec{}
	pcm := pcmSamplesToBytes([]int16{-12000, -1000, 0, 1000, 12000})
	encoded := codec.Encode(pcm)
	if len(encoded) != len(pcm)/2 {
		t.Fatalf("unexpected pcmu encoded length: %d", len(encoded))
	}
	if encoded[2] != 0xff {
		t.Fatalf("expected silence to encode as 0xff, got %#x", encoded[2])
	}
	decoded := codec.Decode(encoded)
	assertPCMShape(t, pcm, decoded)
}

func TestPCMACodecEncodeDecode(t *testing.T) {
	codec := PCMACodec{}
	pcm := pcmSamplesToBytes([]int16{-12000, -1000, 0, 1000, 12000})
	encoded := codec.Encode(pcm)
	if len(encoded) != len(pcm)/2 {
		t.Fatalf("unexpected pcma encoded length: %d", len(encoded))
	}
	if encoded[2] != 0xd5 {
		t.Fatalf("expected silence to encode as 0xd5, got %#x", encoded[2])
	}
	decoded := codec.Decode(encoded)
	assertPCMShape(t, pcm, decoded)
}

func TestResamplePCM16LE(t *testing.T) {
	payload := pcmSamplesToBytes([]int16{1, 2, 3, 4})
	resampled := ResamplePCM16LE(payload, 8000, 16000)
	samples := pcmBytesToSamples(resampled)
	if len(samples) != 8 {
		t.Fatalf("unexpected resampled sample count: %d", len(samples))
	}
	if samples[0] != 1 || samples[1] != 1 || samples[6] != 4 || samples[7] != 4 {
		t.Fatalf("unexpected resampled samples: %v", samples)
	}
}

func assertPCMShape(t *testing.T, original, decoded []byte) {
	t.Helper()
	if len(decoded) != len(original) {
		t.Fatalf("unexpected decoded length: %d", len(decoded))
	}
	origSamples := pcmBytesToSamples(original)
	decodedSamples := pcmBytesToSamples(decoded)
	for i := range origSamples {
		if origSamples[i] == 0 {
			if abs16(decodedSamples[i]) > 1024 {
				t.Fatalf("expected near-zero decoded sample, got %d", decodedSamples[i])
			}
			continue
		}
		if sign16(origSamples[i]) != sign16(decodedSamples[i]) {
			t.Fatalf("expected sign to be preserved at index %d: orig=%d decoded=%d", i, origSamples[i], decodedSamples[i])
		}
	}
	_ = binary.LittleEndian.Uint16(decoded[:2])
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

func sign16(v int16) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}
