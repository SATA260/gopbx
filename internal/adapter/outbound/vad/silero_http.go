// 这个文件实现基于本机 HTTP sidecar 的 Silero 打分器。

package vad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPScorer struct {
	baseURL string
	client  *http.Client
}

func NewHTTPScorer(endpoint string) (*HTTPScorer, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("vad endpoint is empty")
	}
	return &HTTPScorer{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 500 * time.Millisecond},
	}, nil
}

func (s *HTTPScorer) Name() string { return "silero-http" }

func (s *HTTPScorer) Close() error { return nil }

func (s *HTTPScorer) Score(ctx context.Context, pcm16le []byte, sampleRate int) (float32, error) {
	if s == nil {
		return 0, fmt.Errorf("vad scorer is nil")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/score?sampleRate="+strconv.Itoa(sampleRate), bytes.NewReader(pcm16le))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 0, fmt.Errorf("vad scorer status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		SpeechProb float32 `json:"speechProb"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload.SpeechProb, nil
}
