// 这个文件定义媒体包模型，表示在内部链路中流转的最小音频数据单元。

package stream

type Packet struct {
	TrackID string
	Data    []byte
}
