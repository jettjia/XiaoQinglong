package po

import (
	"github.com/jettjia/igo-pkg/pkg/database/db"

	"github.com/jettjia/xiaoqinglong/agent-frame/config"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/agent"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/chat"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/job"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/knowledge_base"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/memory"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/skill"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/user"
)

// AutoTable auto create table
func AutoTable() (err error) {
	conf := config.NewConfig()
	dbCli := db.NewDBClient(conf).Conn

	err = dbCli.AutoMigrate(
		user.SysUser{},
		user.SysLog{},
		model.SysModel{},
		knowledge_base.SysKnowledgeBase{},
		skill.SysSkill{},
		agent.SysAgent{},
		channel.SysChannel{},
		chat.ChatSession{},
		chat.ChatMessage{},
		chat.ChatApproval{},
		chat.ChatTokenStats{},
		memory.AgentMemory{},
		memory.MemoryIndex{},
		job.JobExecutionPO{},
	)

	return
}
