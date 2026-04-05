package wework

import (
	"context"
	"errors"
	"os"

	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// WsManager WS连接管理器（单例）
type WsManager struct {
	weworkWsHandler *WsHandler
}

var wsManager *WsManager

// GetWsManager 获取WS管理器单例
func GetWsManager() *WsManager {
	if wsManager == nil {
		wsManager = &WsManager{}
	}
	return wsManager
}

// SendText 通过 WebSocket 发送文本消息
func (m *WsManager) SendText(ctx context.Context, receiveID, msgType, content string) error {
	if m.weworkWsHandler == nil {
		return errors.New("wework ws handler not initialized")
	}
	return m.weworkWsHandler.SendText(ctx, receiveID, msgType, content)
}

// StartWeWorkWs 启动企业微信 WebSocket 连接
func (m *WsManager) StartWeWorkWs(ctx context.Context, onMessage func(ctx *publicChannel.MessageContext) error) error {
	botID := os.Getenv("WEWORK_BOT_ID")
	secret := os.Getenv("WEWORK_SECRET")
	url := os.Getenv("WEWORK_WS_URL")

	if botID == "" || secret == "" {
		logger.GetRunnerLogger().Warn("[WsManager] WEWORK_BOT_ID or WEWORK_SECRET not set, skipping WS")
		return nil
	}

	if url == "" {
		url = "wss://openws.work.weixin.qq.com"
	}

	logger.GetRunnerLogger().Infof("[WsManager] Starting WeWork WebSocket, bot_id=%s", botID)

	handler := NewWsHandler(WsHandlerConfig{
		BotID: botID,
		Secret: secret,
		URL:   url,
		OnMessage: func(ctx *MessageContext) error {
			channelCtx := &publicChannel.MessageContext{
				ChannelCode: ctx.ChannelCode,
				SessionID:   ctx.SessionID,
				UserID:      ctx.UserID,
				Content:     ctx.Content,
				Header:      ctx.Header,
			}
			return onMessage(channelCtx)
		},
	})

	if err := handler.Start(ctx); err != nil {
		logger.GetRunnerLogger().Errorf("[WsManager] Failed to start WeWork WS: %v", err)
		return err
	}

	m.weworkWsHandler = handler
	return nil
}

// Stop 停止所有WS连接
func (m *WsManager) Stop() {
	if m.weworkWsHandler != nil {
		m.weworkWsHandler.Stop()
	}
}