// 这个文件是对象存储适配器占位，后续用于上传录音与归档数据到 S3 或 MinIO。

package storage

type S3Writer struct{}

func (S3Writer) Name() string { return "s3" }
