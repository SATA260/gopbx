// 这个文件定义编解码类型和统一工厂，负责把兼容配置中的 codec 名称映射成可用的媒体编解码器。

package codec

import (
	"encoding/binary"
	"strings"
)

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

// pcmBytesToSamples 把 little-endian PCM 字节流切成 int16 样本。
// 如果字节数为奇数，尾部残字节会被忽略，避免编码时读出越界样本。
func pcmBytesToSamples(payload []byte) []int16 {
	if len(payload) < 2 {
		return nil
	}
	count := len(payload) / 2
	samples := make([]int16, count)
	for i := 0; i < count; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(payload[i*2:]))
	}
	return samples
}

func pcmSamplesToBytes(samples []int16) []byte {
	if len(samples) == 0 {
		return nil
	}
	payload := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(payload[i*2:], uint16(sample))
	}
	return payload
}
