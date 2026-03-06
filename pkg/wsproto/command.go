// 这个文件定义 WebSocket 命令协议模型，承载建会、播报和挂断等请求数据。

package wsproto

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
	Command     CommandName    `json:"command"`
	Option      *CallOption    `json:"option,omitempty"`
	Text        string         `json:"text,omitempty"`
	PlayID      string         `json:"playId,omitempty"`
	AutoHangup  bool           `json:"autoHangup,omitempty"`
	Streaming   bool           `json:"streaming,omitempty"`
	EndOfStream bool           `json:"endOfStream,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Initiator   string         `json:"initiator,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

type CallOption struct {
	Offer   string     `json:"offer,omitempty"`
	Caller  string     `json:"caller,omitempty"`
	Callee  string     `json:"callee,omitempty"`
	Denoise bool       `json:"denoise,omitempty"`
	ASR     *ASRConfig `json:"asr,omitempty"`
	TTS     *TTSConfig `json:"tts,omitempty"`
	Extra   any        `json:"extra,omitempty"`
}

type ASRConfig struct {
	Provider  string `json:"provider,omitempty" yaml:"provider,omitempty"`
	AppID     string `json:"appId,omitempty" yaml:"appId,omitempty"`
	SecretID  string `json:"secretId,omitempty" yaml:"secretId,omitempty"`
	SecretKey string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
	Language  string `json:"language,omitempty" yaml:"language,omitempty"`
}

type TTSConfig struct {
	Provider  string  `json:"provider,omitempty" yaml:"provider,omitempty"`
	Speaker   string  `json:"speaker,omitempty" yaml:"speaker,omitempty"`
	AppID     string  `json:"appId,omitempty" yaml:"appId,omitempty"`
	SecretID  string  `json:"secretId,omitempty" yaml:"secretId,omitempty"`
	SecretKey string  `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
	Speed     float64 `json:"speed,omitempty" yaml:"speed,omitempty"`
	Volume    int     `json:"volume,omitempty" yaml:"volume,omitempty"`
}

type ICEServer struct {
	URLs       []string `json:"urls" yaml:"urls"`
	Username   string   `json:"username,omitempty" yaml:"username,omitempty"`
	Credential string   `json:"credential,omitempty" yaml:"credential,omitempty"`
	Password   string   `json:"password,omitempty" yaml:"password,omitempty"`
}
