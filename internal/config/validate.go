// 这个文件负责配置校验，确保服务启动前关键参数满足最基本要求。

package config

import (
	"fmt"
	"time"

	"gopbx/pkg/wsproto"
)

func (c *Config) Validate() error {
	if c.Server.Address == "" {
		c.Server.Address = ":8080"
	}
	if c.Server.ShutdownTimeout == "" {
		c.Server.ShutdownTimeout = "10s"
	}
	if c.LLMProxy.Endpoint == "" {
		c.LLMProxy.Endpoint = "https://api.openai.com/v1"
	}
	if len(c.ICEServers) == 0 {
		c.ICEServers = []wsproto.ICEServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}}
	}
	if _, err := time.ParseDuration(c.Server.ShutdownTimeout); err != nil {
		return fmt.Errorf("server.shutdown_timeout must be a valid duration")
	}
	return nil
}
