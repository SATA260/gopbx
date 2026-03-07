// 这个文件实现最小 dump writer，用来按 JSONL 记录会话 command/event 轨迹。

package callrecord

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DumpEntryType string

const (
	DumpEntryEvent   DumpEntryType = "event"
	DumpEntryCommand DumpEntryType = "command"
)

type DumpEntry struct {
	Type      DumpEntryType `json:"type"`
	Timestamp uint64        `json:"timestamp"`
	Content   string        `json:"content"`
}

type DumpWriter struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// OpenDumpWriter 按会话维度打开一个 JSONL 文件。
// 这里沿用每会话一个文件的方式，便于直接把同一通会话的 command/event 时序打包归档。
func OpenDumpWriter(root, sessionID string) (*DumpWriter, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(root, sessionID+".events.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &DumpWriter{file: file, path: path}, nil
}

func (w *DumpWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

func (w *DumpWriter) WriteCommand(content []byte) error {
	return w.write(DumpEntryCommand, content)
}

func (w *DumpWriter) WriteEvent(content []byte) error {
	return w.write(DumpEntryEvent, content)
}

func (w *DumpWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// write 是内部统一落盘入口。
// 它只负责把原始协议 JSON 包上一层元信息，不改写 content，后续排查时可以直接回放原始命令和事件。
func (w *DumpWriter) write(kind DumpEntryType, content []byte) error {
	if w == nil {
		return nil
	}

	entry := DumpEntry{
		Type:      kind,
		Timestamp: uint64(time.Now().UnixMilli()),
		Content:   string(content),
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	if _, err := w.file.Write(append(line, '\n')); err != nil {
		return err
	}
	return w.file.Sync()
}
