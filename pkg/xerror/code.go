// 这个文件定义通用错误码与错误结构，方便协议层输出一致的错误语义。

package xerror

type Code string

const (
	CodeInvalidCommand Code = "invalid_command"
	CodeUpgradeFailed  Code = "upgrade_failed"
	CodeInternal       Code = "internal_error"
)

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	return string(e.Code) + ": " + e.Message
}
