// 这个文件实现 HTTP 归档适配器，用来把话单 JSON 直接投递到外部归档服务。

package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopbx/internal/app/callrecord"
)

type HTTPWriter struct {
	Endpoint string
	APIKey   string
	Client   *http.Client
}

func (HTTPWriter) Name() string { return "http" }

func (w HTTPWriter) Write(record callrecord.Record) error {
	if w.Endpoint == "" {
		return fmt.Errorf("http callrecord endpoint is not configured")
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(w.Endpoint, "/"), bytes.NewReader(body))
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
		return fmt.Errorf("http callrecord writer status: %d", resp.StatusCode)
	}
	return nil
}
