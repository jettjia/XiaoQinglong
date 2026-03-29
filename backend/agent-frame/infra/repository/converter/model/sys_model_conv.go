package model

import (
	"time"

	"github.com/jinzhu/copier"

	entityModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/model"
	poModel "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/model"
)

// E2PSysModelAdd entity数据转换成数据库po
func E2PSysModelAdd(en *entityModel.SysModel) *poModel.SysModel {
	var po poModel.SysModel
	po.CreatedAt = time.Now().UnixNano()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	return &po
}

// E2PSysModelDel entity数据转换成数据库po
func E2PSysModelDel(en *entityModel.SysModel) *poModel.SysModel {
	var po poModel.SysModel
	po.DeletedAt = time.Now().UnixMilli()

	return &po
}

// E2PSysModelUpdate entity数据转换成数据库po
func E2PSysModelUpdate(en *entityModel.SysModel) *poModel.SysModel {
	var po poModel.SysModel
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}

	po.UpdatedAt = time.Now().UnixNano()
	return &po
}

// P2ESysModel 数据库po转换成entity
func P2ESysModel(po *poModel.SysModel) *entityModel.SysModel {
	var entity entityModel.SysModel
	if err := copier.Copy(&entity, &po); err != nil {
		panic(any(err))
	}

	return &entity
}

func P2ESysModels(pos []*poModel.SysModel) []*entityModel.SysModel {
	ens := make([]*entityModel.SysModel, 0)
	if len(pos) == 0 {
		return ens
	}

	for _, val := range pos {
		cfg := P2ESysModel(val)
		ens = append(ens, cfg)
	}

	return ens
}