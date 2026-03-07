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

	state  State
	dump   bool
	answer string

	closedAt       *time.Time
	answeredAt     *time.Time
	closeRequested bool
	closeInfo      CloseInfo
	closeFn        func()
	closeFnCalled  bool
	done           chan struct{}
	trackSeq       uint64
	currentTrackID string
}

type Snapshot struct {
	ID         string
	Type       Type
	CreatedAt  time.Time
	Option     *wsproto.CallOption
	Commands   []wsproto.CommandName
	State      State
	Dump       bool
	Answer     string
	AnsweredAt *time.Time
	ClosedAt   *time.Time
	CloseInfo  CloseInfo
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

func (s *Session) RecordAnswer(answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answer = answer
	now := time.Now().UTC()
	s.answeredAt = &now
}

func (s *Session) Fail(err string) {
	s.RequestClose(CloseInfo{Cause: CloseCauseError, Err: err})
}

// RequestClose 负责把各种关闭来源收敛成一次关闭请求。
// 这里不直接做最终清理，只负责记录关闭原因、切状态，并触发外部绑定的连接关闭函数。
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

// BindCloseFunc 绑定底层连接的关闭动作。
// 如果会话在绑定前已经进入关闭流程，这里会立刻执行一次，避免出现状态已关闭但连接仍存活的情况。
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

// Finalize 负责把会话推进到 Closed，并返回最终收敛后的关闭信息。
// fallback 用于兜底处理“对端断开但业务侧没显式给出原因”这类场景。
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

// StartTrack 会为一次新的服务端播报分配可追踪的 trackId。
// 如果命令里自带 playId，就优先复用它，方便外部调用方把控制命令和事件串起来。
func (s *Session) StartTrack(prefix string, playID *string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if playID != nil && *playID != "" {
		s.currentTrackID = *playID
		return s.currentTrackID
	}
	s.trackSeq++
	s.currentTrackID = prefix + "-" + s.ID + "-" + formatTrackSeq(s.trackSeq)
	return s.currentTrackID
}

// ClearTrack 在播报自然结束或被 interrupt 打断时清空当前活动 track。
func (s *Session) ClearTrack() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	trackID := s.currentTrackID
	s.currentTrackID = ""
	return trackID
}

func (s *Session) CurrentTrackID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTrackID
}

// Snapshot 返回一份可安全跨协程使用的会话快照，避免清理阶段直接暴露会话内部锁保护的数据结构。
func (s *Session) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	commands := make([]wsproto.CommandName, len(s.Commands))
	copy(commands, s.Commands)
	return Snapshot{
		ID:         s.ID,
		Type:       s.Type,
		CreatedAt:  s.CreatedAt,
		Option:     s.Option,
		Commands:   commands,
		State:      s.state,
		Dump:       s.dump,
		Answer:     s.answer,
		AnsweredAt: s.answeredAt,
		ClosedAt:   s.closedAt,
		CloseInfo:  s.closeInfo,
	}
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

func formatTrackSeq(v uint64) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append(buf, digits[v%10])
		v /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
