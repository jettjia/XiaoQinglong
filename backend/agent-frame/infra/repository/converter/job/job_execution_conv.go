package job

import (
	"time"

	"github.com/jinzhu/copier"

	entityJob "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/job"
	poJob "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/job"
)

// E2PJobExecutionAdd entity数据转换成数据库po
func E2PJobExecutionAdd(en *entityJob.JobExecution) *poJob.JobExecutionPO {
	var po poJob.JobExecutionPO
	po.CreatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	return &po
}

// E2PJobExecutionDel entity数据转换成数据库po (软删除)
func E2PJobExecutionDel(en *entityJob.JobExecution) *poJob.JobExecutionPO {
	var po poJob.JobExecutionPO
	po.DeletedAt = time.Now().UnixMilli()

	return &po
}

// E2PJobExecutionUpdate entity数据转换成数据库po
func E2PJobExecutionUpdate(en *entityJob.JobExecution) *poJob.JobExecutionPO {
	var po poJob.JobExecutionPO
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	po.UpdatedAt = time.Now().UnixMilli()
	return &po
}

// P2EJobExecution 数据库po转换成entity
func P2EJobExecution(po *poJob.JobExecutionPO) *entityJob.JobExecution {
	var entity entityJob.JobExecution
	if err := copier.Copy(&entity, &po); err != nil {
		panic(any(err))
	}

	return &entity
}

// P2EJobExecutions 数据库po列表转换成entity列表
func P2EJobExecutions(pos []*poJob.JobExecutionPO) []*entityJob.JobExecution {
	ens := make([]*entityJob.JobExecution, 0)
	if len(pos) == 0 {
		return ens
	}

	for _, val := range pos {
		cfg := P2EJobExecution(val)
		ens = append(ens, cfg)
	}

	return ens
}
