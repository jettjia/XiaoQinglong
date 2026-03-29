package agent

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// CreateSysAgent 创建 Agent
func (h *Handler) CreateSysAgent(c *gin.Context) {
	dtoReq := dto.CreateSysAgentReq{}
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

	res, err := h.SysAgentSrv.CreateSysAgent(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, res)
}

// DeleteSysAgent 删除 Agent
func (h *Handler) DeleteSysAgent(c *gin.Context) {
	dtoReq := dto.DelSysAgentReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = h.SysAgentSrv.DeleteSysAgent(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// UpdateSysAgent 更新 Agent
func (h *Handler) UpdateSysAgent(c *gin.Context) {
	dtoReq := dto.UpdateSysAgentReq{}
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

	err = h.SysAgentSrv.UpdateSysAgent(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// FindSysAgentById 查询 Agent
func (h *Handler) FindSysAgentById(c *gin.Context) {
	dtoReq := dto.FindSysAgentByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysAgentSrv.FindSysAgentById(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysAgentAll 查询所有 Agent
func (h *Handler) FindSysAgentAll(c *gin.Context) {
	dtoReq := dto.FindSysAgentAllReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		// 如果没有body，使用空的req
		dtoReq = dto.FindSysAgentAllReq{}
	}

	rsp, err := h.SysAgentSrv.FindSysAgentAll(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysAgentPage 分页查询 Agent
func (h *Handler) FindSysAgentPage(c *gin.Context) {
	dtoReq := dto.FindSysAgentPageReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysAgentSrv.FindSysAgentPage(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// UploadSysAgent 上传并导入 Agent JSON 配置
func (h *Handler) UploadSysAgent(c *gin.Context) {
	dtoReq := dto.UploadSysAgentReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	res, err := h.SysAgentSrv.UploadSysAgent(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, res)
}
