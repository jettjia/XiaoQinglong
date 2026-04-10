package user

import (
	"encoding/json"

	entityUser "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/user"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/ievent"
)

var (
	UserTopic = "xtext.user.create"
)

// 案例：发布创建用户事件
func PublishCreateUser(sysUserEn *entityUser.SysUser) (err error) {
	cli := ievent.NewEventCli()

	byteData, err := json.Marshal(sysUserEn)
	if err != nil {
		return
	}

	return cli.Pub(UserTopic, byteData)
}
