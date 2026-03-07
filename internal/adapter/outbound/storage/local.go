// 这个文件实现本地存储适配器，用来把话单写入磁盘，作为默认归档方式。

package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gopbx/internal/app/callrecord"
)

type LocalWriter struct {
	Root string
}

func (w LocalWriter) Name() string { return "local" }

func (w LocalWriter) Write(record callrecord.Record) error {
	if err := os.MkdirAll(w.Root, 0o755); err != nil {
		return err
	}
	path := filepath.Join(w.Root, record.CallID+".call.json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
