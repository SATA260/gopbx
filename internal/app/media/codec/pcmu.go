// 这个文件实现 PCMU 编解码器，负责在线性 PCM16 与 G.711 u-law 之间做真实转换。

package codec

type PCMUCodec struct{}

func (PCMUCodec) Type() Type { return PCMU }

func (PCMUCodec) SampleRate() int { return 8000 }

// Encode 会把 16bit little-endian PCM 压缩成单字节 u-law 数据。
func (PCMUCodec) Encode(payload []byte) []byte {
	samples := pcmBytesToSamples(payload)
	if len(samples) == 0 {
		return nil
	}
	out := make([]byte, len(samples))
	for i, sample := range samples {
		out[i] = linearToMuLaw(sample)
	}
	return out
}

// Decode 会把单字节 u-law 数据还原成 16bit little-endian PCM。
func (PCMUCodec) Decode(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	samples := make([]int16, len(payload))
	for i, value := range payload {
		samples[i] = muLawToLinear(value)
	}
	return pcmSamplesToBytes(samples)
}

func linearToMuLaw(sample int16) byte {
	const (
		bias = 0x84
		clip = 32635
	)
	pcm := int(sample)
	sign := 0
	if pcm < 0 {
		sign = 0x80
		pcm = -pcm
		if pcm < 0 {
			pcm = clip
		}
	}
	if pcm > clip {
		pcm = clip
	}
	pcm += bias

	exponent := 7
	for expMask := 0x4000; exponent > 0 && (pcm&expMask) == 0; exponent-- {
		expMask >>= 1
	}
	mantissa := (pcm >> (exponent + 3)) & 0x0f
	return byte(^(sign | (exponent << 4) | mantissa))
}

func muLawToLinear(value byte) int16 {
	const bias = 0x84
	value = ^value
	sign := value & 0x80
	exponent := (value >> 4) & 0x07
	mantissa := value & 0x0f
	pcm := ((int(mantissa) << 3) + bias) << exponent
	pcm -= bias
	if sign != 0 {
		pcm = -pcm
	}
	return int16(pcm)
}
