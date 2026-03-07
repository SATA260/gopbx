// 这个文件实现最小录音处理器占位，当前先统计写入字节数，为后续真实录音归档保留接口位置。

package processor

import (
	"sync"

	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
)

type Recorder struct {
	mu       sync.Mutex
	bytesSum uint64
}

func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) Name() string { return "recorder" }

func (r *Recorder) Process(packet mediaentity.Packet) []protocol.Event {
	r.mu.Lock()
	r.bytesSum += uint64(len(packet.Data))
	r.mu.Unlock()
	return nil
}

func (r *Recorder) Bytes() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytesSum
}
