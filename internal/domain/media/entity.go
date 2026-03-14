// 这个文件定义媒体领域实体，表达内部流转的媒体数据结构。

package media

type PacketKind string

const (
	PacketKindAudio      PacketKind = "audio"
	PacketKindSegmentEnd PacketKind = "segmentEnd"
)

type Packet struct {
	TrackID string
	Data    []byte
	Kind    PacketKind
}

func NormalizePacketKind(kind PacketKind) PacketKind {
	if kind == PacketKindSegmentEnd {
		return PacketKindSegmentEnd
	}
	return PacketKindAudio
}

func (p Packet) ResolvedKind() PacketKind {
	return NormalizePacketKind(p.Kind)
}
