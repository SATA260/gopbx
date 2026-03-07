// 这个文件处理通话入口，负责握手建会、首包校验和命令循环。

package httpinbound

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"

	wsinbound "gopbx/internal/adapter/inbound/ws"
	asroutbound "gopbx/internal/adapter/outbound/asr"
	llmoutbound "gopbx/internal/adapter/outbound/llm"
	ttsoutbound "gopbx/internal/adapter/outbound/tts"
	"gopbx/internal/app/callrecord"
	"gopbx/internal/app/session"
	"gopbx/internal/compat"
	"gopbx/internal/config"
	"gopbx/pkg/wsproto"

	"github.com/gorilla/websocket"
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
	dumpEnabled := parseDumpFlag(c.QueryParam("dump"))
	dumpWriter := openDumpWriter(h.Config.RecorderPath, sessionID, dumpEnabled)
	defer closeDumpWriter(dumpWriter)

	var activeSession *session.Session
	closeInfo := session.CloseInfo{Cause: session.CloseCauseDisconnect}
	defer func() {
		if activeSession != nil {
			session.Cleanup(h.Sessions, activeSession, closeInfo)
		}
	}()

	messageType, firstMessage, err := conn.ReadMessage()
	if err != nil {
		return nil
	}
	if messageType != 1 {
		_ = writeError(conn, dumpWriter, sessionID, "handle_call", "Invalid message type")
		return nil
	}

	cmd, err := wsinbound.DecodeCommand(firstMessage)
	if err != nil {
		_ = writeError(conn, dumpWriter, sessionID, "handle_call", err.Error())
		return nil
	}
	_ = dumpWriter.WriteCommand(firstMessage)
	if err := wsinbound.ValidateFirstCommand(cmd); err != nil {
		_ = writeError(conn, dumpWriter, sessionID, "handle_call", err.Error())
		return nil
	}
	callOption, err := cmd.CallOption()
	if err != nil {
		_ = writeError(conn, dumpWriter, sessionID, "handle_call", err.Error())
		return nil
	}
	if err := validateProviders(callOption); err != nil {
		_ = writeError(conn, dumpWriter, sessionID, "handle_call", err.Error())
		return nil
	}

	activeSession = h.Sessions.Create(sessionID, callType, callOption)
	activeSession.SetDumpEnabled(dumpEnabled)
	activeSession.BeginHandshake(callOption)
	activeSession.BindCloseFunc(func() {
		_ = conn.Close()
	})

	answer := wsproto.EventEnvelope{
		Event:     compat.EventAnswer,
		TrackID:   activeSession.ID,
		Timestamp: wsproto.NowMillis(),
		SDP:       buildAnswerSDP(callType, callOption),
	}
	if err := writeEvent(conn, dumpWriter, answer); err != nil {
		closeInfo = session.CloseInfo{Cause: session.CloseCauseError, Err: err.Error()}
		activeSession.Fail(err.Error())
		return nil
	}
	activeSession.MarkActive()

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
			_ = writeError(conn, dumpWriter, activeSession.ID, "handle_call", err.Error())
			closeInfo = session.CloseInfo{Cause: session.CloseCauseError, Err: err.Error()}
			activeSession.Fail(err.Error())
			return nil
		}
		_ = dumpWriter.WriteCommand(payload)

		for _, evt := range h.Router.Route(activeSession, command) {
			if err := writeEvent(conn, dumpWriter, evt); err != nil {
				closeInfo = session.CloseInfo{Cause: session.CloseCauseError, Err: err.Error()}
				activeSession.Fail(err.Error())
				return nil
			}
		}

		if command.Command == wsproto.CommandHangup {
			closeInfo = session.CloseInfo{
				Cause:     session.CloseCauseHangup,
				Reason:    derefString(command.Reason),
				Initiator: derefString(command.Initiator),
			}
			activeSession.RequestClose(closeInfo)
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

func parseDumpFlag(raw string) bool {
	if raw == "" {
		return true
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return true
	}
	return enabled
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func openDumpWriter(root, sessionID string, enabled bool) *callrecord.DumpWriter {
	if !enabled {
		return nil
	}
	writer, err := callrecord.OpenDumpWriter(root, sessionID)
	if err != nil {
		return nil
	}
	return writer
}

func closeDumpWriter(writer *callrecord.DumpWriter) {
	if writer != nil {
		_ = writer.Close()
	}
}

func writeError(conn *websocket.Conn, writer *callrecord.DumpWriter, trackID, sender, message string) error {
	return writeEvent(conn, writer, wsproto.NewErrorEvent(trackID, sender, message))
}

func writeEvent(conn *websocket.Conn, writer *callrecord.DumpWriter, evt wsproto.EventEnvelope) error {
	data, err := wsinbound.MarshalEvent(evt)
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return err
	}
	if writer != nil {
		_ = writer.WriteEvent(data)
	}
	return nil
}
