// 这个文件定义编解码类型，统一媒体层可支持的音频编码标识。

package codec

type Type string

const (
	PCMU Type = "PCMU"
	PCMA Type = "PCMA"
	G722 Type = "G722"
)
