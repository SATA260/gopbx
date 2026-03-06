// 这个文件定义 WebSocket 事件协议模型，承载应答、指标和错误等回推数据。

package wsproto

type EventEnvelope struct {
	Event     string         `json:"event"`
	TrackID   string         `json:"trackId,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Key       string         `json:"key,omitempty"`
	Duration  uint32         `json:"duration,omitempty"`
	SDP       string         `json:"sdp,omitempty"`
	Text      string         `json:"text,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Index     int            `json:"index,omitempty"`
	StartTime int64          `json:"startTime,omitempty"`
	EndTime   int64          `json:"endTime,omitempty"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
}
