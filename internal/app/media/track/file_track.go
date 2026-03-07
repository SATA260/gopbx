// 这个文件实现文件音轨兼容壳，负责把提示音/文件播报命令整理成稳定的 track 标识和事件元数据。

package track

type FileTrack struct {
	ID     string
	URL    string
	PlayID string
}

func NewFileTrack(id, url string, playID *string) *FileTrack {
	return &FileTrack{ID: id, URL: url, PlayID: derefString(playID)}
}

func (t *FileTrack) TrackID() string {
	if t == nil {
		return ""
	}
	if t.PlayID != "" {
		return t.PlayID
	}
	return t.ID
}

func (t *FileTrack) MetricsData() map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"trackId": t.TrackID(),
		"url":     t.URL,
	}
}
