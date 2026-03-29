package model

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// 请求对象
type (
	// CreateSysModelReq 创建SysModel 请求对象
	CreateSysModelReq struct {
		CreatedBy  string `json:"created_by"`
		Name      string `json:"name" validate:"required"`
		Provider  string `json:"provider" validate:"required"`
		BaseUrl   string `json:"baseUrl"`
		ApiKey    string `json:"apiKey"`
		ModelType string `json:"modelType" validate:"required"`
		Category  string `json:"category"`
	}

	// DelSysModelReq 删除 请求对象
	DelSysModelReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// UpdateSysModelReq 修改SysModel 请求对象
	UpdateSysModelReq struct {
		Ulid       string `validate:"required" uri:"ulid" json:"ulid"`
		UpdatedBy  string `json:"updated_by"`
		Name       string `json:"name"`
		Provider   string `json:"provider"`
		BaseUrl    string `json:"baseUrl"`
		ApiKey     string `json:"apiKey"`
		ModelType  string `json:"modelType"`
		Category   string `json:"category"`
		Status     string `json:"status"`
		Latency    string `json:"latency"`
	}

	// FindSysModelByIdReq 查询 请求对象
	FindSysModelByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindSysModelAllReq 查询 请求对象
	FindSysModelAllReq struct {
		ModelType string `json:"modelType"`
	}

	// FindSysModelPageReq 分页查询 请求对象
	FindSysModelPageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}
)

// 输出对象
type (
	// CreateSysModelRsp 创建SysModel 返回对象
	CreateSysModelRsp struct {
		Ulid string `json:"ulid"`
	}

	// FindSysModelRsp 查询SysModel 返回对象
	FindSysModelRsp struct {
		Ulid          string `json:"ulid"`
		CreatedAt     int64  `json:"created_at"`
		UpdatedAt     int64  `json:"updated_at"`
		CreatedBy     string `json:"created_by"`
		UpdatedBy     string `json:"updated_by"`
		Name          string `json:"name"`
		Provider      string `json:"provider"`
		BaseUrl       string `json:"baseUrl"`
		ApiKey        string `json:"api_key"`
		ModelType     string `json:"modelType"`
		Category      string `json:"category"`
		Status        string `json:"status"`
		Latency       string `json:"latency"`
		ContextWindow string `json:"contextWindow"`
		Usage         int    `json:"usage"`
	}

	// FindSysModelPageRsp 列表查询 返回对象
	FindSysModelPageRsp struct {
		Entries  []*FindSysModelRsp `json:"entries"`
		PageData *builder.PageData `json:"page_data"`
	}
)