package channel

import (
	"github.com/jinzhu/copier"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/channel"
	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/channel"
)

// SysChannelAssembler assembler
type SysChannelAssembler struct {
}

// NewSysChannelAssembler NewSysChannelAssembler
func NewSysChannelAssembler() *SysChannelAssembler {
	return &SysChannelAssembler{}
}

//////////////////////////////////////////////////////////////////
// dto to entity

// D2ECreateSysChannel dto转换成entity
func (a *SysChannelAssembler) D2ECreateSysChannel(dto *dto.CreateSysChannelReq) *entity.SysChannel {
	var rspEn entity.SysChannel

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EDeleteSysChannel dto转换成entity
func (a *SysChannelAssembler) D2EDeleteSysChannel(dto *dto.DelSysChannelReq) *entity.SysChannel {
	var rspEn entity.SysChannel

	rspEn.Ulid = dto.Ulid

	return &rspEn
}

// D2EUpdateSysChannel dto转换成entity
func (a *SysChannelAssembler) D2EUpdateSysChannel(dto *dto.UpdateSysChannelReq) *entity.SysChannel {
	var rspEn entity.SysChannel

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	// 手动处理 Enabled 字段，因为 DTO 是 *bool 而 Entity 是 bool
	if dto.Enabled != nil {
		rspEn.Enabled = *dto.Enabled
	}

	// 手动处理 Sort 字段，因为 DTO 是 *int 而 Entity 是 int
	if dto.Sort != nil {
		rspEn.Sort = *dto.Sort
	}

	return &rspEn
}

//////////////////////////////////////////////////////////////
// entity to dto

// E2DCreateSysChannel entity转换成dto
func (a *SysChannelAssembler) E2DCreateSysChannel(en *entity.SysChannel) (dtoRsp *dto.CreateSysChannelRsp) {
	dtoRsp = &dto.CreateSysChannelRsp{
		Ulid: en.Ulid,
	}

	return
}

// E2DFindSysChannelRsp entity转换成dto
func (a *SysChannelAssembler) E2DFindSysChannelRsp(en *entity.SysChannel) *dto.FindSysChannelRsp {
	var rspDto dto.FindSysChannelRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DGetSysChannels entity转换成dto
func (a *SysChannelAssembler) E2DGetSysChannels(ens []*entity.SysChannel) []*dto.FindSysChannelRsp {
	if len(ens) == 0 {
		return []*dto.FindSysChannelRsp{}
	}

	var channelsRsp []*dto.FindSysChannelRsp
	for _, v := range ens {
		cfg := a.E2DFindSysChannelRsp(v)
		channelsRsp = append(channelsRsp, cfg)
	}

	return channelsRsp
}