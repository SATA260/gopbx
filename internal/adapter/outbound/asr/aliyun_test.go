// 这个文件验证阿里云实时识别回包解析，确保 typed 结构可以稳定产出 delta/final 事件所需字段。

package asr

import "testing"

func TestParseAliyunResultDelta(t *testing.T) {
	raw := `{"header":{"namespace":"SpeechTranscriber","name":"TranscriptionResultChanged","status":20000000,"message_id":"mid-1","task_id":"task-1","status_text":"Gateway:SUCCESS:Success."},"payload":{"index":1,"time":360,"result":"你好","confidence":0.91,"words":[],"status":1,"gender":"","begin_time":120,"fixed_result":"","unfixed_result":"","stash_result":{"sentenceId":3,"beginTime":120,"text":"你好","fixedText":"","unfixedText":"你好","currentTime":360,"words":[]},"audio_extra_info":"","sentence_id":"sentence-1","gender_score":0.0}}`
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
	raw := `{"header":{"namespace":"SpeechTranscriber","name":"SentenceEnd","status":20000000,"message_id":"mid-2","task_id":"task-1","status_text":"Gateway:SUCCESS:Success."},"payload":{"index":1,"time":1800,"result":"今天北京天气很好","confidence":0.97,"words":[],"status":0,"gender":"","begin_time":500,"fixed_result":"","unfixed_result":"","stash_result":{"sentenceId":9,"beginTime":500,"text":"","fixedText":"","unfixedText":"","currentTime":1800,"words":[]},"audio_extra_info":"","sentence_id":"sentence-9","gender_score":0.0}}`
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
		`{"header":{"namespace":"SpeechTranscriber","name":"SentenceEnd","status":40000000,"message_id":"mid-3","task_id":"task-1","status_text":"Gateway:FAILED:Failed."}}`,
		`{"header":{"namespace":"SpeechTranscriber","name":"SentenceEnd","status":20000000,"message_id":"mid-4","task_id":"task-1","status_text":"Gateway:SUCCESS:Success."},"payload":{"index":1,"time":0,"begin_time":0}}`,
	}
	for _, raw := range tests {
		if _, ok := parseAliyunResult(raw, false); ok {
			t.Fatalf("expected payload to be rejected: %s", raw)
		}
	}
}
