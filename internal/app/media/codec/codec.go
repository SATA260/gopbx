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
	default:
		return PCMU
	}
}

func New(name string) Codec {
	switch Parse(name) {
	case PCMA:
		return PCMACodec{}
	default:
		return PCMUCodec{}
	}
}

func FromWebRTCMime(mime string) (Type, bool) {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "audio/pcmu":
		return PCMU, true
	case "audio/pcma":
		return PCMA, true
	default:
		return "", false
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

// ResamplePCM16LE 用最简单的最近邻方式把 PCM16 little-endian 音频重采样到目标采样率。
// 当前主要用于把 8k G.711 入站音频统一成 16k PCM，供实时识别后端稳定消费。
func ResamplePCM16LE(payload []byte, srcRate, dstRate int) []byte {
	if len(payload) == 0 || srcRate <= 0 || dstRate <= 0 || srcRate == dstRate {
		return cloneBytes(payload)
	}
	input := pcmBytesToSamples(payload)
	if len(input) == 0 {
		return nil
	}
	outLen := len(input) * dstRate / srcRate
	if outLen <= 0 {
		outLen = len(input)
	}
	output := make([]int16, outLen)
	for i := 0; i < outLen; i++ {
		srcIndex := i * srcRate / dstRate
		if srcIndex >= len(input) {
			srcIndex = len(input) - 1
		}
		output[i] = input[srcIndex]
	}
	return pcmSamplesToBytes(output)
}
