package feishu

import (
	"context"
	"os"

	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// WsManager WS连接管理器（单例）
type WsManager struct {
	feishuWsHandler *WsHandler
}

var wsManager *WsManager

// GetWsManager 获取WS管理器单例
func GetWsManager() *WsManager {
	if wsManager == nil {
		wsManager = &WsManager{}
	}
	return wsManager
}

// StartFeishuWs 启动飞书WebSocket连接
func (m *WsManager) StartFeishuWs(ctx context.Context, onMessage func(ctx *publicChannel.MessageContext) error) error {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	domain := os.Getenv("FEISHU_DOMAIN")
	verificationToken := os.Getenv("FEISHU_VERIFICATION_TOKEN")
	encryptKey := os.Getenv("FEISHU_ENCRYPT_KEY")

	if appID == "" || appSecret == "" {
		logger.GetRunnerLogger().Warn("[WsManager] FEISHU_APP_ID or FEISHU_APP_SECRET not set, skipping WS")
		return nil
	}

	logger.GetRunnerLogger().Infof("[WsManager] Starting Feishu WebSocket, app_id=%s", appID)

	handler := NewWsHandler(WsHandlerConfig{
		AppID:             appID,
		AppSecret:         appSecret,
		Domain:            domain,
		VerificationToken: verificationToken,
		EncryptKey:        encryptKey,
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
		logger.GetRunnerLogger().Errorf("[WsManager] Failed to start Feishu WS: %v", err)
		return err
	}

	m.feishuWsHandler = handler
	return nil
}

// Stop 停止所有WS连接
func (m *WsManager) Stop() {
	if m.feishuWsHandler != nil {
		m.feishuWsHandler.Stop()
	}
}