// 这个文件定义外部 VAD 打分器抽象，并封装 HTTP/native 两种 Silero 接法的选择逻辑。

package vad

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopbx/pkg/wsproto"
)

const (
	TypePassthrough = "passthrough"
	TypeEnergy      = "energy"
	TypeHybrid      = "hybrid"

	SchemeHTTP   = "http"
	SchemeHTTPS  = "https"
	SchemeCGO    = "cgo"
	SchemeNative = "native"

	NativeModelPathEnv = "GOPBX_VAD_NATIVE_MODEL_PATH"
)

type Scorer interface {
	Name() string
	Score(ctx context.Context, pcm16le []byte, sampleRate int) (float32, error)
}

type nativeConfig struct {
	ModelPath string
}

func NormalizeType(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func IsSegmented(option *wsproto.VADOption) bool {
	switch NormalizeType(derefString(option, func(v *wsproto.VADOption) *string { return v.Type })) {
	case TypeEnergy, TypeHybrid:
		return true
	default:
		return false
	}
}

func ValidateOption(option *wsproto.VADOption) error {
	if option == nil {
		return nil
	}
	switch NormalizeType(derefString(option, func(v *wsproto.VADOption) *string { return v.Type })) {
	case "", TypePassthrough, TypeEnergy, TypeHybrid:
	default:
		return fmt.Errorf("unsupported vad type: %s", derefString(option, func(v *wsproto.VADOption) *string { return v.Type }))
	}
	if option.Samplerate != nil && *option.Samplerate != 8000 && *option.Samplerate != 16000 {
		return fmt.Errorf("unsupported vad samplerate: %d", *option.Samplerate)
	}
	if option.Ratio != nil && *option.Ratio <= 0 {
		return fmt.Errorf("vad ratio must be greater than 0")
	}
	if option.VoiceThreshold != nil && (*option.VoiceThreshold < 0 || *option.VoiceThreshold > 1) {
		return fmt.Errorf("vad voiceThreshold must be between 0 and 1")
	}
	if _, err := resolveScorerTarget(option); err != nil {
		return err
	}
	return nil
}

func NewScorer(option *wsproto.VADOption) (Scorer, error) {
	if NormalizeType(derefString(option, func(v *wsproto.VADOption) *string { return v.Type })) != TypeHybrid {
		return nil, nil
	}
	target, err := resolveScorerTarget(option)
	if err != nil {
		return nil, err
	}
	switch target.scheme {
	case "", SchemeCGO, SchemeNative:
		if target.native.ModelPath == "" {
			return nil, nil
		}
		return newNativeScorer(target.native)
	case SchemeHTTP, SchemeHTTPS:
		return NewHTTPScorer(target.endpoint)
	default:
		return nil, fmt.Errorf("unsupported vad endpoint scheme: %s", target.scheme)
	}
}

type scorerTarget struct {
	scheme   string
	endpoint string
	native   nativeConfig
}

func resolveScorerTarget(option *wsproto.VADOption) (scorerTarget, error) {
	raw := strings.TrimSpace(derefString(option, func(v *wsproto.VADOption) *string { return v.Endpoint }))
	if raw == "" {
		return scorerTarget{scheme: SchemeNative, native: nativeConfig{ModelPath: strings.TrimSpace(os.Getenv(NativeModelPathEnv))}}, nil
	}
	if looksLikeFilePath(raw) {
		return scorerTarget{scheme: SchemeNative, native: nativeConfig{ModelPath: raw}}, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return scorerTarget{}, fmt.Errorf("invalid vad endpoint: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	switch scheme {
	case SchemeHTTP, SchemeHTTPS:
		return scorerTarget{scheme: scheme, endpoint: raw}, nil
	case SchemeCGO, SchemeNative:
		modelPath := decodeNativeModelPath(parsed)
		if modelPath == "" {
			modelPath = strings.TrimSpace(os.Getenv(NativeModelPathEnv))
		}
		return scorerTarget{scheme: SchemeNative, native: nativeConfig{ModelPath: modelPath}}, nil
	default:
		return scorerTarget{}, fmt.Errorf("unsupported vad endpoint scheme: %s", scheme)
	}
}

func decodeNativeModelPath(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	path := parsed.Path
	if parsed.Host != "" {
		if path == "" || path == "/" {
			path = parsed.Host
		} else {
			path = "/" + strings.TrimPrefix(filepath.Join(parsed.Host, path), "/")
		}
	}
	if path == "/" {
		return ""
	}
	return path
}

func looksLikeFilePath(v string) bool {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") {
		return true
	}
	if strings.Contains(trimmed, "\\") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(trimmed), ".onnx")
}

func derefString(option *wsproto.VADOption, pick func(*wsproto.VADOption) *string) string {
	if option == nil {
		return ""
	}
	v := pick(option)
	if v == nil {
		return ""
	}
	return *v
}
