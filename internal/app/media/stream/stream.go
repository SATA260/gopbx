// 这个文件定义媒体流中枢，负责把入站音频包送入处理器链并返回生成的协议事件。

package stream

import (
	"sync"

	"gopbx/internal/app/media/processor"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
)

type Stream struct {
	mu     sync.RWMutex
	id     string
	chain  *processor.Chain
	closed bool
}

func New(id string, chain *processor.Chain) *Stream {
	return &Stream{id: id, chain: chain}
}

// Push 把一帧媒体数据交给处理器链；如果流已关闭，就直接丢弃。
func (s *Stream) Push(packet Packet) []protocol.Event {
	s.mu.RLock()
	closed := s.closed
	chain := s.chain
	s.mu.RUnlock()
	if closed || chain == nil {
		return nil
	}
	return chain.Process(mediaentity.Packet{TrackID: packet.TrackID, Data: packet.Data})
}

func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

func (s *Stream) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}
