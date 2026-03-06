// 这个文件定义单会话状态模型，用来记录会话基础信息和命令轨迹。

package session

import (
	"sync"
	"time"

	"gopbx/pkg/wsproto"
)

type Session struct {
	mu        sync.Mutex
	ID        string
	Type      Type
	CreatedAt time.Time
	Option    *wsproto.CallOption
	Commands  []wsproto.CommandName
}

func NewSession(id string, typ Type, option *wsproto.CallOption) *Session {
	return &Session{
		ID:        id,
		Type:      typ,
		CreatedAt: time.Now().UTC(),
		Option:    option,
		Commands:  make([]wsproto.CommandName, 0, 8),
	}
}

func (s *Session) AppendCommand(name wsproto.CommandName) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, name)
}
