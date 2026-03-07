// 这个文件定义编解码类型和统一工厂，负责把兼容配置中的 codec 名称映射成可用的媒体编解码器。

package codec

import "strings"

type Type string

const (
	PCMU Type = "PCMU"
	PCMA Type = "PCMA"
	G722 Type = "G722"
)

type Codec interface {
	Type() Type
	SampleRate() int
	Encode([]byte) []byte
	Decode([]byte) []byte
}

func Parse(name string) Type {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "PCMA":
		return PCMA
	case "G722":
		return G722
	default:
		return PCMU
	}
}

func New(name string) Codec {
	switch Parse(name) {
	case PCMA:
		return PCMACodec{}
	case G722:
		return G722Codec{}
	default:
		return PCMUCodec{}
	}
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
