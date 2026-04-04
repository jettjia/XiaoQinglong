package job

import (
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"
)

// 请求对象
type (
	// FindJobExecutionByIdReq 查询 请求对象
	FindJobExecutionByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"` // ulid
	}

	// FindJobExecutionByAgentIdReq 根据AgentId查询 请求对象
	FindJobExecutionByAgentIdReq struct {
		AgentId string `form:"agent_id" validate:"required" json:"agent_id"` // agent_id
		Limit   int    `form:"limit" json:"limit"`                           // 限制数量
	}

	// FindJobExecutionPageReq 分页查询 请求对象
	FindJobExecutionPageReq struct {
		Query    []*builder.Query  `json:"query"`
		PageData *builder.PageData `json:"page_data"`
		SortData *builder.SortData `json:"sort_data"`
	}
)

// 输出对象
type (
	// FindJobExecutionRsp 查询JobExecution 返回对象
	FindJobExecutionRsp struct {
		Ulid          string `json:"ulid"`          // ulid
		CreatedAt     int64  `json:"created_at"`   // 创建时间
		UpdatedAt     int64  `json:"updated_at"`   // 修改时间
		AgentId       string `json:"agent_id"`     // agent_id
		AgentName     string `json:"agent_name"`   // agent名称
		SessionId     string `json:"session_id"`   // session_id
		Status        string `json:"status"`       // 状态
		TriggerTime   int64  `json:"trigger_time"` // 触发时间
		StartedAt     int64  `json:"started_at"`   // 开始时间
		FinishedAt    int64  `json:"finished_at"`  // 结束时间
		InputSummary  string `json:"input_summary"`  // 输入摘要
		OutputSummary string `json:"output_summary"` // 输出摘要
		OutputFull    string `json:"output_full"`   // 完整输出
		ErrorMsg      string `json:"error_msg"`     // 错误信息
		TokensUsed    int    `json:"tokens_used"`   // token消耗
		LatencyMs     int64  `json:"latency_ms"`    // 延迟毫秒
	}

	// FindJobExecutionPageRsp 分页查询 返回对象
	FindJobExecutionPageRsp struct {
		Entries  []*FindJobExecutionRsp `json:"entries"`
		PageData *builder.PageData     `json:"page_data"`
	}

	// FindJobExecutionListRsp 列表查询 返回对象
	FindJobExecutionListRsp struct {
		Entries []*FindJobExecutionRsp `json:"entries"`
	}
)
