// 这个文件实现阿里云 ASR 适配器，负责把实时语音识别 SDK 封装成会话型 provider。

package asr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	nls "github.com/aliyun/alibabacloud-nls-go-sdk"

	"gopbx/pkg/wsproto"
)

type AliyunProvider struct{}

func (AliyunProvider) Name() string { return ProviderAliyun }

// NewSession 会优先尝试构建真实阿里云实时识别会话；
// 如果当前配置还不完整，就退回到 aliyun 命名的 mock session，避免把现有测试链路直接打断。
func (AliyunProvider) NewSession(cfg *wsproto.ASRConfig) (Session, error) {
	config, startParam, extra, err := newAliyunRecognitionConfig(cfg)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return &mockSession{provider: ProviderAliyun}, nil
	}

	logger := nls.NewNlsLogger(io.Discard, "gopbx-aliyun-asr", log.LstdFlags|log.Lmicroseconds)
	logger.SetLogSil(true)
	logger.SetDebug(false)

	session := &aliyunSession{
		logger:     logger,
		startParam: startParam,
		extra:      extra,
	}
	client, err := nls.NewSpeechTranscription(
		config,
		logger,
		aliyunOnTaskFailed,
		aliyunOnStarted,
		aliyunOnSentenceBegin,
		aliyunOnSentenceEnd,
		aliyunOnResultChanged,
		aliyunOnCompleted,
		aliyunOnClose,
		session,
	)
	if err != nil {
		return nil, err
	}
	session.client = client
	return session, nil
}

type aliyunSession struct {
	mu         sync.Mutex
	client     *nls.SpeechTranscription
	logger     *nls.NlsLogger
	startParam nls.SpeechTranscriptionStartParam
	extra      map[string]interface{}
	results    []Result
	started    bool
	closed     bool
	lastErr    error
}

// WriteAudio 会在第一次音频进入时启动实时识别，然后持续发送音频并把已收到的回调结果拉平给上层处理器。
func (s *aliyunSession) WriteAudio(payload []byte) ([]Result, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	if err := s.ensureStarted(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, nil
	}
	client := s.client
	s.mu.Unlock()

	if err := client.SendAudioData(payload); err != nil {
		return nil, err
	}
	return s.drainResults(), s.takeError()
}

// Close 会停止阿里云实时识别会话并回收底层 websocket 连接。
func (s *aliyunSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	started := s.started
	client := s.client
	logger := s.logger
	s.mu.Unlock()

	if client == nil {
		return nil
	}

	var firstErr error
	if started {
		ready, err := client.Stop()
		if err != nil {
			firstErr = err
		} else if err := waitReady(ready, logger, 20*time.Second); err != nil {
			firstErr = err
		}
	}
	client.Shutdown()
	if err := s.takeError(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *aliyunSession) ensureStarted() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("aliyun asr session is closed")
	}
	if s.started {
		s.mu.Unlock()
		return nil
	}
	client := s.client
	startParam := s.startParam
	extra := s.extra
	logger := s.logger
	s.mu.Unlock()

	ready, err := client.Start(startParam, extra)
	if err != nil {
		return err
	}
	if err := waitReady(ready, logger, 20*time.Second); err != nil {
		return err
	}

	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	return nil
}

func (s *aliyunSession) appendResult(result Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, result)
}

func (s *aliyunSession) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastErr == nil {
		s.lastErr = err
	}
}

func (s *aliyunSession) drainResults() []Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.results) == 0 {
		return nil
	}
	out := make([]Result, len(s.results))
	copy(out, s.results)
	s.results = nil
	return out
}

func (s *aliyunSession) takeError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.lastErr
	s.lastErr = nil
	return err
}

func aliyunOnTaskFailed(text string, param interface{}) {
	session, ok := param.(*aliyunSession)
	if !ok {
		return
	}
	session.fail(errors.New(strings.TrimSpace(text)))
}

func aliyunOnStarted(string, interface{}) {}

func aliyunOnSentenceBegin(string, interface{}) {}

func aliyunOnSentenceEnd(text string, param interface{}) {
	session, ok := param.(*aliyunSession)
	if !ok {
		return
	}
	if result, ok := parseAliyunResult(text, true); ok {
		session.appendResult(result)
	}
}

