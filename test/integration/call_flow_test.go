// 这个文件是通话流程集成测试，占位验证命令路由的基础行为是否可用。

package integration_test

import (
	"testing"

	"gopbx/internal/app/session"
	"gopbx/pkg/wsproto"
)

func TestCommandRouterTTS(t *testing.T) {
	router := session.NewCommandRouter()
	s := session.NewSession("s1", session.TypeWebRTC, nil)
	events := router.Route(s, &wsproto.CommandEnvelope{Command: wsproto.CommandTTS, PlayID: "p1"})
	if len(events) == 0 {
		t.Fatal("expected events for tts command")
	}
}
