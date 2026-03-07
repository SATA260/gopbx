// 这个文件实现会话管理器，负责维护 active calls 的创建、查询和销毁。

package session

import (
	"sort"
	"sync"
	"time"

	"gopbx/pkg/wsproto"
)

type Type string

const (
	TypeWebSocket Type = "websocket"
	TypeWebRTC    Type = "webrtc"
)

type Summary struct {
	ID        string              `json:"id"`
	CallType  string              `json:"call_type"`
	CreatedAt time.Time           `json:"created_at"`
	Option    *wsproto.CallOption `json:"option,omitempty"`
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func (m *Manager) Create(id string, typ Type, option *wsproto.CallOption) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := NewSession(id, typ, option)
	m.sessions[id] = s
	return s
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

func (m *Manager) Kill(id string) bool {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	s.RequestClose(CloseInfo{
		Cause:     CloseCauseKill,
		Reason:    "killed",
		Initiator: "system",
	})
	return true
}

func (m *Manager) List() []Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Summary, 0, len(m.sessions))
	for _, s := range m.sessions {
		if !s.VisibleInList() {
			continue
		}
		out = append(out, Summary{
			ID:        s.ID,
			CallType:  string(s.Type),
			CreatedAt: s.CreatedAt,
			Option:    s.Option,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	return out
}
