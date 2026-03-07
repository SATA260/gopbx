// 这个文件实现最小链路追踪组件，先保留 span 生命周期接口，便于后续接入正式 tracing 后端。

package observability

import "time"

type Span struct {
	Name      string
	StartedAt time.Time
	EndedAt   time.Time
}

type Tracer struct{}

func NewTracer() *Tracer {
	return &Tracer{}
}

func (t *Tracer) Start(name string) *Span {
	return &Span{Name: name, StartedAt: time.Now().UTC()}
}

func (s *Span) End() time.Duration {
	if s == nil {
		return 0
	}
	s.EndedAt = time.Now().UTC()
	return s.EndedAt.Sub(s.StartedAt)
}
