// 这个文件实现阿里云 TTS 适配器，负责把语音合成 SDK 封装成可逐块读取的音频流。

package tts

import (
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

func (AliyunProvider) MetricKey(prefix string) string {
	return prefix + ".tts.aliyun"
}

// StartSynthesis 会优先尝试启动真实阿里云 TTS；
// 如果配置还不完整，就退回到 aliyun 命名的 mock 流，避免破坏现有兼容测试链路。
func (AliyunProvider) StartSynthesis(text string, cfg *wsproto.SynthesisOption) (Stream, error) {
	config, startParam, extra, err := newAliyunSynthesisConfig(cfg)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return &mockStream{chunks: []Chunk{{Data: []byte("aliyun-tts:" + text)}}}, nil
	}

	logger := nls.NewNlsLogger(io.Discard, "gopbx-aliyun-tts", log.LstdFlags|log.Lmicroseconds)
	logger.SetLogSil(true)
	logger.SetDebug(false)

	stream := &aliyunStream{
		logger:     logger,
		chunks:     make(chan Chunk, 32),
		done:       make(chan struct{}),
		startParam: startParam,
		extra:      extra,
	}
	client, err := nls.NewSpeechSynthesis(
		config,
		logger,
		false,
		aliyunTTSTaskFailed,
		aliyunTTSSynthesisResult,
		nil,
		aliyunTTSCompleted,
		aliyunTTSClose,
		stream,
	)
	if err != nil {
		return nil, err
	}
	stream.client = client
	ready, err := client.Start(text, startParam, extra)
	if err != nil {
		client.Shutdown()
		return nil, err
	}
	go stream.waitCompletion(ready)
	return stream, nil
}

type aliyunStream struct {
	mu         sync.Mutex
	client     *nls.SpeechSynthesis
	logger     *nls.NlsLogger
	chunks     chan Chunk
	done       chan struct{}
	startParam nls.SpeechSynthesisStartParam
	extra      map[string]interface{}
	closed     bool
	closeOnce  sync.Once
	err        error
}

func (s *aliyunStream) Recv() (Chunk, error) {
	for {
		select {
		case chunk := <-s.chunks:
			return chunk, nil
		case <-s.done:
			select {
			case chunk := <-s.chunks:
				return chunk, nil
			default:
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.err != nil {
					return Chunk{}, s.err
				}
				return Chunk{}, io.EOF
			}
		}
	}
}

func (s *aliyunStream) Close() error {
	var shutdownErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		client := s.client
		s.mu.Unlock()
		if client != nil {
			client.Shutdown()
		}
		close(s.done)
	})
	return shutdownErr
}

func (s *aliyunStream) waitCompletion(ready chan bool) {
	err := waitReady(ready, s.logger, 60*time.Second)
	if err != nil {
		s.setError(err)
	}
	_ = s.Close()
}

func (s *aliyunStream) pushChunk(data []byte) {
	if len(data) == 0 {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	select {
	case s.chunks <- Chunk{Data: append([]byte(nil), data...)}:
	case <-s.done:
	}
}

func (s *aliyunStream) setError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func aliyunTTSTaskFailed(text string, param interface{}) {
	stream, ok := param.(*aliyunStream)
	if !ok {
		return
	}
	stream.setError(errors.New(strings.TrimSpace(text)))
}

func aliyunTTSSynthesisResult(data []byte, param interface{}) {
	stream, ok := param.(*aliyunStream)
	if !ok {
		return
	}
	stream.pushChunk(data)
}

func aliyunTTSCompleted(string, interface{}) {}

func aliyunTTSClose(interface{}) {}

func newAliyunSynthesisConfig(cfg *wsproto.SynthesisOption) (*nls.ConnectionConfig, nls.SpeechSynthesisStartParam, map[string]interface{}, error) {
	param := nls.DefaultSpeechSynthesisParam()
	extra := make(map[string]interface{})
	if cfg == nil {
		return nil, param, nil, nil
	}
	for key, value := range cfg.Extra {
		extra[key] = value
	}
	if cfg.Speaker != nil && strings.TrimSpace(*cfg.Speaker) != "" {
		param.Voice = strings.TrimSpace(*cfg.Speaker)
	}
	if cfg.Codec != nil && strings.TrimSpace(*cfg.Codec) != "" {
		param.Format = strings.TrimSpace(*cfg.Codec)
	}
	if cfg.Samplerate != nil && *cfg.Samplerate > 0 {
		param.SampleRate = int(*cfg.Samplerate)
	}
	if cfg.Volume != nil {
		param.Volume = int(*cfg.Volume)
	}
	if cfg.Speed != nil {
		param.SpeechRate = int(*cfg.Speed)
	}
	if cfg.Subtitle != nil {
		param.EnableSubtitle = *cfg.Subtitle
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
	akid := derefTTSString(cfg.SecretID)
	akkey := derefTTSString(cfg.SecretKey)
	if akid == "" || akkey == "" {
		return nil, param, extra, nil
	}
	config, err := nls.NewConnectionConfigWithAKInfoDefault(endpoint, appKey, akid, akkey)
	return config, param, extra, err
}

func resolveAliyunAppKey(cfg *wsproto.SynthesisOption) string {
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

func derefTTSString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}
