package agent

import (
	"github.com/jinzhu/copier"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/agent"
	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/agent"
)

// SysAgentAssembler assembler
type SysAgentAssembler struct {
}

// NewSysAgentAssembler NewSysAgentAssembler
func NewSysAgentAssembler() *SysAgentAssembler {
	return &SysAgentAssembler{}
}

//////////////////////////////////////////////////////////////////
// dto to entity

// D2ECreateSysAgent dto转换成entity
func (a *SysAgentAssembler) D2ECreateSysAgent(dto *dto.CreateSysAgentReq) *entity.SysAgent {
	var rspEn entity.SysAgent

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EDeleteSysAgent dto转换成entity
func (a *SysAgentAssembler) D2EDeleteSysAgent(dto *dto.DelSysAgentReq) *entity.SysAgent {
	var rspEn entity.SysAgent

	rspEn.Ulid = dto.Ulid

	return &rspEn
}

// D2EUpdateSysAgent dto转换成entity
func (a *SysAgentAssembler) D2EUpdateSysAgent(dto *dto.UpdateSysAgentReq) *entity.SysAgent {
	var rspEn entity.SysAgent

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}
	return &rspEn
}

//////////////////////////////////////////////////////////////
// entity to dto

// E2DCreateSysAgent entity转换成dto
func (a *SysAgentAssembler) E2DCreateSysAgent(en *entity.SysAgent) (dtoRsp *dto.CreateSysAgentRsp) {
	dtoRsp = &dto.CreateSysAgentRsp{
		Ulid: en.Ulid,
	}

	return
}

// E2DFindSysAgentRsp entity转换成dto
func (a *SysAgentAssembler) E2DFindSysAgentRsp(en *entity.SysAgent) *dto.FindSysAgentRsp {
	var rspDto dto.FindSysAgentRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DGetSysAgents entity转换成dto
func (a *SysAgentAssembler) E2DGetSysAgents(ens []*entity.SysAgent) []*dto.FindSysAgentRsp {
	if len(ens) == 0 {
		return []*dto.FindSysAgentRsp{}
	}

	var agentsRsp []*dto.FindSysAgentRsp
	for _, v := range ens {
		cfg := a.E2DFindSysAgentRsp(v)
		agentsRsp = append(agentsRsp, cfg)
	}

	return agentsRsp
}
