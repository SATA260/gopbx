// 这个文件负责把内部业务事件编码成对外可发送的 JSON 文本。

package wsinbound

import (
	"encoding/json"

	"gopbx/pkg/wsproto"
)

func MarshalEvent(evt wsproto.EventEnvelope) ([]byte, error) {
	return json.Marshal(evt)
}
