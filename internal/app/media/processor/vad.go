// 这个文件实现最小 VAD 处理器占位，当前用于保留处理器链顺序并为后续 speaking/silence 事件预留入口。

package processor

import (
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
)

type VAD struct{}

func NewVAD() *VAD { return &VAD{} }

func (v *VAD) Name() string { return "vad" }

func (v *VAD) Process(mediaentity.Packet) []protocol.Event { return nil }
