package channel

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entity "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/channel"
	repo "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/repo/channel"
)

// SysChannelSvc domain service
type SysChannelSvc struct {
	sysChannelRepo *repo.SysChannel
}

// NewSysChannelSvc NewSysChannelSvc
func NewSysChannelSvc() *SysChannelSvc {
	return &SysChannelSvc{
		sysChannelRepo: repo.NewSysChannelImpl(),
	}
}

func (s *SysChannelSvc) Create(ctx context.Context, en *entity.SysChannel) (ulid string, err error) {
	return s.sysChannelRepo.Create(ctx, en)
}

func (s *SysChannelSvc) Delete(ctx context.Context, en *entity.SysChannel) error {
	return s.sysChannelRepo.Delete(ctx, en)
}

func (s *SysChannelSvc) Update(ctx context.Context, en *entity.SysChannel) error {
	return s.sysChannelRepo.Update(ctx, en)
}

func (s *SysChannelSvc) FindById(ctx context.Context, ulid string) (*entity.SysChannel, error) {
	return s.sysChannelRepo.FindById(ctx, ulid)
}

func (s *SysChannelSvc) FindAll(ctx context.Context, queries []*builder.Query) ([]*entity.SysChannel, error) {
	return s.sysChannelRepo.FindAll(ctx, queries)
}

func (s *SysChannelSvc) FindPage(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entity.SysChannel, *builder.PageData, error) {
	return s.sysChannelRepo.FindPage(ctx, queries, reqPage, reqSort)
}

func (s *SysChannelSvc) FindByCode(ctx context.Context, code string) (*entity.SysChannel, error) {
	return s.sysChannelRepo.FindByCode(ctx, code)
}
