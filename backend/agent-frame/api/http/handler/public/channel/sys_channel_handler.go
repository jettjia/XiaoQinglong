package channel

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// CreateSysChannel 创建 Channel
func (h *Handler) CreateSysChannel(c *gin.Context) {
	dtoReq := dto.CreateSysChannelReq{}
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

	res, err := h.SysChannelSrv.CreateSysChannel(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, res)
}

// DeleteSysChannel 删除 Channel
func (h *Handler) DeleteSysChannel(c *gin.Context) {
	dtoReq := dto.DelSysChannelReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = h.SysChannelSrv.DeleteSysChannel(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// UpdateSysChannel 更新 Channel
func (h *Handler) UpdateSysChannel(c *gin.Context) {
	dtoReq := dto.UpdateSysChannelReq{}
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

	err = h.SysChannelSrv.UpdateSysChannel(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// FindSysChannelById 查询 Channel
func (h *Handler) FindSysChannelById(c *gin.Context) {
	dtoReq := dto.FindSysChannelByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysChannelSrv.FindSysChannelById(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysChannelAll 查询所有 Channel
func (h *Handler) FindSysChannelAll(c *gin.Context) {
	dtoReq := dto.FindSysChannelAllReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		// 如果没有body，使用空的req
		dtoReq = dto.FindSysChannelAllReq{}
	}

	rsp, err := h.SysChannelSrv.FindSysChannelAll(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysChannelPage 分页查询 Channel
func (h *Handler) FindSysChannelPage(c *gin.Context) {
	dtoReq := dto.FindSysChannelPageReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysChannelSrv.FindSysChannelPage(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}