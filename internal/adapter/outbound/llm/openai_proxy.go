// 这个文件实现 LLM 出站代理，负责把请求透传到上游兼容接口并回流响应。

package llmoutbound

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopbx/internal/config"

	"github.com/labstack/echo/v4"
)

type Proxy struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func NewProxy(cfg config.LLMProxyConfig) *Proxy {
	return &Proxy{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		apiKey:   cfg.APIKey,
		client:   http.DefaultClient,
	}
}

func (p *Proxy) Proxy(c echo.Context) error {
	if p.endpoint == "" {
		return echo.NewHTTPError(http.StatusNotImplemented, "llm proxy endpoint is not configured")
	}

	path := strings.TrimPrefix(c.Param("*"), "/")
	forwardURL, err := url.Parse(p.endpoint + "/" + path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("parse endpoint: %v", err))
	}
	forwardURL.RawQuery = c.Request().URL.RawQuery

	req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodPost, forwardURL.String(), c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("build request: %v", err))
	}

	for key, values := range c.Request().Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if req.Header.Get(echo.HeaderAuthorization) == "" && p.apiKey != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("proxy upstream: %v", err))
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			c.Response().Header().Add(key, value)
		}
	}
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(resp.StatusCode)
	_, err = io.Copy(c.Response(), resp.Body)
	return err
}
