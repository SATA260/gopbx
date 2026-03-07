// 这个文件实现会话命令分发器，把 WS 命令映射成兼容既有协议的事件响应。

package session

import (
	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

type CommandResult struct {
	Events []wsproto.EventEnvelope
	Close  *CloseInfo
}

type CommandRouter struct{}

func NewCommandRouter() *CommandRouter {
	return &CommandRouter{}
}

// Route 负责把业务命令翻译成对外事件序列。
// 这里先实现兼容壳：优先保证事件名、字段和顺序稳定，再逐步把底层媒体能力替换成真实实现。
func (r *CommandRouter) Route(s *Session, cmd *wsproto.CommandEnvelope) CommandResult {
	s.AppendCommand(cmd.Command)

	timestamp := wsproto.NowMillis()

	switch cmd.Command {
	case wsproto.CommandTTS:
		trackID := s.StartTrack("tts", cmd.PlayID)
		events := []wsproto.EventEnvelope{
			{
				Event:     compat.EventTrackStart,
				TrackID:   trackID,
				Timestamp: timestamp,
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       "ttfb.tts.mock",
				Duration:  wsproto.Uint64(0),
				Data: map[string]any{
					"speaker":     derefString(cmd.Speaker),
					"playId":      derefString(cmd.PlayID),
					"streaming":   derefBool(cmd.Streaming),
					"endOfStream": derefBool(cmd.EndOfStream),
					"length":      len(cmd.Text),
				},
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       "completed.tts.mock",
				Duration:  wsproto.Uint64(0),
				Data: map[string]any{
					"trackId": trackID,
					"length":  len(cmd.Text),
				},
			},
			{
				Event:     compat.EventTrackEnd,
				TrackID:   trackID,
				Timestamp: timestamp,
				Duration:  wsproto.Uint64(0),
			},
		}
		s.ClearTrack()
		closeInfo := autoHangupClose(cmd)
		if closeInfo != nil {
			events = append(events, hangupEvent(timestamp, closeInfo))
		}
		return CommandResult{Events: events, Close: closeInfo}
	case wsproto.CommandPlay:
		trackID := s.StartTrack("play", cmd.PlayID)
		events := []wsproto.EventEnvelope{
			{
				Event:     compat.EventTrackStart,
				TrackID:   trackID,
				Timestamp: timestamp,
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       "completed.play.mock",
				Duration:  wsproto.Uint64(0),
				Data: map[string]any{
					"trackId": trackID,
					"url":     derefString(cmd.URL),
				},
			},
			{
				Event:     compat.EventTrackEnd,
				TrackID:   trackID,
				Timestamp: timestamp,
				Duration:  wsproto.Uint64(0),
			},
		}
		s.ClearTrack()
		closeInfo := autoHangupClose(cmd)
		if closeInfo != nil {
			events = append(events, hangupEvent(timestamp, closeInfo))
		}
		return CommandResult{Events: events, Close: closeInfo}
	case wsproto.CommandHistory:
		return CommandResult{Events: []wsproto.EventEnvelope{{
			Event:     compat.EventAddHistory,
			Timestamp: timestamp,
			Sender:    s.ID,
			Speaker:   derefString(cmd.Speaker),
			Text:      cmd.Text,
		}}}
	case wsproto.CommandInterrupt:
		trackID := s.ClearTrack()
		if trackID == "" {
			trackID = s.ID
		}
		return CommandResult{Events: []wsproto.EventEnvelope{{
			Event:     compat.EventInterruption,
			TrackID:   trackID,
			Timestamp: timestamp,
			Position:  wsproto.Uint64(0),
		}}}
	case wsproto.CommandHangup:
		return CommandResult{
			Events: []wsproto.EventEnvelope{{
				Event:     compat.EventHangup,
				Timestamp: timestamp,
				Reason:    derefString(cmd.Reason),
				Initiator: derefString(cmd.Initiator),
			}},
			Close: &CloseInfo{
				Cause:     CloseCauseHangup,
				Reason:    derefString(cmd.Reason),
				Initiator: derefString(cmd.Initiator),
			},
		}
	case wsproto.CommandCandidate,
		wsproto.CommandPause,
		wsproto.CommandResume,
		wsproto.CommandRefer,
		wsproto.CommandMute,
		wsproto.CommandUnmute,
		wsproto.CommandReject:
		return CommandResult{}
	default:
		return CommandResult{}
	}
}

func autoHangupClose(cmd *wsproto.CommandEnvelope) *CloseInfo {
	if cmd.AutoHangup == nil || !*cmd.AutoHangup {
		return nil
	}
	return &CloseInfo{
		Cause:     CloseCauseHangup,
		Reason:    "autohangup",
		Initiator: "system",
	}
}

func hangupEvent(timestamp int64, info *CloseInfo) wsproto.EventEnvelope {
	if info == nil {
		return wsproto.EventEnvelope{}
	}
	return wsproto.EventEnvelope{
		Event:     compat.EventHangup,
		Timestamp: timestamp,
		Reason:    info.Reason,
		Initiator: info.Initiator,
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}
