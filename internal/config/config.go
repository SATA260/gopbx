// 这个文件定义配置模型与加载逻辑，负责把 YAML 配置转成业务可用对象。

package config

import (
	"os"
	"time"

	"gopbx/pkg/wsproto"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig        `yaml:"server"`
	LLMProxy   LLMProxyConfig      `yaml:"llmProxy"`
	ICEServers []wsproto.ICEServer `yaml:"iceServers"`
}

type ServerConfig struct {
	Address         string `yaml:"address"`
	ShutdownTimeout string `yaml:"shutdownTimeout"`
}

func (c ServerConfig) ShutdownTimeoutDuration() time.Duration {
	if c.ShutdownTimeout == "" {
		return 10 * time.Second
	}
	d, err := time.ParseDuration(c.ShutdownTimeout)
	if err != nil {
		return 10 * time.Second
	}
	return d
}

type LLMProxyConfig struct {
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Address:         ":8080",
			ShutdownTimeout: "10s",
		},
		LLMProxy: LLMProxyConfig{
			Endpoint: "https://api.openai.com",
		},
		ICEServers: []wsproto.ICEServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}},
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		cfg := Default()
		return cfg, cfg.Validate()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, cfg.Validate()
}
