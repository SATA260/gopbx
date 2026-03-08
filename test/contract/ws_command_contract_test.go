// 这个文件是 WS 命令合同测试，校验首包约束和既有命令字段结构是否保持稳定。

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
	if err := wsinbound.ValidateFirstCommand(&wsproto.CommandEnvelope{Command: wsproto.CommandAccept}); err != nil {
		t.Fatalf("accept should be allowed: %v", err)
	}
	if err := wsinbound.ValidateFirstCommand(&wsproto.CommandEnvelope{Command: wsproto.CommandTTS}); err == nil {
		t.Fatal("tts should not be allowed as first command")
	} else if err.Error() != "the first message must be an invite" {
		t.Fatalf("unexpected first-command error: %v", err)
	}
}

func TestDecodeInviteCommandFixture(t *testing.T) {
	cmd, err := wsinbound.DecodeCommand(mustReadFixture(t, "invite.json"))
	if err != nil {
		t.Fatalf("decode invite fixture: %v", err)
	}
	if cmd.Command != wsproto.CommandInvite {
		t.Fatalf("unexpected command: %s", cmd.Command)
	}
	option, err := cmd.CallOption()
	if err != nil {
		t.Fatalf("decode invite option: %v", err)
	}
	if option == nil || option.Offer == nil || *option.Offer != "v=0" {
		t.Fatalf("unexpected invite option: %+v", option)
	}
	if option.ASR == nil || option.ASR.Provider == nil || *option.ASR.Provider != "aliyun" {
		t.Fatalf("unexpected asr option: %+v", option.ASR)
	}
	if option.TTS == nil || option.TTS.Provider == nil || *option.TTS.Provider != "aliyun" {
		t.Fatalf("unexpected tts option: %+v", option.TTS)
	}
	if option.Codec == nil || *option.Codec != "pcmu" {
		t.Fatalf("unexpected codec: %+v", option.Codec)
	}
}

func TestDecodeTTSCommandFixture(t *testing.T) {
	cmd, err := wsinbound.DecodeCommand(mustReadFixture(t, "tts.json"))
	if err != nil {
		t.Fatalf("decode tts fixture: %v", err)
	}
	if cmd.Command != wsproto.CommandTTS {
		t.Fatalf("unexpected command: %s", cmd.Command)
	}
	if cmd.PlayID == nil || *cmd.PlayID != "p1" {
		t.Fatalf("unexpected playId: %+v", cmd.PlayID)
	}
	if cmd.EndOfStream == nil || !*cmd.EndOfStream {
		t.Fatalf("unexpected endOfStream: %+v", cmd.EndOfStream)
	}
	option, err := cmd.TTSOption()
	if err != nil {
		t.Fatalf("decode tts option: %v", err)
	}
	if option == nil || option.Provider == nil || *option.Provider != "aliyun" {
		t.Fatalf("unexpected tts option: %+v", option)
	}
}
