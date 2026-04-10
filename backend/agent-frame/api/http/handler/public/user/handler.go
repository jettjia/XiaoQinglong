package user

import (
	serviceUser "github.com/jettjia/xiaoqinglong/agent-frame/application/service/user"
)

type Handler struct {
	SysUserSrv *serviceUser.SysUserService
}

func NewHandler() *Handler {
	return &Handler{
		SysUserSrv: serviceUser.NewSysUserService(),
	}
}
