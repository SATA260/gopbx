// 这个文件定义话单写入接口，约束不同存储后端的统一写入能力。

package callrecord

type Writer interface {
	Write(Record) error
}
