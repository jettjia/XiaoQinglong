package chat

import (
	"github.com/jinzhu/copier"

	dto "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/chat"
	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/chat"
)

// ChatAssembler assembler
type ChatAssembler struct {
}

// NewChatAssembler NewChatAssembler
func NewChatAssembler() *ChatAssembler {
	return &ChatAssembler{}
}

//////////////////////////////////////////////////////////////////
// ChatSession: dto to entity

// D2ECreateChatSession dto转换成entity
func (a *ChatAssembler) D2ECreateChatSession(dto *dto.CreateChatSessionReq) *entity.ChatSession {
	var rspEn entity.ChatSession

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EUpdateChatSession dto转换成entity
func (a *ChatAssembler) D2EUpdateChatSession(dto *dto.UpdateChatSessionReq) *entity.ChatSession {
	var rspEn entity.ChatSession

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EDeleteChatSession dto转换成entity
func (a *ChatAssembler) D2EDeleteChatSession(dto *dto.DelChatSessionReq) *entity.ChatSession {
	var rspEn entity.ChatSession
	rspEn.Ulid = dto.Ulid
	return &rspEn
}

// ChatSession: entity to dto

// E2DCreateChatSession entity转换成dto
func (a *ChatAssembler) E2DCreateChatSession(en *entity.ChatSession) *dto.CreateChatSessionRsp {
	return &dto.CreateChatSessionRsp{
		Ulid: en.Ulid,
	}
}

// E2DChatSession entity转换成dto
func (a *ChatAssembler) E2DChatSession(en *entity.ChatSession) *dto.ChatSessionRsp {
	var rspDto dto.ChatSessionRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DChatSessions entity转换成dto列表
func (a *ChatAssembler) E2DChatSessions(ens []*entity.ChatSession) []*dto.ChatSessionRsp {
	if len(ens) == 0 {
		return []*dto.ChatSessionRsp{}
	}

	var result []*dto.ChatSessionRsp
	for _, v := range ens {
		result = append(result, a.E2DChatSession(v))
	}

	return result
}

//////////////////////////////////////////////////////////////////
// ChatMessage: dto to entity

// D2ECreateChatMessage dto转换成entity
func (a *ChatAssembler) D2ECreateChatMessage(dto *dto.CreateChatMessageReq) *entity.ChatMessage {
	var rspEn entity.ChatMessage

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EUpdateChatMessage dto转换成entity
func (a *ChatAssembler) D2EUpdateChatMessage(dto *dto.UpdateChatMessageReq) *entity.ChatMessage {
	var rspEn entity.ChatMessage

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// ChatMessage: entity to dto

// E2DCreateChatMessage entity转换成dto
func (a *ChatAssembler) E2DCreateChatMessage(en *entity.ChatMessage) *dto.CreateChatMessageRsp {
	return &dto.CreateChatMessageRsp{
		Ulid: en.Ulid,
	}
}

// E2DChatMessage entity转换成dto
func (a *ChatAssembler) E2DChatMessage(en *entity.ChatMessage) *dto.ChatMessageRsp {
	var rspDto dto.ChatMessageRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	// 强制覆盖 metadata，确保从 entity 获取正确的值
	rspDto.Metadata = en.Metadata
	if rspDto.Metadata == "" && en.Metadata != "" {
		println("[WARN] E2DChatMessage: copier overwrote Metadata, en.Metadata =", en.Metadata)
	}

	return &rspDto
}

// E2DChatMessages entity转换成dto列表
func (a *ChatAssembler) E2DChatMessages(ens []*entity.ChatMessage) []*dto.ChatMessageRsp {
	if len(ens) == 0 {
		return []*dto.ChatMessageRsp{}
	}

	var result []*dto.ChatMessageRsp
	for _, v := range ens {
		result = append(result, a.E2DChatMessage(v))
	}

	return result
}

//////////////////////////////////////////////////////////////////
// ChatApproval: dto to entity

// D2ECreateChatApproval dto转换成entity
func (a *ChatAssembler) D2ECreateChatApproval(dto *dto.CreateChatApprovalReq) *entity.ChatApproval {
	var rspEn entity.ChatApproval

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// D2EUpdateChatApproval dto转换成entity
func (a *ChatAssembler) D2EUpdateChatApproval(dto *dto.UpdateChatApprovalStatusReq) *entity.ChatApproval {
	var rspEn entity.ChatApproval

	if err := copier.Copy(&rspEn, &dto); err != nil {
		panic(any(err))
	}

	return &rspEn
}

// ChatApproval: entity to dto

// E2DCreateChatApproval entity转换成dto
func (a *ChatAssembler) E2DCreateChatApproval(en *entity.ChatApproval) *dto.CreateChatApprovalRsp {
	return &dto.CreateChatApprovalRsp{
		Ulid: en.Ulid,
	}
}

// E2DChatApproval entity转换成dto
func (a *ChatAssembler) E2DChatApproval(en *entity.ChatApproval) *dto.ChatApprovalRsp {
	var rspDto dto.ChatApprovalRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}

// E2DChatApprovals entity转换成dto列表
func (a *ChatAssembler) E2DChatApprovals(ens []*entity.ChatApproval) []*dto.ChatApprovalRsp {
	if len(ens) == 0 {
		return []*dto.ChatApprovalRsp{}
	}

	var result []*dto.ChatApprovalRsp
	for _, v := range ens {
		result = append(result, a.E2DChatApproval(v))
	}

	return result
}

//////////////////////////////////////////////////////////////////
// ChatTokenStats: entity to dto

// E2DChatTokenStats entity转换成dto
func (a *ChatAssembler) E2DChatTokenStats(en *entity.ChatTokenStats) *dto.ChatTokenStatsRsp {
	var rspDto dto.ChatTokenStatsRsp

	if err := copier.Copy(&rspDto, &en); err != nil {
		panic(any(err))
	}

	return &rspDto
}