package subscribe

import (
	serviceUser "github.com/jettjia/xiaoqinglong/agent-frame/application/service/user"
)

type Subscribe struct {
	SysUserSrv *serviceUser.SysUserService
}

func NewSubscribe() *Subscribe {
	return &Subscribe{
		SysUserSrv: serviceUser.NewSysUserService(),
	}
}
