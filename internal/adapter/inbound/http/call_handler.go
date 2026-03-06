// 这个文件处理通话入口，负责握手建会、首包校验和命令循环。

package httpinbound

import (
	"fmt"
	"net/http"
	"time"

	wsinbound "gopbx/internal/adapter/inbound/ws"
	asroutbound "gopbx/internal/adapter/outbound/asr"
	llmoutbound "gopbx/internal/adapter/outbound/llm"
	ttsoutbound "gopbx/internal/adapter/outbound/tts"
	"gopbx/internal/app/session"
	"gopbx/internal/compat"
	"gopbx/internal/config"
	"gopbx/pkg/wsproto"

	"github.com/labstack/echo/v4"
)

type Handlers struct {
	Config   *config.Config
	Sessions *session.Manager
	Router   *session.CommandRouter
	proxy    *llmoutbound.Proxy
}

func NewHandlers(cfg *config.Config, sessions *session.Manager) *Handlers {
	return &Handlers{
		Config:   cfg,
		Sessions: sessions,
		Router:   session.NewCommandRouter(),
		proxy:    llmoutbound.NewProxy(cfg.LLMProxy),
	}
}

func (h *Handlers) HandleCallWS(c echo.Context) error {
	return h.serveWS(c, session.TypeWebSocket)
}

func (h *Handlers) HandleWebRTCCallWS(c echo.Context) error {
	return h.serveWS(c, session.TypeWebRTC)
}

func (h *Handlers) serveWS(c echo.Context, callType session.Type) error {
	upgrader := wsinbound.NewUpgrader()
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("upgrade websocket: %v", err))
	}
	defer conn.Close()

	messageType, firstMessage, err := conn.ReadMessage()
	if err != nil {
		return nil
	}
	if messageType != 1 {
		_ = wsinbound.WriteError(conn, "first message must be a text command")
		return nil
	}

	cmd, err := wsinbound.DecodeCommand(firstMessage)
	if err != nil {
		_ = wsinbound.WriteError(conn, err.Error())
		return nil
	}
	if err := wsinbound.ValidateFirstCommand(cmd); err != nil {
		_ = wsinbound.WriteError(conn, err.Error())
		return nil
	}
	if err := validateProviders(cmd.Option); err != nil {
		_ = wsinbound.WriteError(conn, err.Error())
		return nil
	}

	sessionID := c.QueryParam("id")
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}

	s := h.Sessions.Create(sessionID, callType, cmd.Option)
	defer session.Cleanup(h.Sessions, s.ID)

	answer := wsproto.EventEnvelope{
		Event:     compat.EventAnswer,
		TrackID:   s.ID,
		Timestamp: time.Now().UnixMilli(),
		SDP:       buildAnswerSDP(callType, cmd.Option),
	}
	if err := wsinbound.WriteEvent(conn, answer); err != nil {
		return nil
	}

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return nil
		}

		if messageType != 1 {
			continue
		}

		command, err := wsinbound.DecodeCommand(payload)
		if err != nil {
			_ = wsinbound.WriteError(conn, err.Error())
			continue
		}

		for _, evt := range h.Router.Route(s, command) {
			if err := wsinbound.WriteEvent(conn, evt); err != nil {
				return nil
			}
		}

		if command.Command == wsproto.CommandHangup {
			return nil
		}
	}
}

func buildAnswerSDP(callType session.Type, option *wsproto.CallOption) string {
	if callType != session.TypeWebRTC {
		return ""
	}
	if option == nil || option.Offer == "" {
		return "v=0\r\ns=gopbx\r\n"
	}
	return option.Offer
}

func validateProviders(option *wsproto.CallOption) error {
	if option == nil {
		return nil
	}
	if option.ASR != nil {
		if err := asroutbound.ValidateProvider(option.ASR.Provider); err != nil {
			return err
		}
	}
	if option.TTS != nil {
		if err := ttsoutbound.ValidateProvider(option.TTS.Provider); err != nil {
			return err
		}
	}
	return nil
}
