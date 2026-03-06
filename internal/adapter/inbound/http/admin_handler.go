// 这个文件处理管理接口，负责查询活跃通话和执行强制挂断。

package httpinbound

import (
	"gopbx/pkg/wsproto"

	"github.com/labstack/echo/v4"
)

type listCallsResponse struct {
	Calls []listCallItem `json:"calls"`
}

type listCallItem struct {
	ID        string             `json:"id"`
	CallType  string             `json:"call_type"`
	CreatedAt string             `json:"created_at"`
	Option    *sessionCallOption `json:"option"`
}

type sessionCallOption = wsproto.CallOption

func (h *Handlers) HandleListCalls(c echo.Context) error {
	summaries := h.Sessions.List()
	items := make([]listCallItem, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, listCallItem{
			ID:        summary.ID,
			CallType:  summary.CallType,
			CreatedAt: summary.CreatedAt.UTC().Format("2006-01-02T15:04:05-07:00"),
			Option:    (*sessionCallOption)(summary.Option),
		})
	}
	return c.JSON(200, listCallsResponse{Calls: items})
}

func (h *Handlers) HandleKillCall(c echo.Context) error {
	id := c.Param("id")
	h.Sessions.Kill(id)
	return c.JSON(200, true)
}
