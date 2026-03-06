// 这个文件处理通话入口，负责握手建会、首包校验和命令循环。

package httpinbound

import (
	"crypto/rand"
	"fmt"
	"net/http"

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

	sessionID := c.QueryParam("id")
	if sessionID == "" {
		sessionID = newSessionID()
	}

	messageType, firstMessage, err := conn.ReadMessage()
	if err != nil {
		return nil
	}
	if messageType != 1 {
		_ = wsinbound.WriteError(conn, sessionID, "handle_call", "Invalid message type")
		return nil
	}

	cmd, err := wsinbound.DecodeCommand(firstMessage)
	if err != nil {
		_ = wsinbound.WriteError(conn, sessionID, "handle_call", err.Error())
		return nil
	}
	if err := wsinbound.ValidateFirstCommand(cmd); err != nil {
		_ = wsinbound.WriteError(conn, sessionID, "handle_call", err.Error())
		return nil
	}
	callOption, err := cmd.CallOption()
	if err != nil {
		_ = wsinbound.WriteError(conn, sessionID, "handle_call", err.Error())
		return nil
	}
	if err := validateProviders(callOption); err != nil {
		_ = wsinbound.WriteError(conn, sessionID, "handle_call", err.Error())
		return nil
	}

	s := h.Sessions.Create(sessionID, callType, callOption)
	defer session.Cleanup(h.Sessions, s.ID)

	answer := wsproto.EventEnvelope{
		Event:     compat.EventAnswer,
		TrackID:   s.ID,
		Timestamp: wsproto.NowMillis(),
		SDP:       buildAnswerSDP(callType, callOption),
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
	if option == nil || option.Offer == nil || *option.Offer == "" {
		return "v=0\r\ns=gopbx\r\n"
	}
	return *option.Offer
}

func validateProviders(option *wsproto.CallOption) error {
	if option == nil {
		return nil
	}
	if option.ASR != nil && option.ASR.Provider != nil {
		if err := asroutbound.ValidateProvider(*option.ASR.Provider); err != nil {
			return err
		}
	}
	if option.TTS != nil && option.TTS.Provider != nil {
		if err := ttsoutbound.ValidateProvider(*option.TTS.Provider); err != nil {
			return err
		}
	}
	return nil
}

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess-%d", wsproto.NowMillis())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	)
}
