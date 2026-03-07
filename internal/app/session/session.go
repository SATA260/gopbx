// 这个文件定义单会话状态模型，用来记录会话基础信息、生命周期和关闭控制。

package session

import (
	"sync"
	"time"

	"gopbx/pkg/wsproto"
)

type State string

const (
	StateConnected   State = "connected"
	StateHandshaking State = "handshaking"
	StateActive      State = "active"
	StateClosing     State = "closing"
	StateClosed      State = "closed"
	StateFailed      State = "failed"
)

type CloseCause string

const (
	CloseCauseHangup     CloseCause = "hangup"
	CloseCauseKill       CloseCause = "kill"
	CloseCauseDisconnect CloseCause = "disconnect"
	CloseCauseError      CloseCause = "error"
)

type CloseInfo struct {
	Cause     CloseCause
	Reason    string
	Initiator string
	Err       string
}

type Session struct {
	mu sync.RWMutex

	ID        string
	Type      Type
	CreatedAt time.Time
	Option    *wsproto.CallOption
	Commands  []wsproto.CommandName

	state State
	dump  bool

	closedAt       *time.Time
	closeRequested bool
	closeInfo      CloseInfo
	closeFn        func()
	closeFnCalled  bool
	done           chan struct{}
}

func NewSession(id string, typ Type, option *wsproto.CallOption) *Session {
	return &Session{
		ID:        id,
		Type:      typ,
		CreatedAt: time.Now().UTC(),
		Option:    option,
		Commands:  make([]wsproto.CommandName, 0, 8),
		state:     StateConnected,
		dump:      true,
		done:      make(chan struct{}),
	}
}

func (s *Session) AppendCommand(name wsproto.CommandName) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, name)
}

func (s *Session) SetDumpEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dump = enabled
}

func (s *Session) DumpEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dump
}

func (s *Session) BeginHandshake(option *wsproto.CallOption) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isTerminalLocked() {
		return
	}
	if option != nil {
		s.Option = option
	}
	s.state = StateHandshaking
}

func (s *Session) MarkActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isTerminalLocked() {
		return
	}
	s.state = StateActive
}

func (s *Session) Fail(err string) {
	s.RequestClose(CloseInfo{Cause: CloseCauseError, Err: err})
}

func (s *Session) RequestClose(info CloseInfo) {
	var closeFn func()

	s.mu.Lock()
	if s.closeRequested {
		if s.closeInfo.Cause == "" {
			s.closeInfo.Cause = info.Cause
		}
		if s.closeInfo.Reason == "" {
			s.closeInfo.Reason = info.Reason
		}
		if s.closeInfo.Initiator == "" {
			s.closeInfo.Initiator = info.Initiator
		}
		if s.closeInfo.Err == "" {
			s.closeInfo.Err = info.Err
		}
	} else {
		s.closeRequested = true
		s.closeInfo = normalizeCloseInfo(info)
	}

	if !s.isClosedLocked() {
		if s.closeInfo.Cause == CloseCauseError {
			s.state = StateFailed
		} else {
			s.state = StateClosing
		}
	}

	if s.closeFn != nil && !s.closeFnCalled {
		s.closeFnCalled = true
		closeFn = s.closeFn
	}
	s.mu.Unlock()

	if closeFn != nil {
		closeFn()
	}
}

func (s *Session) BindCloseFunc(fn func()) {
	var callNow bool

	s.mu.Lock()
	s.closeFn = fn
	if s.closeRequested && s.closeFn != nil && !s.closeFnCalled {
		s.closeFnCalled = true
		callNow = true
	}
	s.mu.Unlock()

	if callNow {
		fn()
	}
}

func (s *Session) Finalize(fallback CloseInfo) CloseInfo {
	s.mu.Lock()
	info := normalizeCloseInfo(fallback)
	if s.closeRequested {
		info = mergeCloseInfo(s.closeInfo, info)
	} else {
		s.closeRequested = true
	}
	s.closeInfo = info
	if !s.isClosedLocked() {
		now := time.Now().UTC()
		s.closedAt = &now
		s.state = StateClosed
	}
	done := s.done
	s.done = nil
	s.mu.Unlock()

	if done != nil {
		close(done)
	}
	return info
}

func (s *Session) Status() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Session) CloseInfo() CloseInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closeInfo
}

func (s *Session) VisibleInList() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateConnected || s.state == StateHandshaking || s.state == StateActive
}

func (s *Session) Done() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

func (s *Session) isTerminalLocked() bool {
	return s.state == StateClosing || s.state == StateClosed || s.state == StateFailed
}

func (s *Session) isClosedLocked() bool {
	return s.state == StateClosed
}

func normalizeCloseInfo(info CloseInfo) CloseInfo {
	if info.Cause == "" {
		info.Cause = CloseCauseDisconnect
	}
	return info
}

func mergeCloseInfo(primary, fallback CloseInfo) CloseInfo {
	if primary.Cause == "" {
		primary.Cause = fallback.Cause
	}
	if primary.Reason == "" {
		primary.Reason = fallback.Reason
	}
	if primary.Initiator == "" {
		primary.Initiator = fallback.Initiator
	}
	if primary.Err == "" {
		primary.Err = fallback.Err
	}
	return primary
}
