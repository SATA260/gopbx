// 这个文件负责严格解析客户端文本命令，并校验首包是否合法。

package wsinbound

import (
	"fmt"

	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
	"gopbx/pkg/xjson"
)

func DecodeCommand(message []byte) (*wsproto.CommandEnvelope, error) {
	var cmd wsproto.CommandEnvelope
	if err := xjson.DecodeStrict(message, &cmd); err != nil {
		return nil, err
	}
	if cmd.Command == "" {
		return nil, fmt.Errorf("missing command")
	}
	return &cmd, nil
}

func ValidateFirstCommand(cmd *wsproto.CommandEnvelope) error {
	if !compat.IsFirstCommandAllowed(cmd.Command) {
		return fmt.Errorf("first command must be invite or accept")
	}
	return nil
}
