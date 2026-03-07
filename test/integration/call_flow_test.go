// 这个文件验证通话生命周期中的关键兼容行为：answer、hangup 与会话清理。

package integration_test

import (
	"testing"
	"time"

	"gopbx/internal/app/session"
	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

func TestCommandRouterTTS(t *testing.T) {
	router := session.NewCommandRouter()
	s := session.NewSession("s1", session.TypeWebRTC, nil)
	playID := "p1"
	result := router.Route(s, &wsproto.CommandEnvelope{Command: wsproto.CommandTTS, PlayID: &playID})
	if len(result.Events) == 0 {
		t.Fatal("expected events for tts command")
	}
}

func TestHangupClosesSessionAndRemovesItFromList(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCall, "hangup-session")

	answer := sendInviteAndReadAnswer(t, conn)
	requireEventName(t, answer, compat.EventAnswer)

	eventually(t, time.Second, func() bool {
		ids := listCallIDs(t, server.URL)
		return len(ids) == 1 && ids[0] == "hangup-session"
	}, "expected session to appear in /call/lists after answer")

	if err := conn.WriteJSON(map[string]any{
		"command":   "hangup",
		"reason":    "done",
		"initiator": "client",
	}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}

	hangup := readEvent(t, conn)
	requireEventName(t, hangup, compat.EventHangup)
	requireEventField(t, hangup, "reason", "done")
	requireEventField(t, hangup, "initiator", "client")

	eventually(t, time.Second, func() bool {
		return len(listCallIDs(t, server.URL)) == 0
	}, "expected hangup session to disappear from /call/lists")

	expectClose(t, conn)
}
