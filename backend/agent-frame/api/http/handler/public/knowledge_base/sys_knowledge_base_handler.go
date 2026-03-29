package knowledge_base

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/knowledge_base"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// CreateSysKnowledgeBase 创建知识库
func (h *Handler) CreateSysKnowledgeBase(c *gin.Context) {
	dtoReq := dto.CreateSysKnowledgeBaseReq{}
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

	res, err := h.SysKnowledgeBaseSrv.CreateSysKnowledgeBase(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, res)
}

// DeleteSysKnowledgeBase 删除知识库
func (h *Handler) DeleteSysKnowledgeBase(c *gin.Context) {
	dtoReq := dto.DelSysKnowledgeBaseReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = h.SysKnowledgeBaseSrv.DeleteSysKnowledgeBase(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// UpdateSysKnowledgeBase 更新知识库
func (h *Handler) UpdateSysKnowledgeBase(c *gin.Context) {
	dtoReq := dto.UpdateSysKnowledgeBaseReq{}
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

	err = h.SysKnowledgeBaseSrv.UpdateSysKnowledgeBase(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// FindSysKnowledgeBaseById 查询知识库
func (h *Handler) FindSysKnowledgeBaseById(c *gin.Context) {
	dtoReq := dto.FindSysKnowledgeBaseByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysKnowledgeBaseSrv.FindSysKnowledgeBaseById(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysKnowledgeBaseAll 查询所有知识库
func (h *Handler) FindSysKnowledgeBaseAll(c *gin.Context) {
	dtoReq := dto.FindSysKnowledgeBaseAllReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		// 如果没有body，使用空的req
		dtoReq = dto.FindSysKnowledgeBaseAllReq{}
	}

	rsp, err := h.SysKnowledgeBaseSrv.FindSysKnowledgeBaseAll(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysKnowledgeBasePage 分页查询知识库
func (h *Handler) FindSysKnowledgeBasePage(c *gin.Context) {
	dtoReq := dto.FindSysKnowledgeBasePageReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysKnowledgeBaseSrv.FindSysKnowledgeBasePage(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// RecallTest 召回测试
func (h *Handler) RecallTest(c *gin.Context) {
	dtoReq := dto.FindSysKnowledgeBaseByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	recallReq := dto.RecallTestReq{}
	err = c.BindJSON(&recallReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysKnowledgeBaseSrv.RecallTest(c, dtoReq.Ulid, &recallReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}
