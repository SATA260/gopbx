// 这个文件负责配置校验，确保服务启动前关键参数满足最基本要求。

package config

import "fmt"

func (c *Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}
	if c.Server.ShutdownTimeout == "" {
		c.Server.ShutdownTimeout = "10s"
	}
	return nil
}
