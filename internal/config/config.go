// 这个文件定义配置模型与加载逻辑，负责用 koanf 把环境变量配置转成业务可用对象。

package config

import (
	"encoding/json"
	"strings"
	"time"

	"gopbx/pkg/wsproto"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Server       ServerConfig
	LLMProxy     LLMProxyConfig
	RecorderPath string
	ICEServers   []wsproto.ICEServer
}

type rawConfig struct {
	Server       ServerConfig   `koanf:"server"`
	LLMProxy     LLMProxyConfig `koanf:"llm_proxy"`
	RecorderPath string         `koanf:"recorder_path"`
	ICEServers   string         `koanf:"ice_servers"`
}

type ServerConfig struct {
	Address         string `koanf:"address"`
	ShutdownTimeout string `koanf:"shutdown_timeout"`
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
	Endpoint string `koanf:"endpoint"`
	APIKey   string `koanf:"api_key"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Address:         ":8080",
			ShutdownTimeout: "10s",
		},
		LLMProxy: LLMProxyConfig{
			Endpoint: "https://api.openai.com/v1",
		},
		RecorderPath: "./tmp/recordings",
		ICEServers:   nil,
	}
}

func Load() (*Config, error) {
	defaults := Default()
	k := koanf.New(".")

	err := k.Load(env.Provider("GOPBX_", ".", func(s string) string {
		key := strings.TrimPrefix(s, "GOPBX_")
		key = strings.ToLower(key)
		key = strings.ReplaceAll(key, "__", ".")
		return key
	}), nil)
	if err != nil {
		return nil, err
	}

	raw := rawConfig{
		Server:       defaults.Server,
		LLMProxy:     defaults.LLMProxy,
		RecorderPath: defaults.RecorderPath,
	}
	if len(defaults.ICEServers) > 0 {
		data, err := json.Marshal(defaults.ICEServers)
		if err != nil {
			return nil, err
		}
		raw.ICEServers = string(data)
	}

	if err := k.Unmarshal("", &raw); err != nil {
		return nil, err
	}

	cfg := &Config{
		Server:       raw.Server,
		LLMProxy:     raw.LLMProxy,
		RecorderPath: raw.RecorderPath,
	}

	if raw.ICEServers != "" {
		if err := json.Unmarshal([]byte(raw.ICEServers), &cfg.ICEServers); err != nil {
			return nil, err
		}
	}

	return cfg, cfg.Validate()
}
