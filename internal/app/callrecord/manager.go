// 这个文件定义话单管理器，负责把会话结束后的话单交给具体 writer 持久化。

package callrecord

import "sync"

type Manager struct {
	mu      sync.Mutex
	writer  Writer
	records []Record
}

func NewManager(writer Writer) *Manager {
	return &Manager{writer: writer, records: make([]Record, 0, 16)}
}

// Write 会先在内存里保留一份最近写入的话单，再把它交给外部 writer。
// 这样测试和后续调试都能直接拿到当前进程里已经归档过的结果。
func (m *Manager) Write(record Record) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.records = append(m.records, record)
	writer := m.writer
	m.mu.Unlock()
	if writer == nil {
		return nil
	}
	return writer.Write(record)
}

func (m *Manager) Records() []Record {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, len(m.records))
	copy(out, m.records)
	return out
}
