// 这个文件集中维护对外 HTTP 路由常量，用来保证接口路径长期稳定。

package compat

const (
	RouteCall        = "/call"
	RouteCallWebRTC  = "/call/webrtc"
	RouteCallLists   = "/call/lists"
	RouteCallKill    = "/call/kill/:id"
	RouteICEServers  = "/iceservers"
	RouteLLMProxyAny = "/llm/v1/*"
)
