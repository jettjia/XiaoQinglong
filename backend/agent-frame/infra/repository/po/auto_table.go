package po

import (
	"github.com/jettjia/igo-pkg/pkg/database/db"

	"github.com/jettjia/xiaoqinglong/agent-frame/config"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/user"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/model"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/knowledge_base"
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
	)

	return
}
