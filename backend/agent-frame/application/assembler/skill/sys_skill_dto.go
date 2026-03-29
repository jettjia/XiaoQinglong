package skill

import (
	"github.com/jinzhu/copier"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/skill"
	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/skill"
)

// SysSkillDto assembler
type SysSkillDto struct {
}

// NewSysSkillDto NewSysSkillDto
func NewSysSkillDto() *SysSkillDto {
	return &SysSkillDto{}
}

//////////////////////////////////////////////////////////////////
// dto to entity

// D2ECreateSysSkill dto转换成entity
func (a *SysSkillDto) D2ECreateSysSkill(dto *dto.CreateSysSkillReq) *entity.SysSkill {
	var rspEn entity.SysSkill

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EDeleteSysSkill dto转换成entity
func (a *SysSkillDto) D2EDeleteSysSkill(dto *dto.DelSysSkillReq) *entity.SysSkill {
	var rspEn entity.SysSkill

	rspEn.Ulid = dto.Ulid

	return &rspEn
}

// D2EUpdateSysSkill dto转换成entity
func (a *SysSkillDto) D2EUpdateSysSkill(dto *dto.UpdateSysSkillReq) *entity.SysSkill {
	var rspEn entity.SysSkill

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}
	return &rspEn
}

//////////////////////////////////////////////////////////////
// entity to dto

// E2DCreateSysSkill entity转换成dto
func (a *SysSkillDto) E2DCreateSysSkill(en *entity.SysSkill) (dtoRsp *dto.CreateSysSkillRsp) {
	dtoRsp = &dto.CreateSysSkillRsp{
		Ulid: en.Ulid,
	}

	return
}

// E2DFindSysSkillRsp entity转换成dto
func (a *SysSkillDto) E2DFindSysSkillRsp(en *entity.SysSkill) *dto.FindSysSkillRsp {
	var rspDto dto.FindSysSkillRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DGetSysSkills entity转换成dto
func (a *SysSkillDto) E2DGetSysSkills(ens []*entity.SysSkill) []*dto.FindSysSkillRsp {
	if len(ens) == 0 {
		return []*dto.FindSysSkillRsp{}
	}

	var skillsRsp []*dto.FindSysSkillRsp
	for _, v := range ens {
		cfg := a.E2DFindSysSkillRsp(v)
		skillsRsp = append(skillsRsp, cfg)
	}

	return skillsRsp
}
