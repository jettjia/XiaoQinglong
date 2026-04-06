package weixin

import (
	"context"
	"errors"
	"os"

	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// WsManager WS连接管理器（单例）
type WsManager struct {
	weixinWsHandler *WsHandler
	onMessage       func(ctx *publicChannel.MessageContext) error
}

var wsManager *WsManager

// GetWsManager 获取WS管理器单例
func GetWsManager() *WsManager {
	if wsManager == nil {
		wsManager = &WsManager{}
	}
	return wsManager
}

// SendText 通过长轮询发送文本消息
func (m *WsManager) SendText(ctx context.Context, receiveID, msgType, content string) error {
	if m.weixinWsHandler == nil {
		return errors.New("weixin ws handler not initialized")
	}
	return m.weixinWsHandler.SendText(ctx, receiveID, msgType, content)
}

// StartWeixin 启动微信长轮询连接
func (m *WsManager) StartWeixin(ctx context.Context, onMessage func(ctx *publicChannel.MessageContext) error) error {
	accountID := os.Getenv("WEIXIN_ACCOUNT_ID")
	if accountID == "" {
		accountID = "default"
	}

	// 保存 onMessage 回调，用于登录后启动
	m.onMessage = onMessage

	logger.GetRunnerLogger().Infof("[WsManager] Starting Weixin, account=%s", accountID)

	handler, err := NewWsHandler(WsHandlerConfig{
		AccountID: accountID,
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
	if err != nil {
		logger.GetRunnerLogger().Errorf("[WsManager] Failed to create Weixin handler: %v", err)
		return err
	}

	m.weixinWsHandler = handler

	// 检查是否需要登录
	if handler.NeedQRLogin() {
		logger.GetRunnerLogger().Warn("[WsManager] Weixin not logged in, skipping long polling start")
		// 登录后需要调用 StartAfterLogin 启动长轮询
		return nil
	}

	if err := handler.Start(ctx); err != nil {
		logger.GetRunnerLogger().Errorf("[WsManager] Failed to start Weixin: %v", err)
		return err
	}

	return nil
}

// StartAfterLogin 登录成功后启动长轮询
func (m *WsManager) StartAfterLogin(ctx context.Context) error {
	if m.weixinWsHandler == nil {
		return errors.New("weixin ws handler not initialized")
	}

	logger.GetRunnerLogger().Info("[WsManager] Starting Weixin long polling after login")
	return m.weixinWsHandler.Start(ctx)
}

// NeedQRLogin 检查是否需要二维码登录
func (m *WsManager) NeedQRLogin() bool {
	if m.weixinWsHandler == nil {
		return true
	}
	return m.weixinWsHandler.NeedQRLogin()
}

// GetHandler 获取 handler（用于登录等操作）
func (m *WsManager) GetHandler() *WsHandler {
	return m.weixinWsHandler
}

// SetHandler 设置 handler（用于登录后）
func (m *WsManager) SetHandler(handler *WsHandler) {
	m.weixinWsHandler = handler
}

// GetQRCode 获取登录二维码
func (m *WsManager) GetQRCode(ctx context.Context) (string, error) {
	if m.weixinWsHandler == nil {
		return "", errors.New("weixin ws handler not initialized")
	}
	return m.weixinWsHandler.GetQRCode(ctx)
}

// CompleteQRLogin 完成二维码登录
func (m *WsManager) CompleteQRLogin(ctx context.Context, qrcode string, onStatus func(status string)) (*TokenInfo, error) {
	if m.weixinWsHandler == nil {
		return nil, errors.New("weixin ws handler not initialized")
	}
	return m.weixinWsHandler.CompleteQRLogin(ctx, qrcode, onStatus)
}

// Stop 停止所有WS连接
func (m *WsManager) Stop() {
	if m.weixinWsHandler != nil {
		m.weixinWsHandler.Stop()
	}
}
