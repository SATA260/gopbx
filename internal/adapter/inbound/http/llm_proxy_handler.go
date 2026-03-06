// 这个文件处理 LLM 代理入口，把兼容路径转发到上游模型服务。

package httpinbound

import "github.com/labstack/echo/v4"

func (h *Handlers) HandleLLMProxy(c echo.Context) error {
	return h.proxy.Proxy(c)
}
