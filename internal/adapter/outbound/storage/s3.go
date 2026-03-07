// 这个文件实现 S3 兼容归档适配器，用来把话单 JSON 上传到对象存储的固定 key。

package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"gopbx/internal/app/callrecord"
)

type S3Writer struct {
	Endpoint string
	Bucket   string
	Prefix   string
	APIKey   string
	Client   *http.Client
}

func (S3Writer) Name() string { return "s3" }

func (w S3Writer) Write(record callrecord.Record) error {
	if w.Endpoint == "" || w.Bucket == "" {
		return fmt.Errorf("s3 callrecord writer endpoint or bucket is not configured")
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	key := path.Join(strings.Trim(w.Prefix, "/"), record.CallID+".call.json")
	base, err := url.Parse(strings.TrimRight(w.Endpoint, "/") + "/")
	if err != nil {
		return err
	}
	base.Path = path.Join(base.Path, w.Bucket, key)
	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodPut, base.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("s3 callrecord writer status: %d", resp.StatusCode)
	}
	return nil
}
