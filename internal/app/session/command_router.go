// 这个文件实现会话命令分发器，把 WS 命令映射成兼容既有协议的事件响应。

package session

import (
	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

type CommandRouter struct{}

func NewCommandRouter() *CommandRouter {
	return &CommandRouter{}
}

func (r *CommandRouter) Route(s *Session, cmd *wsproto.CommandEnvelope) []wsproto.EventEnvelope {
	s.AppendCommand(cmd.Command)

	timestamp := wsproto.NowMillis()

	switch cmd.Command {
	case wsproto.CommandTTS:
		return []wsproto.EventEnvelope{
			{
				Event:     compat.EventTrackStart,
				TrackID:   s.ID,
				Timestamp: timestamp,
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       "ttfb.tts.mock",
				Duration:  wsproto.Uint64(0),
				Data: map[string]any{
					"speaker": cmd.Speaker,
					"playId":  cmd.PlayID,
					"length":  0,
				},
			},
			{
				Event:     compat.EventTrackEnd,
				TrackID:   s.ID,
				Timestamp: timestamp,
				Duration:  wsproto.Uint64(0),
			},
		}
	case wsproto.CommandHistory:
		return []wsproto.EventEnvelope{{
			Event:     compat.EventAddHistory,
			Timestamp: timestamp,
			Sender:    s.ID,
			Speaker:   derefString(cmd.Speaker),
			Text:      cmd.Text,
		}}
	case wsproto.CommandInterrupt:
		return []wsproto.EventEnvelope{{
			Event:     compat.EventInterruption,
			TrackID:   s.ID,
			Timestamp: timestamp,
			Position:  wsproto.Uint64(0),
		}}
	case wsproto.CommandHangup:
		return []wsproto.EventEnvelope{{
			Event:     compat.EventHangup,
			Timestamp: timestamp,
			Reason:    derefString(cmd.Reason),
			Initiator: derefString(cmd.Initiator),
		}}
	case wsproto.CommandCandidate,
		wsproto.CommandPause,
		wsproto.CommandResume,
		wsproto.CommandRefer,
		wsproto.CommandMute,
		wsproto.CommandUnmute,
		wsproto.CommandReject,
		wsproto.CommandPlay:
		return nil
	default:
		return nil
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
