package user

import (
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/user"
)

type SysUser struct {
	user.SysUser

	// 扩展
	LoginIp string `json:"login_ip"` // 登录Ip
}
