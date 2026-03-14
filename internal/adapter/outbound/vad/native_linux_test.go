//go:build silero_native && cgo && linux

package vad

import (
	"context"
	"math"
	"os"
	"testing"
)

func TestNativeScorerSmoke(t *testing.T) {
	modelPath := os.Getenv(NativeModelPathEnv)
	if modelPath == "" {
		t.Skip("native model path not set")
	}
	scorer, err := newNativeScorer(nativeConfig{ModelPath: modelPath})
	if err != nil {
		t.Fatalf("create native scorer: %v", err)
	}
	defer func() {
		if err := scorer.(interface{ Close() error }).Close(); err != nil {
			t.Fatalf("close native scorer: %v", err)
		}
	}()

	ctx := context.Background()
	silenceProb, err := scorer.Score(ctx, make([]byte, 640*3), 16000)
	if err != nil {
		t.Fatalf("score silence: %v", err)
	}
	if silenceProb < 0 || silenceProb > 1 || math.IsNaN(float64(silenceProb)) {
		t.Fatalf("unexpected silence probability: %v", silenceProb)
	}

	toneProb, err := scorer.Score(ctx, makeTonePCM16LE(16000, 220, 300), 16000)
	if err != nil {
		t.Fatalf("score tone: %v", err)
	}
	if toneProb < 0 || toneProb > 1 || math.IsNaN(float64(toneProb)) {
		t.Fatalf("unexpected tone probability: %v", toneProb)
	}
}

func makeTonePCM16LE(sampleRate, freqHz, durationMs int) []byte {
	sampleCount := sampleRate * durationMs / 1000
	payload := make([]byte, sampleCount*2)
	for i := 0; i < sampleCount; i++ {
		value := int16(18000 * math.Sin(2*math.Pi*float64(freqHz)*float64(i)/float64(sampleRate)))
		payload[i*2] = byte(value)
		payload[i*2+1] = byte(uint16(value) >> 8)
	}
	return payload
}
