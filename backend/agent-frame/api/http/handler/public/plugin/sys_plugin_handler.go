package plugin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dtoPlugin "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/plugin"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// GetPlugins 获取插件列表
// GET /plugin/list
func (h *Handler) GetPlugins(c *gin.Context) {
	// 业务处理
	res, err := h.SysPluginSrv.GetPlugins(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, res)
}

// GetUserInstances 获取用户插件实例
// GET /plugin/instances
func (h *Handler) GetUserInstances(c *gin.Context) {
	// 业务处理
	res, err := h.SysPluginSrv.GetUserInstances(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, res)
}

// GetInstanceById 获取实例详情
// GET /plugin/instance/:ulid
func (h *Handler) GetInstanceById(c *gin.Context) {
	// 参数解析
	dtoReq := dtoPlugin.GetInstanceByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 参数过滤
	err = validate.Validate(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 业务处理
	rsp, err := h.SysPluginSrv.GetInstanceById(c, dtoReq.Ulid)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// DeleteInstance 删除插件实例
// DELETE /plugin/instance/:ulid
func (h *Handler) DeleteInstance(c *gin.Context) {
	// 参数解析
	dtoReq := dtoPlugin.DeleteInstanceReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 参数过滤
	err = validate.Validate(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	deletedBy := c.GetString("user_id")

	// 业务处理
	err = h.SysPluginSrv.DeleteInstance(c, &dtoReq, deletedBy)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// RefreshToken 刷新令牌
// POST /plugin/instance/:ulid/refresh
func (h *Handler) RefreshToken(c *gin.Context) {
	// 参数解析
	dtoReq := dtoPlugin.RefreshTokenReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 参数过滤
	err = validate.Validate(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 业务处理
	rsp, err := h.SysPluginSrv.RefreshToken(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// StartAuth 开始授权
// POST /plugin/auth/start
func (h *Handler) StartAuth(c *gin.Context) {
	// 参数解析
	dtoReq := dtoPlugin.StartAuthReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 参数过滤
	err = validate.Validate(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 业务处理
	res, err := h.SysPluginSrv.StartAuth(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, res)
}

// PollAuth 轮询授权状态
// POST /plugin/auth/poll
func (h *Handler) PollAuth(c *gin.Context) {
	// 参数解析
	dtoReq := dtoPlugin.PollAuthReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 参数过滤
	err = validate.Validate(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 业务处理
	res, err := h.SysPluginSrv.PollAuth(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, res)
}

// GetAuthUrl 获取授权URL
// GET /plugin/auth/url
func (h *Handler) GetAuthUrl(c *gin.Context) {
	// 参数解析
	dtoReq := dtoPlugin.StartAuthReq{}
	err := c.ShouldBindQuery(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	// 业务处理
	res, err := h.SysPluginSrv.StartAuth(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, res)
}

// GetPublicKey 获取RSA公钥
// GET /plugin/public-key
func (h *Handler) GetPublicKey(c *gin.Context) {
	// 业务处理
	res, err := h.SysPluginSrv.GetPublicKey(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, res)
}