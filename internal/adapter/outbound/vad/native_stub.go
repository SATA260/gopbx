//go:build !silero_native || !cgo || !linux

package vad

import "fmt"

func newNativeScorer(cfg nativeConfig) (Scorer, error) {
	if cfg.ModelPath == "" {
		return nil, nil
	}
	return nil, fmt.Errorf("native Silero scorer requires linux+cgo and build tag silero_native")
}
