package private

import (
	serviceUser "github.com/jettjia/xiaoqinglong/agent-frame/application/service/user"
)

type PrivateHandler struct {
	SysUserSrv *serviceUser.SysUserService
}

func NewPrivateHandler() *PrivateHandler {
	return &PrivateHandler{
		SysUserSrv: serviceUser.NewSysUserService(),
	}
}
