// 这个文件验证管理接口与会话生命周期的联动行为。

package integration_test

import (
	"testing"
	"time"

	"gopbx/internal/compat"
)

func TestKillCallClosesSessionAndUpdatesList(t *testing.T) {
	_, server := newIntegrationServer(t)
	conn1 := dialCallWS(t, server.URL, compat.RouteCall, "session-1")
	conn2 := dialCallWS(t, server.URL, compat.RouteCall, "session-2")

	requireEventName(t, sendInviteAndReadAnswer(t, conn1), compat.EventAnswer)
	requireEventName(t, sendInviteAndReadAnswer(t, conn2), compat.EventAnswer)

	eventually(t, time.Second, func() bool {
		ids := listCallIDs(t, server.URL)
		return len(ids) == 2
	}, "expected two sessions in /call/lists after both invites")

	postKill(t, server.URL, "session-1")

	eventually(t, time.Second, func() bool {
		ids := listCallIDs(t, server.URL)
		return len(ids) == 1 && ids[0] == "session-2"
	}, "expected killed session to disappear from /call/lists")

	expectClose(t, conn1)

	if err := conn2.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send cleanup hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn2), compat.EventHangup)
	eventually(t, time.Second, func() bool {
		return len(listCallIDs(t, server.URL)) == 0
	}, "expected all sessions to be removed after cleanup hangup")
	expectClose(t, conn2)
}
