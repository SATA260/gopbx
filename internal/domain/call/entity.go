// 这个文件定义通话领域实体，表达业务视角下的会话基础属性。

package call

import "time"

type Entity struct {
	ID        string
	Type      string
	CreatedAt time.Time
}
