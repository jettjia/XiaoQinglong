package model

import (
	serviceModel "github.com/jettjia/xiaoqinglong/agent-frame/application/service/model"
)

type Handler struct {
	SysModelSrv *serviceModel.SysModelService
}

func NewHandler() *Handler {
	return &Handler{
		SysModelSrv: serviceModel.NewSysModelService(),
	}
}