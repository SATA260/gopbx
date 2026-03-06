// 这个文件提供协议事件的领域别名，方便业务层按领域语义引用事件模型。

package protocol

import "gopbx/pkg/wsproto"

type Event = wsproto.EventEnvelope
