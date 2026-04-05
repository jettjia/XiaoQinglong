package dingtalk

import (
	"context"
	"errors"
	"os"

	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// WsManager WS连接管理器（单例）
type WsManager struct {
	dingtalkWsHandler *WsHandler
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
	if m.dingtalkWsHandler == nil {
		return errors.New("dingtalk ws handler not initialized")
	}
	return m.dingtalkWsHandler.SendText(ctx, receiveID, msgType, content)
}

// StartDingTalkWs 启动钉钉 WebSocket 连接
func (m *WsManager) StartDingTalkWs(ctx context.Context, onMessage func(ctx *publicChannel.MessageContext) error) error {
	clientID := os.Getenv("DINGTALK_CLIENT_ID")
	clientSecret := os.Getenv("DINGTALK_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		logger.GetRunnerLogger().Warn("[WsManager] DINGTALK_CLIENT_ID or DINGTALK_CLIENT_SECRET not set, skipping WS")
		return nil
	}

	logger.GetRunnerLogger().Infof("[WsManager] Starting DingTalk WebSocket, client_id=%s", clientID)

	handler := NewWsHandler(WsHandlerConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
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
		logger.GetRunnerLogger().Errorf("[WsManager] Failed to start DingTalk WS: %v", err)
		return err
	}

	m.dingtalkWsHandler = handler
	return nil
}

// Stop 停止所有WS连接
func (m *WsManager) Stop() {
	if m.dingtalkWsHandler != nil {
		m.dingtalkWsHandler.Stop()
	}
}