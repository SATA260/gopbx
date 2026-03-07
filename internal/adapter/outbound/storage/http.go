// 这个文件实现 HTTP 存储适配器占位，当前先保留统一接口，后续可把话单上报给外部归档服务。

package storage

import (
	"errors"

	"gopbx/internal/app/callrecord"
)

type HTTPWriter struct{}

func (HTTPWriter) Name() string { return "http" }

func (HTTPWriter) Write(callrecord.Record) error {
	return errors.New("http callrecord writer is not implemented")
}
