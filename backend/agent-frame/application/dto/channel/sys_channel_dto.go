package channel

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// 请求对象
type (
	// CreateSysChannelReq 创建请求对象
	CreateSysChannelReq struct {
		CreatedBy   string         `json:"created_by"`
		Name        string         `json:"name" validate:"required"`
		Code        string         `json:"code" validate:"required"`
		Description string         `json:"description"`
		Icon        string         `json:"icon"`
		Enabled     bool           `json:"enabled"`
		Sort        int            `json:"sort"`
		Config      map[string]any `json:"config"` // 渠道配置
	}

	// DelSysChannelReq 删除请求对象
	DelSysChannelReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// UpdateSysChannelReq 修改请求对象
	UpdateSysChannelReq struct {
		Ulid        string         `validate:"required" uri:"ulid" json:"ulid"`
		UpdatedBy   string         `json:"updated_by"`
		Name        string         `json:"name"`
		Code        string         `json:"code"`
		Description string         `json:"description"`
		Icon        string         `json:"icon"`
		Enabled     *bool          `json:"enabled"`
		Sort        *int           `json:"sort"`
		Config      map[string]any `json:"config"` // 渠道配置
	}

	// FindSysChannelByIdReq 查询请求对象
	FindSysChannelByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindSysChannelAllReq 查询请求对象
	FindSysChannelAllReq struct {
		Name string `json:"name"`
	}

	// FindSysChannelPageReq 分页查询请求对象
	FindSysChannelPageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}
)

// 输出对象
type (
	// CreateSysChannelRsp 创建返回对象
	CreateSysChannelRsp struct {
		Ulid string `json:"ulid"`
	}

	// FindSysChannelRsp 查询返回对象
	FindSysChannelRsp struct {
		Ulid        string         `json:"ulid"`
		CreatedAt   int64          `json:"created_at"`
		UpdatedAt   int64          `json:"updated_at"`
		CreatedBy   string         `json:"created_by"`
		UpdatedBy   string         `json:"updated_by"`
		Name        string         `json:"name"`
		Code        string         `json:"code"`
		Description string         `json:"description"`
		Icon        string         `json:"icon"`
		Enabled     bool           `json:"enabled"`
		Sort        int            `json:"sort"`
		Config      map[string]any `json:"config"` // 渠道配置
	}

	// FindSysChannelPageRsp 列表查询返回对象
	FindSysChannelPageRsp struct {
		Entries  []*FindSysChannelRsp    `json:"entries"`
		PageData *builder.PageData `json:"page_data"`
	}
)