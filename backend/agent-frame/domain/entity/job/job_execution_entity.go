package job

import (
	poJob "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/job"
)

type JobExecution struct {
	poJob.JobExecutionPO

	// 扩展字段
}

func (e *JobExecution) TableName() string {
	return "job_execution_log"
}

// JobStatus constants
const (
	JobStatusRunning = "running"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
)
