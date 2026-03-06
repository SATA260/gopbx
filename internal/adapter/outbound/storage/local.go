// 这个文件是本地存储适配器占位，后续用于落地录音和话单文件。

package storage

type LocalWriter struct{}

func (LocalWriter) Name() string { return "local" }
