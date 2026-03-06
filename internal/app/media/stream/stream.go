// 这个文件定义媒体流中枢，占位表达多 Track 之间的音频转发管道。

package stream

type Stream struct {
	Packets chan Packet
}

func New() *Stream {
	return &Stream{Packets: make(chan Packet, 32)}
}
