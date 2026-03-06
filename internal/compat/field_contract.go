// 这个文件集中维护协议字段名常量，用来保证 JSON 和 WS 报文字段不漂移。

package compat

const (
	FieldCommand    = "command"
	FieldEvent      = "event"
	FieldTrackID    = "trackId"
	FieldPlayID     = "playId"
	FieldTimestamp  = "timestamp"
	FieldAutoHangup = "autoHangup"
)
