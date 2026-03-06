// 这个文件定义处理器链，占位组织降噪、VAD、ASR 和录音等处理环节。

package processor

type Chain struct {
	Names []string
}

func NewChain(names ...string) *Chain {
	return &Chain{Names: names}
}
