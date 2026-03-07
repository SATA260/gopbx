// 这个文件根据运行配置选择话单 writer，让本地、HTTP 和 S3 兼容归档后端可以统一装配。

package storage

import (
	"net/http"
	"time"

	"gopbx/internal/app/callrecord"
	"gopbx/internal/config"
)

func NewCallRecordWriter(cfg *config.Config) callrecord.Writer {
	if cfg == nil {
		return LocalWriter{}
	}
	timeout, err := time.ParseDuration(cfg.CallRecord.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	switch cfg.CallRecord.Type {
	case "http":
		return HTTPWriter{Endpoint: cfg.CallRecord.Endpoint, APIKey: cfg.CallRecord.APIKey, Client: client}
	case "s3":
		return S3Writer{
			Endpoint: cfg.CallRecord.Endpoint,
			Bucket:   cfg.CallRecord.Bucket,
			Prefix:   cfg.CallRecord.Prefix,
			APIKey:   cfg.CallRecord.APIKey,
			Client:   client,
		}
	default:
		return LocalWriter{Root: cfg.RecorderPath}
	}
}
