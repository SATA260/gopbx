package vad

import (
	"testing"

	"gopbx/pkg/wsproto"
)

func TestValidateOptionAcceptsNativeEndpoint(t *testing.T) {
	option := &wsproto.VADOption{
		Type:       stringPtr(TypeHybrid),
		Samplerate: uint32Ptr(16000),
		Endpoint:   stringPtr("native:///opt/models/silero_vad.onnx"),
	}
	if err := ValidateOption(option); err != nil {
		t.Fatalf("validate native endpoint: %v", err)
	}
}

func TestResolveScorerTargetUsesEnvForNative(t *testing.T) {
	t.Setenv(NativeModelPathEnv, "/models/silero_vad.onnx")
	target, err := resolveScorerTarget(&wsproto.VADOption{Type: stringPtr(TypeHybrid)})
	if err != nil {
		t.Fatalf("resolve native target: %v", err)
	}
	if target.scheme != SchemeNative {
		t.Fatalf("unexpected scheme: %s", target.scheme)
	}
	if target.native.ModelPath != "/models/silero_vad.onnx" {
		t.Fatalf("unexpected model path: %s", target.native.ModelPath)
	}
}

func TestResolveScorerTargetDetectsHTTP(t *testing.T) {
	target, err := resolveScorerTarget(&wsproto.VADOption{
		Type:     stringPtr(TypeHybrid),
		Endpoint: stringPtr("http://127.0.0.1:8091"),
	})
	if err != nil {
		t.Fatalf("resolve http target: %v", err)
	}
	if target.scheme != SchemeHTTP {
		t.Fatalf("unexpected scheme: %s", target.scheme)
	}
	if target.endpoint != "http://127.0.0.1:8091" {
		t.Fatalf("unexpected endpoint: %s", target.endpoint)
	}
}

func TestResolveScorerTargetAcceptsPlainModelPath(t *testing.T) {
	target, err := resolveScorerTarget(&wsproto.VADOption{
		Type:     stringPtr(TypeHybrid),
		Endpoint: stringPtr("/opt/models/silero_vad.onnx"),
	})
	if err != nil {
		t.Fatalf("resolve file path target: %v", err)
	}
	if target.scheme != SchemeNative {
		t.Fatalf("unexpected scheme: %s", target.scheme)
	}
	if target.native.ModelPath != "/opt/models/silero_vad.onnx" {
		t.Fatalf("unexpected model path: %s", target.native.ModelPath)
	}
}

func TestNewScorerReturnsHTTPImplementation(t *testing.T) {
	scorer, err := NewScorer(&wsproto.VADOption{
		Type:     stringPtr(TypeHybrid),
		Endpoint: stringPtr("http://127.0.0.1:8091"),
	})
	if err != nil {
		t.Fatalf("new http scorer: %v", err)
	}
	if scorer == nil {
		t.Fatal("expected http scorer")
	}
	if scorer.Name() != "silero-http" {
		t.Fatalf("unexpected scorer name: %s", scorer.Name())
	}
	if closer, ok := scorer.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("close http scorer: %v", err)
		}
	}
}

func stringPtr(v string) *string { return &v }

func uint32Ptr(v uint32) *uint32 { return &v }
