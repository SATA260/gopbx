// 这个文件抽象 ICE 提供者，负责为会话协商提供候选服务器列表。

package ice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gopbx/internal/config"
	"gopbx/pkg/wsproto"
)

type Provider interface {
	Get(context.Context) ([]wsproto.ICEServer, error)
}

type StaticProvider struct {
	Servers []wsproto.ICEServer
}

func (p StaticProvider) Get(context.Context) ([]wsproto.ICEServer, error) {
	return p.Servers, nil
}

type RemoteProvider struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func NewRemoteProvider(cfg config.ICEProviderConfig) *RemoteProvider {
	if cfg.Endpoint == "" {
		return nil
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &RemoteProvider{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		apiKey:   cfg.APIKey,
		client:   &http.Client{Timeout: timeout},
	}
}

// Get 会兼容三种常见形态：password、credential 和 null。
// 只要响应本身是合法 JSON，就直接交给统一结构反序列化，便于保留老服务的宽松返回方式。
func (p *RemoteProvider) Get(ctx context.Context) ([]wsproto.ICEServer, error) {
	if p == nil || p.endpoint == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ice provider status: %d", resp.StatusCode)
	}
	var servers []wsproto.ICEServer
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return nil, err
	}
	return servers, nil
}
