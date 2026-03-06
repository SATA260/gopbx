// 这个文件实现会话命令分发器，把 WS 命令映射成业务事件响应。

package session

import (
	"time"

	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

type CommandRouter struct{}

func NewCommandRouter() *CommandRouter {
	return &CommandRouter{}
}

func (r *CommandRouter) Route(s *Session, cmd *wsproto.CommandEnvelope) []wsproto.EventEnvelope {
	s.AppendCommand(cmd.Command)

	base := wsproto.EventEnvelope{
		TrackID:   s.ID,
		Timestamp: time.Now().UnixMilli(),
	}

	switch cmd.Command {
	case wsproto.CommandTTS:
		return []wsproto.EventEnvelope{
			mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventTrackStart, Data: map[string]any{"playId": cmd.PlayID}}),
			mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventMetrics, Key: "ttfb.tts.mock", Duration: 0}),
			mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventTrackEnd, Data: map[string]any{"playId": cmd.PlayID}}),
		}
	case wsproto.CommandHistory:
		return []wsproto.EventEnvelope{mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventAddHistory})}
	case wsproto.CommandInterrupt:
		return []wsproto.EventEnvelope{mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventInterruption})}
	case wsproto.CommandHangup:
		return []wsproto.EventEnvelope{mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventHangup})}
	default:
		return []wsproto.EventEnvelope{mergeEvent(base, wsproto.EventEnvelope{Event: compat.EventOther})}
	}
}

func mergeEvent(base, override wsproto.EventEnvelope) wsproto.EventEnvelope {
	if override.TrackID == "" {
		override.TrackID = base.TrackID
	}
	if override.Timestamp == 0 {
		override.Timestamp = base.Timestamp
	}
	return override
}
