// 这个文件抽象 ICE 提供者，负责为会话协商提供候选服务器列表。

package ice

import "gopbx/pkg/wsproto"

type Provider interface {
	Get() []wsproto.ICEServer
}

type StaticProvider struct {
	Servers []wsproto.ICEServer
}

func (p StaticProvider) Get() []wsproto.ICEServer {
	return p.Servers
}
