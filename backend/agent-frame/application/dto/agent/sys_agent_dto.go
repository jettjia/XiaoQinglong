package agent

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// 请求对象
type (
	// CreateSysAgentReq 创建请求对象
	CreateSysAgentReq struct {
		CreatedBy   string `json:"created_by"`
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Model       string `json:"model"`
		Config      string `json:"config"`
		ConfigJson  string `json:"config_json"`
		Enabled     bool   `json:"enabled"`
		IsSystem    bool   `json:"is_system"`
		Channels    string `json:"channels"`
		IsPeriodic  bool   `json:"is_periodic"`
		CronRule    string `json:"cron_rule"`
		Sort        int    `json:"sort"`
	}

	// DelSysAgentReq 删除请求对象
	DelSysAgentReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// UpdateSysAgentReq 修改请求对象
	UpdateSysAgentReq struct {
		Ulid        string `validate:"required" uri:"ulid" json:"ulid"`
		UpdatedBy   string `json:"updated_by"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Model       string `json:"model"`
		Config      string `json:"config"`
		ConfigJson  string `json:"config_json"`
		Enabled     *bool  `json:"enabled"`
		Channels    string `json:"channels"`
		IsPeriodic  *bool  `json:"is_periodic"`
		CronRule    string `json:"cron_rule"`
		Sort        *int   `json:"sort"`
	}

	// UpdateSysAgentEnabledReq 修改启用状态请求对象
	UpdateSysAgentEnabledReq struct {
		Ulid    string `validate:"required" json:"ulid"`
		Enabled bool   `json:"enabled"`
	}

	// FindSysAgentByIdReq 查询请求对象
	FindSysAgentByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindSysAgentAllReq 查询请求对象
	FindSysAgentAllReq struct {
		Name string `json:"name"`
	}

	// FindSysAgentPageReq 分页查询请求对象
	FindSysAgentPageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}

	// UploadSysAgentReq 上传Agent请求对象
	UploadSysAgentReq struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Model       string `json:"model"`
		Config      string `json:"config"`
		Enabled     bool   `json:"enabled"`
	}
)

// 输出对象
type (
	// CreateSysAgentRsp 创建返回对象
	CreateSysAgentRsp struct {
		Ulid string `json:"ulid"`
	}

	// FindSysAgentRsp 查询返回对象
	FindSysAgentRsp struct {
		Ulid        string `json:"ulid"`
		CreatedAt   int64  `json:"created_at"`
		UpdatedAt   int64  `json:"updated_at"`
		CreatedBy   string `json:"created_by"`
		UpdatedBy   string `json:"updated_by"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Model       string `json:"model"`
		Config      string `json:"config"`
		ConfigJson  string `json:"config_json"`
		IsSystem    bool   `json:"is_system"`
		Enabled     bool   `json:"enabled"`
		Channels    string `json:"channels"`
		IsPeriodic  bool   `json:"is_periodic"`
		CronRule    string `json:"cron_rule"`
		Sort        int    `json:"sort"`
	}

	// FindSysAgentPageRsp 列表查询返回对象
	FindSysAgentPageRsp struct {
		Entries  []*FindSysAgentRsp    `json:"entries"`
		PageData *builder.PageData `json:"page_data"`
	}
)
