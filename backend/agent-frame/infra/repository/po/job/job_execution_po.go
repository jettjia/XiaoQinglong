package job

import (
	"github.com/jettjia/igo-pkg/pkg/util"
	"gorm.io/gorm"
)

type JobExecutionPO struct {
	Ulid          string `gorm:"column:ulid;primaryKey;type:varchar(128);comment:ulid;" json:"ulid"`
	CreatedAt     int64  `gorm:"column:created_at;autoCreateTime:milli;type:bigint;comment:创建时间;" json:"created_at"`
	UpdatedAt     int64  `gorm:"column:updated_at;autoUpdateTime:milli;type:bigint;comment:修改时间;" json:"updated_at"`
	DeletedAt     int64  `gorm:"column:deleted_at;autoDeletedTime:milli;type:bigint;comment:删除时间;" json:"deleted_at"`
	AgentId       string `gorm:"column:agent_id;type:varchar(64);comment:agent_id;" json:"agent_id"`
	AgentName     string `gorm:"column:agent_name;type:varchar(255);comment:agent名称;" json:"agent_name"`
	SessionId     string `gorm:"column:session_id;type:varchar(64);comment:session_id;" json:"session_id"`
	Status        string `gorm:"column:status;type:varchar(20);comment:状态;" json:"status"`
	TriggerTime   int64  `gorm:"column:trigger_time;type:bigint;comment:触发时间;" json:"trigger_time"`
	StartedAt     int64  `gorm:"column:started_at;type:bigint;comment:开始时间;" json:"started_at"`
	FinishedAt    int64  `gorm:"column:finished_at;type:bigint;comment:结束时间;" json:"finished_at"`
	InputSummary  string `gorm:"column:input_summary;type:varchar(500);comment:输入摘要;" json:"input_summary"`
	OutputSummary string `gorm:"column:output_summary;type:varchar(1000);comment:输出摘要;" json:"output_summary"`
	OutputFull    string `gorm:"column:output_full;type:text;comment:完整输出;" json:"output_full"`
	ErrorMsg      string `gorm:"column:error_msg;type:text;comment:错误信息;" json:"error_msg"`
	TokensUsed    int    `gorm:"column:tokens_used;type:int;comment:token消耗;" json:"tokens_used"`
	LatencyMs     int64  `gorm:"column:latency_ms;type:bigint;comment:延迟毫秒;" json:"latency_ms"`
}

func (po *JobExecutionPO) BeforeCreate(tx *gorm.DB) error {
	if po.Ulid == "" {
		po.Ulid = util.Ulid()
	}
	return nil
}

func (po *JobExecutionPO) TableName() string {
	return "job_execution_log"
}

// JobStatus constants
const (
	JobStatusRunning = "running"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
)
