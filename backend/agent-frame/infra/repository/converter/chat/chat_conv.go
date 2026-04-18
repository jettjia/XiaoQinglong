package chat

import (
	"time"

	"github.com/jinzhu/copier"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/chat"
	po "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/chat"
)

// ====== ChatSession ======

// E2PChatSessionAdd entity转po for create
func E2PChatSessionAdd(en *entity.ChatSession) *po.ChatSession {
	var po po.ChatSession
	po.CreatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	return &po
}

// E2PChatSessionUpdate entity转po for update
func E2PChatSessionUpdate(en *entity.ChatSession) *po.ChatSession {
	var po po.ChatSession
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	po.UpdatedAt = time.Now().UnixMilli()
	return &po
}

// P2EChatSession po转entity
func P2EChatSession(p *po.ChatSession) *entity.ChatSession {
	var en entity.ChatSession
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}
	return &en
}

func P2EChatSessions(pos []*po.ChatSession) []*entity.ChatSession {
	ens := make([]*entity.ChatSession, 0)
	if len(pos) == 0 {
		return ens
	}
	for _, val := range pos {
		ens = append(ens, P2EChatSession(val))
	}
	return ens
}

// ====== ChatMessage ======

// E2PChatMessageAdd entity转po for create
func E2PChatMessageAdd(en *entity.ChatMessage) *po.ChatMessage {
	var po po.ChatMessage
	po.CreatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	return &po
}

// E2PChatMessageUpdate entity转po for update
func E2PChatMessageUpdate(en *entity.ChatMessage) *po.ChatMessage {
	var po po.ChatMessage
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	return &po
}

// P2EChatMessage po转entity
func P2EChatMessage(p *po.ChatMessage) *entity.ChatMessage {
	var en entity.ChatMessage
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}
	// 显式复制 Metadata 字段，因为 StringJSON 和 string 类型不匹配
	en.Metadata = p.Metadata.Val
	return &en
}

func P2EChatMessages(pos []*po.ChatMessage) []*entity.ChatMessage {
	ens := make([]*entity.ChatMessage, 0)
	if len(pos) == 0 {
		return ens
	}
	for _, val := range pos {
		ens = append(ens, P2EChatMessage(val))
	}
	return ens
}

// ====== ChatApproval ======

// E2PChatApprovalAdd entity转po for create
func E2PChatApprovalAdd(en *entity.ChatApproval) *po.ChatApproval {
	var po po.ChatApproval
	po.CreatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	return &po
}

// E2PChatApprovalUpdate entity转po for update
func E2PChatApprovalUpdate(en *entity.ChatApproval) *po.ChatApproval {
	var po po.ChatApproval
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	return &po
}

// P2EChatApproval po转entity
func P2EChatApproval(p *po.ChatApproval) *entity.ChatApproval {
	var en entity.ChatApproval
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}
	return &en
}

func P2EChatApprovals(pos []*po.ChatApproval) []*entity.ChatApproval {
	ens := make([]*entity.ChatApproval, 0)
	if len(pos) == 0 {
		return ens
	}
	for _, val := range pos {
		ens = append(ens, P2EChatApproval(val))
	}
	return ens
}

// ====== ChatTokenStats ======

// E2PChatTokenStatsAdd entity转po for create
func E2PChatTokenStatsAdd(en *entity.ChatTokenStats) *po.ChatTokenStats {
	var po po.ChatTokenStats
	po.CreatedAt = time.Now().UnixMilli()
	po.UpdatedAt = time.Now().UnixMilli()
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	return &po
}

// E2PChatTokenStatsUpdate entity转po for update
func E2PChatTokenStatsUpdate(en *entity.ChatTokenStats) *po.ChatTokenStats {
	var po po.ChatTokenStats
	if err := copier.Copy(&po, &en); err != nil {
		panic(any(err))
	}
	po.UpdatedAt = time.Now().UnixMilli()
	return &po
}

// P2EChatTokenStats po转entity
func P2EChatTokenStats(p *po.ChatTokenStats) *entity.ChatTokenStats {
	var en entity.ChatTokenStats
	if err := copier.Copy(&en, &p); err != nil {
		panic(any(err))
	}
	return &en
}
