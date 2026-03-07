// 这个文件实现 G722 编解码器，当前先保留轻量兼容壳，负责提供类型、采样率和字节级透传。

package codec

type G722Codec struct{}

func (G722Codec) Type() Type { return G722 }

func (G722Codec) SampleRate() int { return 16000 }

func (G722Codec) Encode(payload []byte) []byte { return cloneBytes(payload) }

func (G722Codec) Decode(payload []byte) []byte { return cloneBytes(payload) }
