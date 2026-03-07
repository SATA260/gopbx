// 这个文件实现 PCMU 编解码器，当前先保留轻量兼容壳，负责提供类型、采样率和字节级透传。

package codec

type PCMUCodec struct{}

func (PCMUCodec) Type() Type { return PCMU }

func (PCMUCodec) SampleRate() int { return 8000 }

func (PCMUCodec) Encode(payload []byte) []byte { return cloneBytes(payload) }

func (PCMUCodec) Decode(payload []byte) []byte { return cloneBytes(payload) }
