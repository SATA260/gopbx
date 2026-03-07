// 这个文件实现最小指标组件，用来在迁移阶段先记录关键计数和耗时，后续再替换成正式监控后端。

package observability

import "sync"

type MetricsSnapshot struct {
	Counters  map[string]uint64
	Durations map[string][]uint64
}

type Metrics struct {
	mu        sync.Mutex
	counters  map[string]uint64
	durations map[string][]uint64
}

func NewMetrics() *Metrics {
	return &Metrics{
		counters:  make(map[string]uint64),
		durations: make(map[string][]uint64),
	}
}

func (m *Metrics) Inc(name string) {
	if m == nil || name == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name]++
}

func (m *Metrics) Observe(name string, value uint64) {
	if m == nil || name == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[name] = append(m.durations[name], value)
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	counters := make(map[string]uint64, len(m.counters))
	for key, value := range m.counters {
		counters[key] = value
	}
	durations := make(map[string][]uint64, len(m.durations))
	for key, values := range m.durations {
		copyValues := make([]uint64, len(values))
		copy(copyValues, values)
		durations[key] = copyValues
	}
	return MetricsSnapshot{Counters: counters, Durations: durations}
}
