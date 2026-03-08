// 这个文件实现 PCMA 编解码器，负责在线性 PCM16 与 G.711 A-law 之间做真实转换。

package codec

type PCMACodec struct{}

func (PCMACodec) Type() Type { return PCMA }

func (PCMACodec) SampleRate() int { return 8000 }

// Encode 会把 16bit little-endian PCM 压缩成单字节 A-law 数据。
func (PCMACodec) Encode(payload []byte) []byte {
	samples := pcmBytesToSamples(payload)
	if len(samples) == 0 {
		return nil
	}
	out := make([]byte, len(samples))
	for i, sample := range samples {
		out[i] = linearToALaw(sample)
	}
	return out
}

// Decode 会把单字节 A-law 数据还原成 16bit little-endian PCM。
func (PCMACodec) Decode(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	samples := make([]int16, len(payload))
	for i, value := range payload {
		samples[i] = aLawToLinear(value)
	}
	return pcmSamplesToBytes(samples)
}

func linearToALaw(sample int16) byte {
	pcm := int(sample)
	mask := 0xD5
	if pcm < 0 {
		mask = 0x55
		pcm = -pcm - 8
		if pcm < 0 {
			pcm = 0
		}
	}
	if pcm > 0x7fff {
		pcm = 0x7fff
	}

	segment := aLawSegment(pcm)
	var alaw int
	if segment >= 8 {
		alaw = 0x7f
	} else {
		alaw = segment << 4
		if segment < 2 {
			alaw |= (pcm >> 4) & 0x0f
		} else {
			alaw |= (pcm >> (segment + 3)) & 0x0f
		}
	}
	return byte(alaw ^ mask)
}

func aLawToLinear(value byte) int16 {
	value ^= 0x55
	t := int(value&0x0f) << 4
	segment := int((value & 0x70) >> 4)
	switch segment {
	case 0:
		t += 8
	case 1:
		t += 0x108
	default:
		t += 0x108
		t <<= segment - 1
	}
	if value&0x80 == 0 {
		return int16(-t)
	}
	return int16(t)
}

func aLawSegment(pcm int) int {
	for segment := 0; segment < 8; segment++ {
		if pcm <= (0x1f << segment) {
			return segment
		}
	}
	return 8
}
