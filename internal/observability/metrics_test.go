// 这个文件验证最小指标组件的行为，确保迁移阶段的内存指标记录和快照输出稳定可用。

package observability

import "testing"

func TestMetricsSnapshot(t *testing.T) {
	metrics := NewMetrics()
	metrics.Inc("completed.session")
	metrics.Inc("completed.session")
	metrics.Observe("ttfb.tts.mock", 12)
	metrics.Observe("ttfb.tts.mock", 18)

	snapshot := metrics.Snapshot()
	if snapshot.Counters["completed.session"] != 2 {
		t.Fatalf("unexpected counter value: %d", snapshot.Counters["completed.session"])
	}
	if len(snapshot.Durations["ttfb.tts.mock"]) != 2 {
		t.Fatalf("unexpected duration count: %v", snapshot.Durations["ttfb.tts.mock"])
	}
}
