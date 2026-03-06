// 这个文件是指标组件占位，后续用于接入通话耗时与质量指标。

package observability

type Metrics struct{}

func NewMetrics() *Metrics {
	return &Metrics{}
}
