// 这个文件定义 WebSocket 命令协议模型，承载建会、播报和挂断等请求数据。

package wsproto

import "encoding/json"

type CommandName string

const (
	CommandInvite    CommandName = "invite"
	CommandAccept    CommandName = "accept"
	CommandReject    CommandName = "reject"
	CommandCandidate CommandName = "candidate"
	CommandTTS       CommandName = "tts"
	CommandPlay      CommandName = "play"
	CommandInterrupt CommandName = "interrupt"
	CommandPause     CommandName = "pause"
	CommandResume    CommandName = "resume"
	CommandHangup    CommandName = "hangup"
	CommandRefer     CommandName = "refer"
	CommandMute      CommandName = "mute"
	CommandUnmute    CommandName = "unmute"
	CommandHistory   CommandName = "history"
)

type CommandEnvelope struct {
	Command     CommandName     `json:"command"`
	Option      json.RawMessage `json:"option,omitempty"`
	Reason      *string         `json:"reason,omitempty"`
	Code        *uint32         `json:"code,omitempty"`
	Candidates  []string        `json:"candidates,omitempty"`
	Text        string          `json:"text,omitempty"`
	Speaker     *string         `json:"speaker,omitempty"`
	PlayID      *string         `json:"playId,omitempty"`
	AutoHangup  *bool           `json:"autoHangup,omitempty"`
	Streaming   *bool           `json:"streaming,omitempty"`
	EndOfStream *bool           `json:"endOfStream,omitempty"`
	URL         *string         `json:"url,omitempty"`
	Initiator   *string         `json:"initiator,omitempty"`
	Target      *string         `json:"target,omitempty"`
	Options     *ReferOption    `json:"options,omitempty"`
	TrackID     *string         `json:"trackId,omitempty"`
}

func (c *CommandEnvelope) CallOption() (*CallOption, error) {
	if len(c.Option) == 0 {
		return nil, nil
	}
	var option CallOption
	if err := json.Unmarshal(c.Option, &option); err != nil {
		return nil, err
	}
	return &option, nil
}

func (c *CommandEnvelope) TTSOption() (*SynthesisOption, error) {
	if len(c.Option) == 0 {
		return nil, nil
	}
	var option SynthesisOption
	if err := json.Unmarshal(c.Option, &option); err != nil {
		return nil, err
	}
	return &option, nil
}

type CallOption struct {
	Denoise          *bool             `json:"denoise,omitempty"`
	Offer            *string           `json:"offer,omitempty"`
	Callee           *string           `json:"callee,omitempty"`
	Caller           *string           `json:"caller,omitempty"`
	Recorder         *RecorderOption   `json:"recorder,omitempty"`
	VAD              *VADOption        `json:"vad,omitempty"`
	ASR              *ASRConfig        `json:"asr,omitempty"`
	TTS              *SynthesisOption  `json:"tts,omitempty"`
	HandshakeTimeout *string           `json:"handshakeTimeout,omitempty"`
	EnableIPv6       *bool             `json:"enableIpv6,omitempty"`
	Extra            map[string]string `json:"extra,omitempty"`
	Codec            *string           `json:"codec,omitempty"`
	EOU              *EOUOption        `json:"eou,omitempty"`
}

type RecorderOption struct {
	RecorderFile string  `json:"recorderFile,omitempty"`
	Samplerate   *uint32 `json:"samplerate,omitempty"`
	Ptime        any     `json:"ptime,omitempty"`
}

type VADOption struct {
	Type                  *string  `json:"type,omitempty"`
	Samplerate            *uint32  `json:"samplerate,omitempty"`
	SpeechPadding         *uint64  `json:"speechPadding,omitempty"`
	SilencePadding        *uint64  `json:"silencePadding,omitempty"`
	Ratio                 *float32 `json:"ratio,omitempty"`
	VoiceThreshold        *float32 `json:"voiceThreshold,omitempty"`
	MaxBufferDurationSecs *uint64  `json:"maxBufferDurationSecs,omitempty"`
	Endpoint              *string  `json:"endpoint,omitempty"`
	SecretKey             *string  `json:"secretKey,omitempty"`
	SecretID              *string  `json:"secretId,omitempty"`
}

type ASRConfig struct {
	Provider   *string           `json:"provider,omitempty"`
	Model      *string           `json:"model,omitempty"`
	Language   *string           `json:"language,omitempty"`
	AppID      *string           `json:"appId,omitempty"`
	SecretID   *string           `json:"secretId,omitempty"`
	SecretKey  *string           `json:"secretKey,omitempty"`
	ModelType  *string           `json:"modelType,omitempty"`
	BufferSize *int              `json:"bufferSize,omitempty"`
	Samplerate *uint32           `json:"samplerate,omitempty"`
	Endpoint   *string           `json:"endpoint,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

type SynthesisOption struct {
	Samplerate *int32            `json:"samplerate,omitempty"`
	Provider   *string           `json:"provider,omitempty"`
	Speed      *float32          `json:"speed,omitempty"`
	AppID      *string           `json:"appId,omitempty"`
	SecretID   *string           `json:"secretId,omitempty"`
	SecretKey  *string           `json:"secretKey,omitempty"`
	Volume     *int32            `json:"volume,omitempty"`
	Speaker    *string           `json:"speaker,omitempty"`
	Codec      *string           `json:"codec,omitempty"`
	Subtitle   *bool             `json:"subtitle,omitempty"`
	Emotion    *string           `json:"emotion,omitempty"`
	Endpoint   *string           `json:"endpoint,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

type ReferOption struct {
	Bypass     *bool   `json:"bypass,omitempty"`
	Timeout    *uint32 `json:"timeout,omitempty"`
	MOH        *string `json:"moh,omitempty"`
	AutoHangup *bool   `json:"autoHangup,omitempty"`
}

type EOUOption struct {
	Type      *string `json:"type,omitempty"`
	Endpoint  *string `json:"endpoint,omitempty"`
	SecretKey *string `json:"secretKey,omitempty"`
	SecretID  *string `json:"secretId,omitempty"`
	Timeout   *uint32 `json:"timeout,omitempty"`
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   *string  `json:"username,omitempty"`
	Credential *string  `json:"credential,omitempty"`
	Password   *string  `json:"password,omitempty"`
}
