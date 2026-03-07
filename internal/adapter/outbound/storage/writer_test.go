// 这个文件验证话单存储适配器，确保 local/http/s3 三种 writer 都能按预期输出请求或文件。

package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopbx/internal/app/callrecord"
	"gopbx/internal/config"
)

func TestLocalWriterWrite(t *testing.T) {
	root := t.TempDir()
	writer := LocalWriter{Root: root}
	record := callrecord.Record{CallID: "local-1", CallType: "websocket"}
	if err := writer.Write(record); err != nil {
		t.Fatalf("write local callrecord: %v", err)
	}
	path := filepath.Join(root, "local-1.call.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat local callrecord file: %v", err)
	}
}

func TestHTTPWriterWrite(t *testing.T) {
	var seenAuth string
	var seenRecord callrecord.Record
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenRecord); err != nil {
			t.Fatalf("decode http writer body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	writer := HTTPWriter{Endpoint: server.URL, APIKey: "http-key", Client: server.Client()}
	record := callrecord.Record{CallID: "http-1", CallType: "websocket"}
	if err := writer.Write(record); err != nil {
		t.Fatalf("write http callrecord: %v", err)
	}
	if seenAuth != "Bearer http-key" {
		t.Fatalf("unexpected http auth header: %s", seenAuth)
	}
	if seenRecord.CallID != "http-1" {
		t.Fatalf("unexpected http record: %+v", seenRecord)
	}
}

func TestS3WriterWrite(t *testing.T) {
	var seenPath string
	var seenMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	writer := S3Writer{Endpoint: server.URL, Bucket: "bucket-a", Prefix: "calls", Client: server.Client()}
	record := callrecord.Record{CallID: "s3-1", CallType: "webrtc"}
	if err := writer.Write(record); err != nil {
		t.Fatalf("write s3 callrecord: %v", err)
	}
	if seenMethod != http.MethodPut {
		t.Fatalf("unexpected s3 method: %s", seenMethod)
	}
	if seenPath != "/bucket-a/calls/s3-1.call.json" {
		t.Fatalf("unexpected s3 path: %s", seenPath)
	}
}

func TestNewCallRecordWriter(t *testing.T) {
	cfg := config.Default()
	if _, ok := NewCallRecordWriter(cfg).(LocalWriter); !ok {
		t.Fatal("expected default writer to be local")
	}
	cfg.CallRecord.Type = "http"
	if _, ok := NewCallRecordWriter(cfg).(HTTPWriter); !ok {
		t.Fatal("expected http writer from config")
	}
	cfg.CallRecord.Type = "s3"
	if _, ok := NewCallRecordWriter(cfg).(S3Writer); !ok {
		t.Fatal("expected s3 writer from config")
	}
}
