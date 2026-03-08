// 这个文件定义 Go SDK 公开类型，方便其他程序通过统一的 HTTP/WS 客户端接入网关。

package gosdk

import (
	"context"
	"net/http"
	"time"

	"gopbx/pkg/wsproto"

	"github.com/gorilla/websocket"
)

type Command = wsproto.CommandEnvelope
type Event = wsproto.EventEnvelope
type CallOption = wsproto.CallOption
type ASRConfig = wsproto.ASRConfig
type SynthesisOption = wsproto.SynthesisOption
type ICEServer = wsproto.ICEServer

type ClientOptions struct {
	HTTPBaseURL string
	WSBaseURL   string
	HTTPClient  *http.Client
	Dialer      *websocket.Dialer
}

type DialOptions struct {
	SessionID string
	Dump      *bool
	Header    http.Header
}

type ListCallsResponse struct {
	Calls []CallSummary `json:"calls"`
}

type CallSummary struct {
	ID        string      `json:"id"`
	CallType  string      `json:"call_type"`
	CreatedAt string      `json:"created_at"`
	Option    *CallOption `json:"option"`
}

type Session struct {
	callType string
	conn     *websocket.Conn
}

type ReadEventOption struct {
	Timeout time.Duration
}

type ReadBinaryOption struct {
	Timeout time.Duration
}

type ReadEventLoopHandler func(context.Context, Event) error
