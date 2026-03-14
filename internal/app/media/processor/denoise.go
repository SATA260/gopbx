// 这个文件实现最小降噪处理器占位，当前只保留处理器链中的节点位置和命名。

package processor

import (
	mediaentity "gopbx/internal/domain/media"
)

type Denoise struct{}

func NewDenoise() *Denoise { return &Denoise{} }

func (d *Denoise) Name() string { return "denoise" }

func (d *Denoise) Process(packet mediaentity.Packet) Result { return passthrough(packet) }
