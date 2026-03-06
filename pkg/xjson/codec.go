// 这个文件提供 JSON 编解码工具，负责严格解析外部请求并输出标准报文。

package xjson

import (
	"bytes"
	"encoding/json"
)

func DecodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}
