package job

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	jobdto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/job"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/job"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// Handler job handler
type Handler struct {
	JobExecutionSrv *job.JobExecutionService
}

// NewHandler NewHandler
func NewHandler() *Handler {
	return &Handler{
		JobExecutionSrv: job.NewJobExecutionService(),
	}
}

// FindJobExecutionById 查询
func (h *Handler) FindJobExecutionById(c *gin.Context) {
	dtoReq := jobdto.FindJobExecutionByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
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

	rsp, err := h.JobExecutionSrv.FindJobExecutionById(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindJobExecutionByAgentId 根据AgentId查询
func (h *Handler) FindJobExecutionByAgentId(c *gin.Context) {
	dtoReq := jobdto.FindJobExecutionByAgentIdReq{}
	err := c.ShouldBind(&dtoReq)
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

	rsp, err := h.JobExecutionSrv.FindJobExecutionByAgentId(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindJobExecutionPage 分页查询
func (h *Handler) FindJobExecutionPage(c *gin.Context) {
	dtoReq := jobdto.FindJobExecutionPageReq{}
	err := c.ShouldBind(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.JobExecutionSrv.FindJobExecutionPage(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}
