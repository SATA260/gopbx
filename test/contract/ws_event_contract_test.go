// 这个文件是 WS 事件合同测试，校验事件输出字段与命名是否保持稳定。

package contract_test

import (
	"strings"
	"testing"

	wsinbound "gopbx/internal/adapter/inbound/ws"
	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

func TestMarshalEvent(t *testing.T) {
	data, err := wsinbound.MarshalEvent(wsproto.EventEnvelope{Event: compat.EventAnswer, TrackID: "abc"})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "\"event\":\"answer\"") {
		t.Fatalf("unexpected event payload: %s", text)
	}
	if !strings.Contains(text, "\"trackId\":\"abc\"") {
		t.Fatalf("unexpected track id payload: %s", text)
	}
}
