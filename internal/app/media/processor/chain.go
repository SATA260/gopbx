// 这个文件定义处理器链，负责把上行媒体包按顺序送过降噪、VAD、ASR 和录音等环节。

package processor

import (
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
)

type Processor interface {
	Name() string
	Process(mediaentity.Packet) Result
}

type Closer interface {
	Close() error
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
	packets := []mediaentity.Packet{packet}
	events := make([]protocol.Event, 0, len(c.processors))
	for _, processor := range c.processors {
		if len(packets) == 0 {
			break
		}
		nextPackets := make([]mediaentity.Packet, 0, len(packets))
		for _, current := range packets {
			result := processor.Process(current)
			events = append(events, result.Events...)
			nextPackets = append(nextPackets, result.Packets...)
		}
		packets = nextPackets
	}
	return events
}

// Close 会依次关闭支持收尾的处理器。
// 会话型 ASR 在这里释放底层 session，避免连接已经结束但上游流式识别还悬挂着。
func (c *Chain) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	for _, processor := range c.processors {
		closer, ok := processor.(Closer)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
