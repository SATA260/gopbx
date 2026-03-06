// 这个文件是 HTTP 存储适配器占位，后续用于把话单或媒体上报给外部服务。

package storage

type HTTPWriter struct{}

func (HTTPWriter) Name() string { return "http" }
