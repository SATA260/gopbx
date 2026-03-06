// 这个文件定义通话仓储接口，占位抽象会话持久化能力。

package call

type Repository interface {
	Save(Entity) error
	Delete(id string) error
}
