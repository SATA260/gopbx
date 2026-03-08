// 这个文件实现会话命令分发器，把 WS 命令映射成兼容既有协议的事件响应。

package session

import (
	"errors"
	"io"

	ttsadapter "gopbx/internal/adapter/outbound/tts"
	mediatrack "gopbx/internal/app/media/track"
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
		ttsTrack := mediatrack.NewTTSTrack(trackID, cmd.Text, cmd.Speaker, cmd.PlayID)
		provider := resolveTTSProvider(s, cmd)
		option := resolveTTSOption(s, cmd)
		audioBytes, chunkCount, err := synthesizeTTS(s, provider, ttsTrack.TrackID(), cmd.Text, option)
		if err != nil {
			s.ClearTrack()
			return CommandResult{Events: []wsproto.EventEnvelope{
				{
					Event:     compat.EventTrackStart,
					TrackID:   ttsTrack.TrackID(),
					Timestamp: timestamp,
				},
				wsproto.NewErrorEvent(ttsTrack.TrackID(), "tts", err.Error()),
				{
					Event:     compat.EventTrackEnd,
					TrackID:   ttsTrack.TrackID(),
					Timestamp: timestamp,
					Duration:  wsproto.Uint64(0),
				},
			}}
		}
		events := []wsproto.EventEnvelope{
			{
				Event:     compat.EventTrackStart,
				TrackID:   ttsTrack.TrackID(),
				Timestamp: timestamp,
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       provider.MetricKey("ttfb"),
				Duration:  wsproto.Uint64(0),
				Data: withProvider(withAudioStats(
					ttsTrack.MetricsData(derefBool(cmd.Streaming), derefBool(cmd.EndOfStream)),
					audioBytes,
					chunkCount,
				), provider.Name()),
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       provider.MetricKey("completed"),
				Duration:  wsproto.Uint64(0),
				Data: map[string]any{
					"trackId":    ttsTrack.TrackID(),
					"length":     len(cmd.Text),
					"audioBytes": audioBytes,
					"chunks":     chunkCount,
					"provider":   provider.Name(),
				},
			},
			{
				Event:     compat.EventTrackEnd,
				TrackID:   ttsTrack.TrackID(),
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
		fileTrack := mediatrack.NewFileTrack(trackID, derefString(cmd.URL), cmd.PlayID)
		events := []wsproto.EventEnvelope{
			{
				Event:     compat.EventTrackStart,
				TrackID:   fileTrack.TrackID(),
				Timestamp: timestamp,
			},
			{
				Event:     compat.EventMetrics,
				Timestamp: timestamp,
				Key:       "completed.play.mock",
				Duration:  wsproto.Uint64(0),
				Data:      fileTrack.MetricsData(),
			},
			{
				Event:     compat.EventTrackEnd,
				TrackID:   fileTrack.TrackID(),
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

func resolveTTSProvider(s *Session, cmd *wsproto.CommandEnvelope) ttsadapter.Provider {
	if option, err := cmd.TTSOption(); err == nil && option != nil && option.Provider != nil {
		return ttsadapter.ResolveProvider(*option.Provider)
	}
	snapshot := s.Snapshot()
	if snapshot.Option != nil && snapshot.Option.TTS != nil && snapshot.Option.TTS.Provider != nil {
		return ttsadapter.ResolveProvider(*snapshot.Option.TTS.Provider)
	}
	return ttsadapter.ResolveProvider("")
}

func resolveTTSOption(s *Session, cmd *wsproto.CommandEnvelope) *wsproto.SynthesisOption {
	if option, err := cmd.TTSOption(); err == nil && option != nil {
		return option
	}
	snapshot := s.Snapshot()
	if snapshot.Option != nil {
		return snapshot.Option.TTS
	}
	return nil
}

// synthesizeTTS 会把 provider 输出的音频流完整消费掉，并统计字节数与 chunk 数。
// 当前阶段还没有把这些音频真正送到客户端，所以这里先以“安全消费 + 统计”为主，为下一步真实出音留接口。

// synthesizeTTS 会根据当前会话类型选择音频输出方式。
// 现在只有 WebRTC 会绑定真实音频 sink，因此真实音频输出只会走 WebRTC；其他会话只消费流并统计，保持协议兼容。
func synthesizeTTS(s *Session, provider ttsadapter.Provider, trackID, text string, option *wsproto.SynthesisOption) (audioBytes int, chunkCount int, err error) {
	stream, err := provider.StartSynthesis(text, option)
	if err != nil {
		return 0, 0, err
	}
	if sink := s.TTSSink(); sink != nil {
		return sink.PlayTTS(trackID, option, stream)
	}
	defer func() {
		closeErr := stream.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return audioBytes, chunkCount, err
			}
			return audioBytes, chunkCount, recvErr
		}
		audioBytes += len(chunk.Data)
		if len(chunk.Data) > 0 {
			chunkCount++
		}
	}
}

func withProvider(data map[string]any, provider string) map[string]any {
	if data == nil {
		data = make(map[string]any, 1)
	}
	data["provider"] = provider
	return data
}

func withAudioStats(data map[string]any, audioBytes, chunkCount int) map[string]any {
	if data == nil {
		data = make(map[string]any, 2)
	}
	data["audioBytes"] = audioBytes
	data["chunks"] = chunkCount
	return data
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
