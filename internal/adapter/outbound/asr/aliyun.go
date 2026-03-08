// 这个文件实现阿里云 ASR 适配器，负责把实时语音识别 SDK 封装成会话型 provider。

package asr

import (
	"encoding/json"
	"errors"
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

// aliyunRecognitionResponse 按阿里云实时识别回包结构定义，避免继续依赖宽松 map 解析。
// 这样后面如果字段缺失或协议变更，能够更早暴露问题，而不是静默吞掉不兼容字段。
type aliyunRecognitionResponse struct {
	Header  aliyunRecognitionHeader   `json:"header"`
	Payload *aliyunRecognitionPayload `json:"payload,omitempty"`
}

type aliyunRecognitionHeader struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type aliyunRecognitionPayload struct {
	Result        string `json:"result"`
	SentenceID    uint32 `json:"sentence_id"`
	SentenceEnd   bool   `json:"is_sentence_end"`
	BeginTime     int64  `json:"begin_time"`
	EndTime       int64  `json:"end_time"`
	Confidence    int64  `json:"confidence,omitempty"`
	Words         any    `json:"words,omitempty"`
	SentenceBegin *int64 `json:"sentence_begin_time,omitempty"`
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

	var response aliyunRecognitionResponse
	if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
		return Result{}, false
	}
	if !strings.EqualFold(response.Header.Status, "success") {
		return Result{}, false
	}
	if response.Payload == nil {
		return Result{}, false
	}
	text := strings.TrimSpace(response.Payload.Result)
	if text == "" {
		return Result{}, false
	}
	final = final || response.Payload.SentenceEnd
	startTime := response.Payload.BeginTime
	if response.Payload.SentenceBegin != nil {
		startTime = *response.Payload.SentenceBegin
	}
	return Result{
		Final:     final,
		Text:      text,
		StartTime: startTime,
		EndTime:   response.Payload.EndTime,
	}, true
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
