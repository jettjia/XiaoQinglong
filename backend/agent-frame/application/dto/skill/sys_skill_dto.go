package skill

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// 请求对象
type (
	// CreateSysSkillReq 创建请求对象
	CreateSysSkillReq struct {
		CreatedBy   string `json:"created_by"`
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
		SkillType   string `json:"skillType" validate:"required"` // skill/mcp/tool/a2a
		Version     string `json:"version"`
		Path        string `json:"path"`
		Enabled     bool   `json:"enabled"`
		Config      string `json:"config"`
		IsSystem    bool   `json:"is_system"`
		RiskLevel   string `json:"risk_level"` // low/medium/high
	}

	// DelSysSkillReq 删除请求对象
	DelSysSkillReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// UpdateSysSkillReq 修改请求对象
	UpdateSysSkillReq struct {
		Ulid        string `validate:"required" uri:"ulid" json:"ulid"`
		UpdatedBy   string `json:"updated_by"`
		Name        string `json:"name"`
		Description string `json:"description"`
		SkillType   string `json:"skillType"`
		Version     string `json:"version"`
		Path        string `json:"path"`
		Enabled     *bool  `json:"enabled"`
		Config      string `json:"config"`
		RiskLevel   string `json:"risk_level"` // low/medium/high
	}

	// FindSysSkillByIdReq 查询请求对象
	FindSysSkillByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindSysSkillAllReq 查询请求对象
	FindSysSkillAllReq struct {
		SkillType string `json:"skill_type"`
		Name      string `json:"name"`
	}

	// FindSysSkillPageReq 分页查询请求对象
	FindSysSkillPageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}

	// CheckSkillNameReq 检查名称请求对象
	CheckSkillNameReq struct {
		Name string `json:"name" validate:"required"`
	}

	// UploadSysSkillReq 上传Skill请求对象
	UploadSysSkillReq struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	}
)

// 输出对象
type (
	// CreateSysSkillRsp 创建返回对象
	CreateSysSkillRsp struct {
		Ulid string `json:"ulid"`
	}

	// FindSysSkillRsp 查询返回对象
	FindSysSkillRsp struct {
		Ulid        string `json:"ulid"`
		CreatedAt   int64  `json:"created_at"`
		UpdatedAt   int64  `json:"updated_at"`
		CreatedBy   string `json:"created_by"`
		UpdatedBy   string `json:"updated_by"`
		Name        string `json:"name"`
		Description string `json:"description"`
		SkillType   string `json:"skill_type"`
		Version     string `json:"version"`
		Path        string `json:"path"`
		Enabled     bool   `json:"enabled"`
		Config      string `json:"config"`
		IsSystem    bool   `json:"is_system"`
		RiskLevel   string `json:"risk_level"` // low/medium/high
	}

	// FindSysSkillPageRsp 列表查询返回对象
	FindSysSkillPageRsp struct {
		Entries  []*FindSysSkillRsp `json:"entries"`
		PageData *builder.PageData  `json:"page_data"`
	}

	// CheckSkillNameRsp 检查名称返回对象
	CheckSkillNameRsp struct {
		Exists  bool   `json:"exists"`
		Message string `json:"message"`
	}
)
