// 这个文件负责配置校验，确保服务启动前关键参数满足最基本要求。

package config

import (
	"fmt"
	"time"
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
	if c.RecorderPath == "" {
		c.RecorderPath = "./tmp/recordings"
	}
	if _, err := time.ParseDuration(c.Server.ShutdownTimeout); err != nil {
		return fmt.Errorf("server.shutdown_timeout must be a valid duration")
	}
	return nil
}
