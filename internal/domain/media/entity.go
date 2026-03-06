// 这个文件定义媒体领域实体，表达内部流转的媒体数据结构。

package media

type Packet struct {
	TrackID string
	Data    []byte
}
