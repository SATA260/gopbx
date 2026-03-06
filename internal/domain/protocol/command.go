// 这个文件提供协议命令的领域别名，方便业务层按领域语义引用命令模型。

package protocol

import "gopbx/pkg/wsproto"

type Command = wsproto.CommandEnvelope
