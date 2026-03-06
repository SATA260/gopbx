// 这个文件定义命令兼容规则，重点限制会话首包必须走合法握手流程。

package compat

import "gopbx/pkg/wsproto"

func IsFirstCommandAllowed(name wsproto.CommandName) bool {
	return name == wsproto.CommandInvite || name == wsproto.CommandAccept
}
