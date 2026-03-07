// 这个文件实现 TTS 音轨兼容壳，负责把文本播报命令整理成稳定的 track 标识和事件元数据。

package track

type TTSTrack struct {
	ID      string
	Text    string
	Speaker string
	PlayID  string
}

func NewTTSTrack(id, text string, speaker, playID *string) *TTSTrack {
	return &TTSTrack{
		ID:      id,
		Text:    text,
		Speaker: derefString(speaker),
		PlayID:  derefString(playID),
	}
}

func (t *TTSTrack) TrackID() string {
	if t == nil {
		return ""
	}
	if t.PlayID != "" {
		return t.PlayID
	}
	return t.ID
}

func (t *TTSTrack) MetricsData(streaming, endOfStream bool) map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"speaker":     t.Speaker,
		"playId":      t.PlayID,
		"streaming":   streaming,
		"endOfStream": endOfStream,
		"length":      len(t.Text),
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
