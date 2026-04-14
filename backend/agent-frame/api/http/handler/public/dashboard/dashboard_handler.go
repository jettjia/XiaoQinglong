package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/dashboard"
	svc "github.com/jettjia/xiaoqinglong/agent-frame/application/service/dashboard"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

type Handler struct {
	dashboardSvc *svc.DashboardSvc
}

func NewHandler() *Handler {
	return &Handler{
		dashboardSvc: svc.NewDashboardSvc(),
	}
}

// GetOverview 获取Dashboard概览
func (h *Handler) GetOverview(c *gin.Context) {
	rsp, err := h.dashboardSvc.GetOverview(c, &dto.DashboardOverviewReq{})
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// GetTokenRanking 获取Token使用排行
func (h *Handler) GetTokenRanking(c *gin.Context) {
	dtoReq := dto.TokenUsageRankingReq{}
	if err := c.ShouldBindQuery(&dtoReq); err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	if dtoReq.Limit <= 0 {
		dtoReq.Limit = 10
	}

	rsp, err := h.dashboardSvc.GetTokenUsageRanking(c, &dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// GetAgentRanking 获取智能体使用排行
func (h *Handler) GetAgentRanking(c *gin.Context) {
	dtoReq := dto.AgentUsageRankingReq{}
	if err := c.ShouldBindQuery(&dtoReq); err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	if dtoReq.Limit <= 0 {
		dtoReq.Limit = 10
	}

	rsp, err := h.dashboardSvc.GetAgentUsageRanking(c, &dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// GetChannelActivity 获取渠道活动统计
func (h *Handler) GetChannelActivity(c *gin.Context) {
	rsp, err := h.dashboardSvc.GetChannelActivity(c, &dto.ChannelActivityReq{})
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// GetRecentSessions 获取最近会话
func (h *Handler) GetRecentSessions(c *gin.Context) {
	dtoReq := dto.GetRecentSessionsReq{}
	if err := c.ShouldBindQuery(&dtoReq); err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.dashboardSvc.GetRecentSessions(c, &dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.InternalServerErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}
