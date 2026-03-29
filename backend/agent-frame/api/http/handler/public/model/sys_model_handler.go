package model

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dtoModel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// CreateSysModel 创建模型
func (h *Handler) CreateSysModel(c *gin.Context) {
	dtoReq := dtoModel.CreateSysModelReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = validate.Validate(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}
	dtoReq.CreatedBy = c.GetString("user_id")

	res, err := h.SysModelSrv.CreateSysModel(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, res)
}

// DeleteSysModel 删除模型
func (h *Handler) DeleteSysModel(c *gin.Context) {
	dtoReq := dtoModel.DelSysModelReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = h.SysModelSrv.DeleteSysModel(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// UpdateSysModel 更新模型
func (h *Handler) UpdateSysModel(c *gin.Context) {
	dtoReq := dtoModel.UpdateSysModelReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}
	err = c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = h.SysModelSrv.UpdateSysModel(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// FindSysModelById 查询模型
func (h *Handler) FindSysModelById(c *gin.Context) {
	dtoReq := dtoModel.FindSysModelByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysModelSrv.FindSysModelById(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysModelAll 查询所有模型
func (h *Handler) FindSysModelAll(c *gin.Context) {
	dtoReq := dtoModel.FindSysModelAllReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		// 如果没有body，使用空的req
		dtoReq = dtoModel.FindSysModelAllReq{}
	}

	rsp, err := h.SysModelSrv.FindSysModelAll(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysModelPage 分页查询模型
func (h *Handler) FindSysModelPage(c *gin.Context) {
	dtoReq := dtoModel.FindSysModelPageReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysModelSrv.FindSysModelPage(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}