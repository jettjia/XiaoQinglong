package boot

import (
	"context"
	"errors"
	"log"
	"os"

	dtoChannel "github.com/jettjia/xiaoqinglong/agent-frame/application/dto/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/application/service/channel"
	srvChannel "github.com/jettjia/xiaoqinglong/agent-frame/domain/srv/channel"
	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	feishu "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel/feishu"
	dingtalk "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel/dingtalk"
	wework "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel/wework"
	weixin "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel/weixin"
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
		{"WeWork", "wework", "WeWork Channel", "MessageSquare", 5},
		{"Weixin", "weixin", "Weixin Channel", "MessageSquare", 6},
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

// StartChannelWsConnections 启动渠道的WebSocket连接
func StartChannelWsConnections() error {
	ctx := context.Background()

	feishuMode := os.Getenv("FEISHU_MODE")
	if feishuMode == "websocket" {
		log.Println("[Init] Starting Feishu WebSocket connection")

		wsManager := feishu.GetWsManager()

		// 设置全局 Feishu WS 发送器（供 dispatcher 使用）
		publicChannel.SetFeishuWSSender(wsManager)

		err := wsManager.StartFeishuWs(ctx, func(channelCtx *publicChannel.MessageContext) error {
			log.Printf("[Feishu WS] Received message from user=%s, chat=%s",
				channelCtx.UserID, channelCtx.SessionID)
			// 调用全局 dispatcher 处理消息
			dispatcher := publicChannel.GetGlobalDispatcher()
			if dispatcher == nil {
				log.Println("[Init] Global dispatcher not initialized yet")
				return errors.New("global dispatcher not initialized")
			}
			return dispatcher.HandleFeishuWSMessage(channelCtx)
		})
		if err != nil {
			log.Printf("[Init] Failed to start Feishu WS: %v", err)
			return err
		}
	}

	// 启动钉钉 WebSocket
	dingtalkMode := os.Getenv("DINGTALK_MODE")
	if dingtalkMode == "websocket" {
		log.Println("[Init] Starting DingTalk WebSocket connection")

		wsManager := dingtalk.GetWsManager()

		// 设置全局 DingTalk WS 发送器（供 dispatcher 使用）
		publicChannel.SetDingTalkWSSender(wsManager)

		err := wsManager.StartDingTalkWs(ctx, func(channelCtx *publicChannel.MessageContext) error {
			log.Printf("[DingTalk WS] Received message from user=%s, chat=%s",
				channelCtx.UserID, channelCtx.SessionID)
			// 调用全局 dispatcher 处理消息
			dispatcher := publicChannel.GetGlobalDispatcher()
			if dispatcher == nil {
				log.Println("[Init] Global dispatcher not initialized yet")
				return errors.New("global dispatcher not initialized")
			}
			return dispatcher.HandleDingTalkWSMessage(channelCtx)
		})
		if err != nil {
			log.Printf("[Init] Failed to start DingTalk WS: %v", err)
			return err
		}
	}

	// 启动企业微信 WebSocket
	weworkMode := os.Getenv("WEWORK_MODE")
	if weworkMode == "websocket" {
		log.Println("[Init] Starting WeWork WebSocket connection")

		wsManager := wework.GetWsManager()

		// 设置全局 WeWork WS 发送器（供 dispatcher 使用）
		publicChannel.SetWeWorkWSSender(wsManager)

		err := wsManager.StartWeWorkWs(ctx, func(channelCtx *publicChannel.MessageContext) error {
			log.Printf("[WeWork WS] Received message from user=%s, chat=%s",
				channelCtx.UserID, channelCtx.SessionID)
			// 调用全局 dispatcher 处理消息
			dispatcher := publicChannel.GetGlobalDispatcher()
			if dispatcher == nil {
				log.Println("[Init] Global dispatcher not initialized yet")
				return errors.New("global dispatcher not initialized")
			}
			return dispatcher.HandleWeWorkWSMessage(channelCtx)
		})
		if err != nil {
			log.Printf("[Init] Failed to start WeWork WS: %v", err)
			return err
		}
	}

	// 启动微信长轮询
	weixinMode := os.Getenv("WEIXIN_MODE")
	if weixinMode == "longpolling" {
		log.Println("[Init] Starting Weixin long polling connection")

		wsManager := weixin.GetWsManager()

		// 设置全局 Weixin WS 发送器（供 dispatcher 使用）
		publicChannel.SetWeixinWSSender(wsManager)

		err := wsManager.StartWeixin(ctx, func(channelCtx *publicChannel.MessageContext) error {
			log.Printf("[Weixin WS] Received message from user=%s, chat=%s",
				channelCtx.UserID, channelCtx.SessionID)
			// 调用全局 dispatcher 处理消息
			dispatcher := publicChannel.GetGlobalDispatcher()
			if dispatcher == nil {
				log.Println("[Init] Global dispatcher not initialized yet")
				return errors.New("global dispatcher not initialized")
			}
			return dispatcher.HandleWeixinWSMessage(channelCtx)
		})
		if err != nil {
			log.Printf("[Init] Failed to start Weixin: %v", err)
			return err
		}
	}

	return nil
}
