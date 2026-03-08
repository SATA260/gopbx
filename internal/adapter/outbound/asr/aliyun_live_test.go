//go:build livealiyun

// 这个文件用于临时真实联调阿里云 ASR，验证 token + appkey + typed 回包解析是否能跑通。

package asr

import (
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	nls "github.com/aliyun/alibabacloud-nls-go-sdk"

	ttsadapter "gopbx/internal/adapter/outbound/tts"
	"gopbx/pkg/wsproto"
)

type liveCallbackParam struct {
	delta chan string
	final chan string
	err   chan error
}

func TestLiveAliyunASRWithToken(t *testing.T) {
	token := os.Getenv("ALIYUN_TOKEN")
	appKey := os.Getenv("ALIYUN_APPKEY")
	if token == "" || appKey == "" {
		t.Fatal("ALIYUN_TOKEN or ALIYUN_APPKEY is empty")
	}

	pcmCodec := "pcm"
	sampleRate := int32(16000)
	ttsStream, err := ttsadapter.AliyunProvider{}.StartSynthesis("你好，今天天气怎么样。", &wsproto.SynthesisOption{
		AppID:      &appKey,
		Codec:      &pcmCodec,
		Samplerate: &sampleRate,
		Extra: map[string]string{
			"token": token,
		},
	})
	if err != nil {
		t.Fatalf("start aliyun tts: %v", err)
	}
	defer ttsStream.Close()

	config := nls.NewConnectionConfigWithToken(nls.DEFAULT_URL, appKey, token)
	logger := nls.NewNlsLogger(io.Discard, "aliyun-live-asr", log.LstdFlags|log.Lmicroseconds)
	logger.SetLogSil(true)
	logger.SetDebug(false)
	param := &liveCallbackParam{
		delta: make(chan string, 16),
		final: make(chan string, 16),
		err:   make(chan error, 4),
	}
	client, err := nls.NewSpeechTranscription(
		config,
		logger,
		liveOnTaskFailed,
		liveOnStarted,
		liveOnSentenceBegin,
		liveOnSentenceEnd,
		liveOnResultChanged,
		liveOnCompleted,
		liveOnClose,
		param,
	)
	if err != nil {
		t.Fatalf("create aliyun asr client: %v", err)
	}
	defer client.Shutdown()

	startParam := nls.DefaultSpeechTranscriptionParam()
	startParam.Format = "pcm"
	startParam.SampleRate = 16000
	ready, err := client.Start(startParam, nil)
	if err != nil {
		t.Fatalf("start aliyun asr: %v", err)
	}
	if err := waitReady(ready, logger, 20*time.Second); err != nil {
		t.Fatalf("wait aliyun asr start: %v", err)
	}

	chunkCount := 0
	for {
		chunk, recvErr := ttsStream.Recv()
		if recvErr != nil {
			break
		}
		if len(chunk.Data) == 0 {
			continue
		}
		chunkCount++
		if err := client.SendAudioData(chunk.Data); err != nil {
			t.Fatalf("send audio chunk: %v", err)
		}
		time.Sleep(15 * time.Millisecond)
	}
	if chunkCount == 0 {
		t.Fatal("expected synthesized pcm chunks for live asr test")
	}

	ready, err = client.Stop()
	if err != nil {
		t.Fatalf("stop aliyun asr: %v", err)
	}
	if err := waitReady(ready, logger, 20*time.Second); err != nil {
		t.Fatalf("wait aliyun asr stop: %v", err)
	}

	var deltaRaw, finalRaw string
	collectTimeout := time.After(10 * time.Second)
	for finalRaw == "" {
		select {
		case raw := <-param.delta:
			if deltaRaw == "" {
				deltaRaw = raw
			}
		case raw := <-param.final:
			finalRaw = raw
		case err := <-param.err:
			t.Fatalf("live aliyun callback error: %v", err)
		case <-collectTimeout:
			t.Fatalf("timed out waiting for live aliyun final result, delta=%q final=%q", deltaRaw, finalRaw)
		}
	}

	t.Logf("aliyun delta callback: %s", deltaRaw)
	t.Logf("aliyun final callback: %s", finalRaw)
	if deltaRaw != "" {
		result, ok := parseAliyunResult(deltaRaw, false)
		if !ok {
			t.Fatalf("failed to parse live delta callback: %s", deltaRaw)
		}
		if result.Text == "" {
			t.Fatalf("expected non-empty live delta text: %+v", result)
		}
	}
	result, ok := parseAliyunResult(finalRaw, true)
	if !ok {
		t.Fatalf("failed to parse live final callback: %s", finalRaw)
	}
	if result.Text == "" || !result.Final {
		t.Fatalf("unexpected live final result: %+v", result)
	}
}

func liveOnTaskFailed(text string, param interface{}) {
	p, ok := param.(*liveCallbackParam)
	if !ok {
		return
	}
	p.err <- errors.New(text)
}

func liveOnStarted(string, interface{}) {}

func liveOnSentenceBegin(string, interface{}) {}

func liveOnSentenceEnd(text string, param interface{}) {
	p, ok := param.(*liveCallbackParam)
	if !ok {
		return
	}
	p.final <- text
}

func liveOnResultChanged(text string, param interface{}) {
	p, ok := param.(*liveCallbackParam)
	if !ok {
		return
	}
	p.delta <- text
}

func liveOnCompleted(string, interface{}) {}

func liveOnClose(interface{}) {}
