// 这个文件是 WS 命令合同测试，校验首包命令约束是否保持稳定。

package contract_test

import (
	"testing"

	wsinbound "gopbx/internal/adapter/inbound/ws"
	"gopbx/pkg/wsproto"
)

func TestValidateFirstCommand(t *testing.T) {
	if err := wsinbound.ValidateFirstCommand(&wsproto.CommandEnvelope{Command: wsproto.CommandInvite}); err != nil {
		t.Fatalf("invite should be allowed: %v", err)
	}

	if err := wsinbound.ValidateFirstCommand(&wsproto.CommandEnvelope{Command: wsproto.CommandTTS}); err == nil {
		t.Fatal("tts should not be allowed as first command")
	}
}
