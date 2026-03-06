// 这个文件定义事件名兼容常量，统一对外回推时使用的事件标识。

package compat

const (
	EventIncoming     = "incoming"
	EventAnswer       = "answer"
	EventReject       = "reject"
	EventRinging      = "ringing"
	EventHangup       = "hangup"
	EventSpeaking     = "speaking"
	EventSilence      = "silence"
	EventEOU          = "eou"
	EventDTMF         = "dtmf"
	EventTrackStart   = "trackStart"
	EventTrackEnd     = "trackEnd"
	EventInterruption = "interruption"
	EventASRFinal     = "asrFinal"
	EventASRDelta     = "asrDelta"
	EventMetrics      = "metrics"
	EventError        = "error"
	EventAddHistory   = "addHistory"
	EventOther        = "other"
	EventBinary       = "binary"
)
