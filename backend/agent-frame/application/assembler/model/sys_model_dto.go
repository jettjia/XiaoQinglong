package model

import (
	"github.com/jinzhu/copier"

	dtoModel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/model"
	entityModel "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/model"
)

// SysModelDto assembler
type SysModelDto struct {
}

// NewSysModelDto NewSysModelDto
func NewSysModelDto() *SysModelDto {
	return &SysModelDto{}
}

//////////////////////////////////////////////////////////////////
// dto to entity

// D2ECreateSysModel dto转换成entity
func (a *SysModelDto) D2ECreateSysModel(dto *dtoModel.CreateSysModelReq) *entityModel.SysModel {
	var rspEn entityModel.SysModel

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EDeleteSysModel dto转换成entity
func (a *SysModelDto) D2EDeleteSysModel(dto *dtoModel.DelSysModelReq) *entityModel.SysModel {
	var rspEn entityModel.SysModel

	rspEn.Ulid = dto.Ulid

	return &rspEn
}

// D2EUpdateSysModel dto转换成entity
func (a *SysModelDto) D2EUpdateSysModel(dto *dtoModel.UpdateSysModelReq) *entityModel.SysModel {
	var rspEn entityModel.SysModel

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}
	return &rspEn
}

//////////////////////////////////////////////////////////////
// entity to dto

// E2DCreateSysModel entity转换成dto
func (a *SysModelDto) E2DCreateSysModel(en *entityModel.SysModel) (dto *dtoModel.CreateSysModelRsp) {
	dto = &dtoModel.CreateSysModelRsp{
		Ulid: en.Ulid,
	}

	return
}

// E2DFindSysModelRsp entity转换成dto
func (a *SysModelDto) E2DFindSysModelRsp(en *entityModel.SysModel) *dtoModel.FindSysModelRsp {
	var rspDto dtoModel.FindSysModelRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DGetSysModels entity转换成dto
func (a *SysModelDto) E2DGetSysModels(ens []*entityModel.SysModel) []*dtoModel.FindSysModelRsp {
	if len(ens) == 0 {
		return []*dtoModel.FindSysModelRsp{}
	}

	var modelsRsp []*dtoModel.FindSysModelRsp
	for _, v := range ens {
		cfg := a.E2DFindSysModelRsp(v)
		modelsRsp = append(modelsRsp, cfg)
	}

	return modelsRsp
}