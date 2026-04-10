package knowledge_base

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// 请求对象
type (
	// CreateSysKnowledgeBaseReq 创建请求对象
	CreateSysKnowledgeBaseReq struct {
		CreatedBy    string `json:"created_by"`
		Name         string `json:"name" validate:"required"`
		Description  string `json:"description"`
		RetrievalUrl string `json:"retrievalUrl" validate:"required"`
		Token        string `json:"token"`
		Enabled      bool   `json:"enabled"`
	}

	// DelSysKnowledgeBaseReq 删除请求对象
	DelSysKnowledgeBaseReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// UpdateSysKnowledgeBaseReq 修改请求对象
	UpdateSysKnowledgeBaseReq struct {
		Ulid         string `validate:"required" uri:"ulid" json:"ulid"`
		UpdatedBy    string `json:"updated_by"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		RetrievalUrl string `json:"retrievalUrl"`
		Token        string `json:"token"`
		Enabled      *bool  `json:"enabled"`
	}

	// FindSysKnowledgeBaseByIdReq 查询请求对象
	FindSysKnowledgeBaseByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"`
	}

	// FindSysKnowledgeBaseAllReq 查询请求对象
	FindSysKnowledgeBaseAllReq struct {
	}

	// FindSysKnowledgeBasePageReq 分页查询请求对象
	FindSysKnowledgeBasePageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}

	// RecallTestReq 召回测试请求对象
	RecallTestReq struct {
		Query string `json:"query" validate:"required"`
		TopK  int    `json:"top_k"`
	}
)

// 输出对象
type (
	// CreateSysKnowledgeBaseRsp 创建返回对象
	CreateSysKnowledgeBaseRsp struct {
		Ulid string `json:"ulid"`
	}

	// FindSysKnowledgeBaseRsp 查询返回对象
	FindSysKnowledgeBaseRsp struct {
		Ulid         string `json:"ulid"`
		CreatedAt    int64  `json:"created_at"`
		UpdatedAt    int64  `json:"updated_at"`
		CreatedBy    string `json:"created_by"`
		UpdatedBy    string `json:"updated_by"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		RetrievalUrl string `json:"retrievalUrl"`
		Token        string `json:"token"`
		Enabled      bool   `json:"enabled"`
	}

	// FindSysKnowledgeBasePageRsp 列表查询返回对象
	FindSysKnowledgeBasePageRsp struct {
		Entries  []*FindSysKnowledgeBaseRsp `json:"entries"`
		PageData *builder.PageData            `json:"page_data"`
	}

	// RecallTestRsp 召回测试返回对象
	RecallTestRsp struct {
		Title   string  `json:"title"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	}
)
