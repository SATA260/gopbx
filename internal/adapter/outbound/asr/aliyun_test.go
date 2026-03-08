// 这个文件验证阿里云实时识别回包解析，确保 typed 结构可以稳定产出 delta/final 事件所需字段。

package asr

import "testing"

func TestParseAliyunResultDelta(t *testing.T) {
	raw := `{"header":{"task_id":"task-1","message":"Success","status":"Success"},"payload":{"result":"你好","sentence_id":3,"is_sentence_end":false,"begin_time":120,"end_time":360}}`
	result, ok := parseAliyunResult(raw, false)
	if !ok {
		t.Fatal("expected delta result to parse")
	}
	if result.Final {
		t.Fatal("expected delta result to stay non-final")
	}
	if result.Text != "你好" {
		t.Fatalf("unexpected delta text: %s", result.Text)
	}
	if result.StartTime != 120 || result.EndTime != 360 {
		t.Fatalf("unexpected delta timing: %+v", result)
	}
}

func TestParseAliyunResultFinal(t *testing.T) {
	raw := `{"header":{"task_id":"task-1","message":"Success","status":"Success"},"payload":{"result":"今天北京天气很好","sentence_id":9,"is_sentence_end":true,"begin_time":500,"end_time":1800}}`
	result, ok := parseAliyunResult(raw, false)
	if !ok {
		t.Fatal("expected final result to parse")
	}
	if !result.Final {
		t.Fatal("expected final result to be marked final")
	}
	if result.Text != "今天北京天气很好" {
		t.Fatalf("unexpected final text: %s", result.Text)
	}
	if result.StartTime != 500 || result.EndTime != 1800 {
		t.Fatalf("unexpected final timing: %+v", result)
	}
}

func TestParseAliyunResultRejectsInvalidPayload(t *testing.T) {
	tests := []string{
		``,
		`not-json`,
		`{"header":{"task_id":"task-1","message":"BadRequest","status":"Failed"}}`,
		`{"header":{"task_id":"task-1","message":"Success","status":"Success"},"payload":{"sentence_id":1,"is_sentence_end":false,"begin_time":0,"end_time":0}}`,
	}
	for _, raw := range tests {
		if _, ok := parseAliyunResult(raw, false); ok {
			t.Fatalf("expected payload to be rejected: %s", raw)
		}
	}
}
