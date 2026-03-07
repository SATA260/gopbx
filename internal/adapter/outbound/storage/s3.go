// 这个文件实现对象存储适配器占位，当前先保留统一接口，后续可把话单和录音上传到 S3/MinIO。

package storage

import (
	"errors"

	"gopbx/internal/app/callrecord"
)

type S3Writer struct{}

func (S3Writer) Name() string { return "s3" }

func (S3Writer) Write(callrecord.Record) error {
	return errors.New("s3 callrecord writer is not implemented")
}
