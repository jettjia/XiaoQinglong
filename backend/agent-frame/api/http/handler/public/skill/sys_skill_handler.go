package skill

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/igo-pkg/pkg/validate"
	"github.com/jettjia/igo-pkg/pkg/xerror"
	"github.com/jettjia/igo-pkg/pkg/xresponse"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/types/apierror"
)

// CreateSysSkill 创建 Skill
func (h *Handler) CreateSysSkill(c *gin.Context) {
	dtoReq := dto.CreateSysSkillReq{}
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

	res, err := h.SysSkillSrv.CreateSysSkill(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, res)
}

// DeleteSysSkill 删除 Skill
func (h *Handler) DeleteSysSkill(c *gin.Context) {
	dtoReq := dto.DelSysSkillReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	err = h.SysSkillSrv.DeleteSysSkill(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// UpdateSysSkill 更新 Skill
func (h *Handler) UpdateSysSkill(c *gin.Context) {
	dtoReq := dto.UpdateSysSkillReq{}
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

	err = h.SysSkillSrv.UpdateSysSkill(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusNoContent, nil)
}

// FindSysSkillById 查询 Skill
func (h *Handler) FindSysSkillById(c *gin.Context) {
	dtoReq := dto.FindSysSkillByIdReq{}
	err := c.ShouldBindUri(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysSkillSrv.FindSysSkillById(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysSkillAll 查询所有 Skill
func (h *Handler) FindSysSkillAll(c *gin.Context) {
	dtoReq := dto.FindSysSkillAllReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		// 如果没有body，使用空的req
		dtoReq = dto.FindSysSkillAllReq{}
	}

	rsp, err := h.SysSkillSrv.FindSysSkillAll(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// FindSysSkillPage 分页查询 Skill
func (h *Handler) FindSysSkillPage(c *gin.Context) {
	dtoReq := dto.FindSysSkillPageReq{}
	err := c.BindJSON(&dtoReq)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause(err.Error()))
		_ = c.Error(err)
		return
	}

	rsp, err := h.SysSkillSrv.FindSysSkillPage(c, &dtoReq)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// CheckSkillName 检查同名 Skill
func (h *Handler) CheckSkillName(c *gin.Context) {
	dtoReq := dto.CheckSkillNameReq{}
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

	rsp, err := h.SysSkillSrv.CheckSkillName(c, dtoReq.Name)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusOK, rsp)
}

// UploadSysSkill 上传并安装 Skill
func (h *Handler) UploadSysSkill(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause("failed to get uploaded file"))
		_ = c.Error(err)
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		err = xerror.NewErrorOpt(apierror.BadRequestErr, xerror.WithCause("failed to read uploaded file"))
		_ = c.Error(err)
		return
	}

	createdBy := c.GetString("user_id")

	rsp, err := h.SysSkillSrv.UploadSysSkill(c, fileData, header.Filename, createdBy)
	if err != nil {
		_ = c.Error(err)
		return
	}

	xresponse.RspOk(c, http.StatusCreated, rsp)
}
