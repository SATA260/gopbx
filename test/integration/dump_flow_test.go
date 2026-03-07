// 这个文件验证 dump 参数的兼容行为，确保 command/event 轨迹默认落盘且可关闭。

package integration_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopbx/internal/compat"
	"gopbx/internal/config"
)

type dumpEntry struct {
	Type      string `json:"type"`
	Timestamp uint64 `json:"timestamp"`
	Content   string `json:"content"`
}

func TestDumpEnabledByDefaultWritesCommandAndEventTrace(t *testing.T) {
	dumpRoot := t.TempDir()
	cfg := config.Default()
	cfg.RecorderPath = dumpRoot
	_, server := newIntegrationServerWithConfig(t, cfg)

	conn := dialCallWS(t, server.URL, compat.RouteCall, "dump-default")
	requireEventName(t, sendInviteAndReadAnswer(t, conn), compat.EventAnswer)

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)

	dumpFile := filepath.Join(dumpRoot, "dump-default.events.jsonl")
	eventually(t, time.Second, func() bool {
		_, err := os.Stat(dumpFile)
		return err == nil
	}, "expected dump file to be created when dump is enabled")

	entries := readDumpEntries(t, dumpFile)
	if len(entries) < 4 {
		t.Fatalf("expected at least 4 dump entries, got %d", len(entries))
	}
	if entries[0].Type != "command" || !strings.Contains(entries[0].Content, `"command":"invite"`) {
		t.Fatalf("unexpected first dump entry: %+v", entries[0])
	}
	if entries[1].Type != "event" || !strings.Contains(entries[1].Content, `"event":"answer"`) {
		t.Fatalf("unexpected answer dump entry: %+v", entries[1])
	}
	if entries[2].Type != "command" || !strings.Contains(entries[2].Content, `"command":"hangup"`) {
		t.Fatalf("unexpected hangup command dump entry: %+v", entries[2])
	}
	if entries[3].Type != "event" || !strings.Contains(entries[3].Content, `"event":"hangup"`) {
		t.Fatalf("unexpected hangup event dump entry: %+v", entries[3])
	}
}

func TestDumpFalseDisablesTraceFile(t *testing.T) {
	dumpRoot := t.TempDir()
	cfg := config.Default()
	cfg.RecorderPath = dumpRoot
	_, server := newIntegrationServerWithConfig(t, cfg)

	conn := dialCallWS(t, server.URL, compat.RouteCall+"?dump=false", "dump-disabled")
	requireEventName(t, sendInviteAndReadAnswer(t, conn), compat.EventAnswer)

	if err := conn.WriteJSON(map[string]any{"command": "hangup"}); err != nil {
		t.Fatalf("send hangup: %v", err)
	}
	requireEventName(t, readEvent(t, conn), compat.EventHangup)
	expectClose(t, conn)

	dumpFile := filepath.Join(dumpRoot, "dump-disabled.events.jsonl")
	if _, err := os.Stat(dumpFile); !os.IsNotExist(err) {
		t.Fatalf("expected no dump file when dump=false, stat err=%v", err)
	}
}

func readDumpEntries(t *testing.T, path string) []dumpEntry {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open dump file: %v", err)
	}
	defer file.Close()

	entries := make([]dumpEntry, 0, 8)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry dumpEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("decode dump entry: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan dump file: %v", err)
	}
	return entries
}
