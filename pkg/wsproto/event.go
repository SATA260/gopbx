// 这个文件定义 WebSocket 事件协议模型，按既有协议的事件返回结构保持兼容。

package wsproto

import "time"

type EventEnvelope struct {
	Event      string            `json:"event"`
	TrackID    string            `json:"trackId,omitempty"`
	Timestamp  int64             `json:"timestamp,omitempty"`
	Caller     string            `json:"caller,omitempty"`
	Callee     string            `json:"callee,omitempty"`
	SDP        string            `json:"sdp,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Code       *uint32           `json:"code,omitempty"`
	EarlyMedia *bool             `json:"earlyMedia,omitempty"`
	StartTime  *int64            `json:"startTime,omitempty"`
	EndTime    *int64            `json:"endTime,omitempty"`
	Completed  *bool             `json:"completed,omitempty"`
	Digit      string            `json:"digit,omitempty"`
	Duration   *uint64           `json:"duration,omitempty"`
	Position   *uint64           `json:"position,omitempty"`
	Text       string            `json:"text,omitempty"`
	Index      *uint32           `json:"index,omitempty"`
	Key        string            `json:"key,omitempty"`
	Data       any               `json:"data,omitempty"`
	Sender     string            `json:"sender,omitempty"`
	Error      string            `json:"error,omitempty"`
	Speaker    string            `json:"speaker,omitempty"`
	Initiator  string            `json:"initiator,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

func Bool(v bool) *bool {
	return &v
}

func Int64(v int64) *int64 {
	return &v
}

func Uint32(v uint32) *uint32 {
	return &v
}

func Uint64(v uint64) *uint64 {
	return &v
}

func NewErrorEvent(trackID, sender, message string) EventEnvelope {
	return EventEnvelope{
		Event:     "error",
		TrackID:   trackID,
		Timestamp: NowMillis(),
		Sender:    sender,
		Error:     message,
	}
}

func NowMillis() int64 {
	return time.Now().UnixMilli()
}
