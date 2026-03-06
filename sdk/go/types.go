// 这个文件定义 Go SDK 复用的协议类型，方便外部调用方直接接入网关。

package gosdk

import "gopbx/pkg/wsproto"

type Command = wsproto.CommandEnvelope
type Event = wsproto.EventEnvelope

type ClientOptions struct {
	HTTPBaseURL string
	WSURL       string
}