func aliyunOnResultChanged(text string, param interface{}) {
	session, ok := param.(*aliyunSession)
	if !ok {
		return
	}
	if result, ok := parseAliyunResult(text, false); ok {
		session.appendResult(result)
	}
}

func aliyunOnCompleted(string, interface{}) {}

func aliyunOnClose(interface{}) {}

func newAliyunRecognitionConfig(cfg *wsproto.ASRConfig) (*nls.ConnectionConfig, nls.SpeechTranscriptionStartParam, map[string]interface{}, error) {
	param := nls.DefaultSpeechTranscriptionParam()
	extra := make(map[string]interface{})
	if cfg == nil {
		return nil, param, nil, nil
	}

	for key, value := range cfg.Extra {
		extra[key] = value
	}
	if cfg.Samplerate != nil && *cfg.Samplerate > 0 {
		param.SampleRate = int(*cfg.Samplerate)
	}
	if format, ok := extra["format"].(string); ok && strings.TrimSpace(format) != "" {
		param.Format = format
	}
	if cfg.Language != nil && *cfg.Language != "" {
		extra["language"] = *cfg.Language
	}
	if cfg.Model != nil && *cfg.Model != "" {
		extra["model"] = *cfg.Model
	}
	if cfg.ModelType != nil && *cfg.ModelType != "" {
		extra["model_type"] = *cfg.ModelType
	}

	endpoint := nls.DEFAULT_URL
	if cfg.Endpoint != nil && strings.TrimSpace(*cfg.Endpoint) != "" {
		endpoint = strings.TrimSpace(*cfg.Endpoint)
	}
	appKey := resolveAliyunAppKey(cfg)
	if appKey == "" {
		return nil, param, extra, nil
	}
	if token, ok := extra["token"].(string); ok && strings.TrimSpace(token) != "" {
		return nls.NewConnectionConfigWithToken(endpoint, appKey, strings.TrimSpace(token)), param, extra, nil
	}
	akid := derefASRString(cfg.SecretID)
	akkey := derefASRString(cfg.SecretKey)
	if akid == "" || akkey == "" {
		return nil, param, extra, nil
	}
	config, err := nls.NewConnectionConfigWithAKInfoDefault(endpoint, appKey, akid, akkey)
	return config, param, extra, err
}

func resolveAliyunAppKey(cfg *wsproto.ASRConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.Extra != nil {
		if appKey := strings.TrimSpace(cfg.Extra["appKey"]); appKey != "" {
			return appKey
		}
		if appKey := strings.TrimSpace(cfg.Extra["app_key"]); appKey != "" {
			return appKey
		}
	}
	if cfg.AppID != nil {
		return strings.TrimSpace(*cfg.AppID)
	}
	return ""
}

func parseAliyunResult(raw string, final bool) (Result, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Result{}, false
	}

	var response nls.CommonResponse
	if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
		return Result{Final: final, Text: trimmed}, true
	}
	text := extractAliyunText(response.Payload)
	if text == "" {
		return Result{}, false
	}
	return Result{
		Final:     final,
		Text:      text,
		StartTime: extractAliyunInt(response.Payload, "begin_time", "start_time", "beginTime", "startTime"),
		EndTime:   extractAliyunInt(response.Payload, "end_time", "endTime"),
	}, true
}

func extractAliyunText(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	for _, key := range []string{"result", "text", "sentence", "transcript"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractAliyunInt(payload map[string]interface{}, keys ...string) int64 {
	if payload == nil {
		return 0
	}
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed)
		case int64:
			return typed
		case int:
			return int64(typed)
		case string:
			var parsed int64
			if _, err := fmt.Sscan(strings.TrimSpace(typed), &parsed); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func waitReady(ch chan bool, logger *nls.NlsLogger, timeout time.Duration) error {
	select {
	case done := <-ch:
		if !done {
			if logger != nil {
				logger.Println("Wait failed")
			}
			return errors.New("wait failed")
		}
		return nil
	case <-time.After(timeout):
		if logger != nil {
			logger.Println("Wait timeout")
		}
		return errors.New("wait timeout")
	}
}

func derefASRString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}
