// 这个文件定义处理器链，负责把上行媒体包按顺序送过降噪、VAD、ASR 和录音等环节。

package processor

import (
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
)

type Processor interface {
	Name() string
	Process(mediaentity.Packet) []protocol.Event
}

type Chain struct {
	processors []Processor
}

func NewChain(processors ...Processor) *Chain {
	return &Chain{processors: processors}
}

func (c *Chain) Names() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.processors))
	for _, processor := range c.processors {
		out = append(out, processor.Name())
	}
	return out
}

// Process 会把同一个媒体包依次送入所有处理器，并把每个处理器产出的协议事件按顺序拼起来。
func (c *Chain) Process(packet mediaentity.Packet) []protocol.Event {
	if c == nil {
		return nil
	}
	events := make([]protocol.Event, 0, len(c.processors))
	for _, processor := range c.processors {
		events = append(events, processor.Process(packet)...)
	}
	return events
}
