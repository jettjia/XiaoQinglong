package knowledge_base

import (
	"github.com/jinzhu/copier"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/knowledge_base"
	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/knowledge_base"
)

// SysKnowledgeBaseDto assembler
type SysKnowledgeBaseDto struct {
}

// NewSysKnowledgeBaseDto NewSysKnowledgeBaseDto
func NewSysKnowledgeBaseDto() *SysKnowledgeBaseDto {
	return &SysKnowledgeBaseDto{}
}

//////////////////////////////////////////////////////////////////
// dto to entity

// D2ECreateSysKnowledgeBase dto转换成entity
func (a *SysKnowledgeBaseDto) D2ECreateSysKnowledgeBase(dto *dto.CreateSysKnowledgeBaseReq) *entity.SysKnowledgeBase {
	var rspEn entity.SysKnowledgeBase

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EDeleteSysKnowledgeBase dto转换成entity
func (a *SysKnowledgeBaseDto) D2EDeleteSysKnowledgeBase(dto *dto.DelSysKnowledgeBaseReq) *entity.SysKnowledgeBase {
	var rspEn entity.SysKnowledgeBase

	rspEn.Ulid = dto.Ulid

	return &rspEn
}

// D2EUpdateSysKnowledgeBase dto转换成entity
func (a *SysKnowledgeBaseDto) D2EUpdateSysKnowledgeBase(dto *dto.UpdateSysKnowledgeBaseReq) *entity.SysKnowledgeBase {
	var rspEn entity.SysKnowledgeBase

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}
	return &rspEn
}

//////////////////////////////////////////////////////////////
// entity to dto

// E2DCreateSysKnowledgeBase entity转换成dto
func (a *SysKnowledgeBaseDto) E2DCreateSysKnowledgeBase(en *entity.SysKnowledgeBase) (dtoRsp *dto.CreateSysKnowledgeBaseRsp) {
	dtoRsp = &dto.CreateSysKnowledgeBaseRsp{
		Ulid: en.Ulid,
	}

	return
}

// E2DFindSysKnowledgeBaseRsp entity转换成dto
func (a *SysKnowledgeBaseDto) E2DFindSysKnowledgeBaseRsp(en *entity.SysKnowledgeBase) *dto.FindSysKnowledgeBaseRsp {
	var rspDto dto.FindSysKnowledgeBaseRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DGetSysKnowledgeBases entity转换成dto
func (a *SysKnowledgeBaseDto) E2DGetSysKnowledgeBases(ens []*entity.SysKnowledgeBase) []*dto.FindSysKnowledgeBaseRsp {
	if len(ens) == 0 {
		return []*dto.FindSysKnowledgeBaseRsp{}
	}

	var knowledgeBasesRsp []*dto.FindSysKnowledgeBaseRsp
	for _, v := range ens {
		cfg := a.E2DFindSysKnowledgeBaseRsp(v)
		knowledgeBasesRsp = append(knowledgeBasesRsp, cfg)
	}

	return knowledgeBasesRsp
}
