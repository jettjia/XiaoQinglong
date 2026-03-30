package boot

import (
	"context"
	"log"

	dtoChannel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/channel"
	srvChannel "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/channel"
)

// initDefaultChannels 初始化默认渠道
func initDefaultChannels() error {
	log.Println("[Init] Initializing default channels")

	channelSvc := channel.NewSysChannelService()
	domainChannelSvc := srvChannel.NewSysChannelSvc()
	ctx := context.Background()

	// 默认渠道列表
	defaultChannels := []struct {
		name        string
		code        string
		description string
		icon        string
		sort        int
	}{
		{"API", "api", "API Channel", "Globe", 1},
		{"Web", "web", "Web Channel", "Globe", 2},
		{"Feishu", "feishu", "Feishu Channel", "MessageSquare", 3},
		{"DingTalk", "dingtalk", "DingTalk Channel", "MessageSquare", 4},
	}

	for _, ch := range defaultChannels {
		// 检查是否已存在
		existing, err := domainChannelSvc.FindByCode(ctx, ch.code)
		if err != nil {
			return err
		}
		if existing != nil {
			// 已存在，跳过
			continue
		}

		// 创建渠道
		createReq := &dtoChannel.CreateSysChannelReq{
			Name:        ch.name,
			Code:        ch.code,
			Description: ch.description,
			Icon:        ch.icon,
			Enabled:     true,
			Sort:        ch.sort,
		}

		_, err = channelSvc.CreateSysChannel(ctx, createReq)
		if err != nil {
			return err
		}
		log.Printf("[Init] Created channel: %s (%s)", ch.name, ch.code)
	}

	return nil
}
