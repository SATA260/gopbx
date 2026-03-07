// 这个文件实现 PCMA 编解码器，当前先保留轻量兼容壳，负责提供类型、采样率和字节级透传。

package codec

type PCMACodec struct{}

func (PCMACodec) Type() Type { return PCMA }

func (PCMACodec) SampleRate() int { return 8000 }

func (PCMACodec) Encode(payload []byte) []byte { return cloneBytes(payload) }

func (PCMACodec) Decode(payload []byte) []byte { return cloneBytes(payload) }
