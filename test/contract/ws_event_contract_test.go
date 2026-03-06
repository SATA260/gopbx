// 这个文件是 WS 事件合同测试，校验事件输出字段与命名是否严格保持既有协议兼容。

package contract_test

import (
	"testing"

	wsinbound "gopbx/internal/adapter/inbound/ws"
	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"
)

func TestMarshalAnswerEvent(t *testing.T) {
	data, err := wsinbound.MarshalEvent(wsproto.EventEnvelope{
		Event:     compat.EventAnswer,
		TrackID:   "session-1",
		Timestamp: 1711111111111,
		SDP:       "v=0",
	})
	if err != nil {
		t.Fatalf("marshal answer event: %v", err)
	}
	assertJSONEq(t, mustReadFixture(t, "answer.json"), data)
}

func TestMarshalMetricsEvent(t *testing.T) {
	data, err := wsinbound.MarshalEvent(wsproto.EventEnvelope{
		Event:     compat.EventMetrics,
		Timestamp: 1711111113333,
		Key:       "ttfb.tts.mock",
		Duration:  wsproto.Uint64(0),
		Data: map[string]any{
			"playId": "p1",
			"length": 0,
		},
	})
	if err != nil {
		t.Fatalf("marshal metrics event: %v", err)
	}
	assertJSONEq(t, mustReadFixture(t, "metrics.json"), data)
}

func TestMarshalAsrFinalEvent(t *testing.T) {
	data, err := wsinbound.MarshalEvent(wsproto.EventEnvelope{
		Event:     compat.EventASRFinal,
		TrackID:   "session-1",
		Timestamp: 1711111112222,
		Index:     wsproto.Uint32(3),
		StartTime: wsproto.Int64(1711111111000),
		EndTime:   wsproto.Int64(1711111112200),
		Text:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal asrFinal event: %v", err)
	}
	assertJSONEq(t, mustReadFixture(t, "asr_final.json"), data)
}

func TestMarshalErrorEvent(t *testing.T) {
	data, err := wsinbound.MarshalEvent(wsproto.EventEnvelope{
		Event:     compat.EventError,
		TrackID:   "session-1",
		Timestamp: 1711111114444,
		Sender:    "handle_call",
		Error:     "the first message must be an invite",
	})
	if err != nil {
		t.Fatalf("marshal error event: %v", err)
	}
	assertJSONEq(t, mustReadFixture(t, "error.json"), data)
}

func TestMarshalTrackEndEvent(t *testing.T) {
	data, err := wsinbound.MarshalEvent(wsproto.EventEnvelope{
		Event:     compat.EventTrackEnd,
		TrackID:   "session-1",
		Timestamp: 1711111115555,
		Duration:  wsproto.Uint64(0),
	})
	if err != nil {
		t.Fatalf("marshal trackEnd event: %v", err)
	}
	assertJSONEq(t, mustReadFixture(t, "track_end.json"), data)
}
