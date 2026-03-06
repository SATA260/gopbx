// 这个文件定义话单模型，占位描述通话归档需要保存的核心字段。

package callrecord

import "time"

type Record struct {
	CallID    string    `json:"callId"`
	CreatedAt time.Time `json:"createdAt"`
}
