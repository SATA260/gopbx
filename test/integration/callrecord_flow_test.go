// 这个文件验证会话结束后的话单归档行为，确保生命周期收尾会把关键元数据写入本地文件。

package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopbx/internal/compat"
)

func TestSessionWritesCallRecordOnClose(t *testing.T) {
	app, server := newIntegrationServer(t)
	conn := dialCallWS(t, server.URL, compat.RouteCall, "record-session")

	requireEventName(t, sendInviteAndReadAnswer(t, conn), compat.EventAnswer)

	if err := conn.WriteJSON(map[string]any{
		"command":   "hangup",
		"reason":    "done",
		"initiator": "client",
	}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)

	eventually(t, time.Second, func() bool {
		return len(app.CallRecords.Records()) == 1
	}, "expected one call record in manager after session close")

	path := filepath.Join(app.Config.RecorderPath, "record-session.call.json")
	eventually(t, time.Second, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, "expected call record file to be written")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read call record file: %v", err)
	}
	var record struct {
		CallID          string   `json:"callId"`
		CallType        string   `json:"callType"`
		HangupReason    string   `json:"hangupReason"`
		HangupInitiator string   `json:"hangupInitiator"`
		DumpEventFile   string   `json:"dumpEventFile"`
		Commands        []string `json:"commands"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode call record file: %v", err)
	}
	if record.CallID != "record-session" {
		t.Fatalf("unexpected callId: %s", record.CallID)
	}
	if record.CallType != "websocket" {
		t.Fatalf("unexpected callType: %s", record.CallType)
	}
	if record.HangupReason != "done" || record.HangupInitiator != "client" {
		t.Fatalf("unexpected hangup info: %+v", record)
	}
	if record.DumpEventFile == "" {
		t.Fatal("expected dumpEventFile to be present")
	}
	if len(record.Commands) == 0 || record.Commands[0] != "hangup" {
		t.Fatalf("unexpected commands: %v", record.Commands)
	}
}
