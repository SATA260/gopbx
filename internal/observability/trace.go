// 这个文件是链路追踪组件占位，后续用于串联信令、媒体与外部调用链路。

package observability

type Tracer struct{}

func NewTracer() *Tracer {
	return &Tracer{}
}
