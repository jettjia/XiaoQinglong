package weixin

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// WsHandler 微信长连接处理器
// 通过长轮询方式接收消息，存储 context_token 用于回复
type WsHandler struct {
	accountID string
	auth      *WeixinAuth
	client    *WeixinAPIClient

	// 回调配置
	onMessage func(ctx *MessageContext) error

	// 存储 context token: userID -> contextToken
	contextTokens sync.Map

	// 停止通道
	stopChan chan struct{}

	// 运行状态
	running   bool
	runningMu sync.Mutex
}

// WsHandlerConfig WebSocket 处理器配置
type WsHandlerConfig struct {
	AccountID string
	OnMessage func(ctx *MessageContext) error // 消息回调
}

// NewWsHandler 创建 WebSocket 处理器
func NewWsHandler(cfg WsHandlerConfig) (*WsHandler, error) {
	auth, err := NewWeixinAuth(cfg.AccountID)
	if err != nil {
		return nil, err
	}

	client := NewWeixinAPIClient(DefaultWeixinBaseURL, "")

	h := &WsHandler{
		accountID: cfg.AccountID,
		auth:      auth,
		client:    client,
		onMessage: cfg.OnMessage,
		stopChan:  make(chan struct{}),
	}

	return h, nil
}

// Start 启动长轮询连接
func (h *WsHandler) Start(ctx context.Context) error {
	h.runningMu.Lock()
	if h.running {
		h.runningMu.Unlock()
		logger.GetRunnerLogger().Info("[Weixin WS] Already running, skipping start")
		return nil
	}
	h.running = true
	h.runningMu.Unlock()

	logger.GetRunnerLogger().Infof("[Weixin WS] Starting, account=%s", h.accountID)

	// 加载存储的 token
	tokenInfo, err := h.auth.LoadToken()
	if err != nil {
		logger.GetRunnerLogger().Warnf("[Weixin WS] Failed to load token: %v", err)
	}

	if tokenInfo != nil && h.auth.IsTokenValid(tokenInfo) {
		h.client.SetToken(tokenInfo.Token)
		logger.GetRunnerLogger().Info("[Weixin WS] Token loaded and valid")
	} else {
		logger.GetRunnerLogger().Warn("[Weixin WS] No valid token found, need QR login")
	}

	// 启动消息接收循环
	go h.receiveMessages(ctx)

	return nil
}

// receiveMessages 处理长轮询消息接收
func (h *WsHandler) receiveMessages(ctx context.Context) {
	var getUpdatesBuf string
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.GetRunnerLogger().Info("[Weixin WS] Context cancelled, stopping")
			return
		case <-h.stopChan:
			logger.GetRunnerLogger().Info("[Weixin WS] Stopped")
			return
		default:
			resp, err := h.client.GetUpdates(ctx, &GetUpdatesReq{
				GetUpdatesBuf: getUpdatesBuf,
			})

			if err != nil {
				logger.GetRunnerLogger().Errorf("[Weixin WS] GetUpdates error: %v", err)

				// Backoff on error
				select {
				case <-ctx.Done():
					return
				case <-h.stopChan:
					return
				case <-time.After(backoff):
					backoff = backoff * 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}
			}

			// 检查 session timeout
			if resp.ErrCode == -14 {
				logger.GetRunnerLogger().Error("[Weixin WS] Session expired, need to re-login")
				return
			}

			// 重置 backoff
			backoff = time.Second

			// 更新 sync cursor
			if resp.GetUpdatesBuf != "" {
				getUpdatesBuf = resp.GetUpdatesBuf
			}

			// 处理消息
			for _, msg := range resp.Msgs {
				if err := h.handleInboundMessage(ctx, msg); err != nil {
					logger.GetRunnerLogger().Errorf("[Weixin WS] Handle message error: %v", err)
				}
			}
		}
	}
}

// handleInboundMessage 处理接收到的消息
func (h *WsHandler) handleInboundMessage(ctx context.Context, msg *WeixinMessage) error {
	// 存储 context token 用于回复
	if msg.ContextToken != "" {
		key := h.contextTokenKey(msg.FromUserID)
		h.contextTokens.Store(key, msg.ContextToken)
		logger.GetRunnerLogger().Infof("[Weixin WS] Stored contextToken for %s: %s", msg.FromUserID, msg.ContextToken)
	} else {
		logger.GetRunnerLogger().Warnf("[Weixin WS] No contextToken in message from %s, msg_id=%d", msg.FromUserID, msg.MessageID)
	}

	// 提取文本内容
	content := h.extractContent(msg)
	if content == "" {
		logger.GetRunnerLogger().Debug("[Weixin WS] Empty message content, ignoring")
		return nil
	}

	logger.GetRunnerLogger().Infof("[Weixin WS] Received message: from=%s, msg_id=%d, content=%s",
		msg.FromUserID, msg.MessageID, content)

	// 构建 MessageContext
	wsCtx := &MessageContext{
		ChannelCode: "weixin",
		SessionID:   msg.FromUserID, // 使用 from_user_id 作为 session_id
		UserID:      msg.FromUserID,
		Content:     content,
		Header: map[string]string{
			"message_id":    string(rune(msg.MessageID)),
			"session_id":    msg.SessionID,
			"message_type":  string(rune(msg.MessageType)),
			"context_token": msg.ContextToken,
		},
	}

	// 调用消息处理回调
	if h.onMessage != nil {
		return h.onMessage(wsCtx)
	}

	return nil
}

// extractContent 从消息中提取文本内容
func (h *WsHandler) extractContent(msg *WeixinMessage) string {
	var parts []string

	for _, item := range msg.ItemList {
		if item.Type == MessageItemTypeText && item.TextItem != nil {
			parts = append(parts, item.TextItem.Text)
		}
	}

	return strings.Join(parts, "\n")
}

// contextTokenKey 生成 context token 的存储 key
func (h *WsHandler) contextTokenKey(userID string) string {
	return h.accountID + ":" + userID
}

// getContextToken 获取用户的 context token
func (h *WsHandler) getContextToken(userID string) string {
	key := h.contextTokenKey(userID)
	if v, ok := h.contextTokens.Load(key); ok {
		return v.(string)
	}
	return ""
}

// Stop 停止长轮询
func (h *WsHandler) Stop() error {
	close(h.stopChan)
	return nil
}

// SendText 发送文本消息
func (h *WsHandler) SendText(ctx context.Context, receiveID, msgType, content string) error {
	// 先调用 GetConfig 获取 typing_ticket
	config, err := h.client.GetConfig(ctx, receiveID, "")
	if err != nil {
		logger.GetRunnerLogger().Warnf("[Weixin WS] GetConfig error: %v", err)
	} else {
		logger.GetRunnerLogger().Infof("[Weixin WS] GetConfig ok, typing_ticket=%s", config.TypingTicket)
		// 发送 typing 状态
		if config.TypingTicket != "" {
			if err := h.client.SendTyping(ctx, receiveID, config.TypingTicket, TypingStatusTyping); err != nil {
				logger.GetRunnerLogger().Warnf("[Weixin WS] SendTyping error: %v", err)
			} else {
				logger.GetRunnerLogger().Infof("[Weixin WS] SendTyping ok")
			}
		}
	}

	// 获取存储的 context_token
	contextToken := h.getContextToken(receiveID)
	if contextToken == "" {
		logger.GetRunnerLogger().Warnf("[Weixin WS] No contextToken found for %s, message may fail", receiveID)
	}

	// 生成 client_id
	clientID := generateClientID()

	logger.GetRunnerLogger().Infof("[Weixin WS] Sending message to %s (length=%d), contextToken=%s, clientID=%s",
		receiveID, len(content), contextToken, clientID)

	// 构建消息
	item := MessageItem{
		Type: MessageItemTypeText,
		TextItem: &TextItem{
			Text: content,
		},
	}

	req := &SendMessageReq{
		ToUserID:     receiveID,
		ClientID:     clientID,
		MessageType:  MessageTypeBot,  // 2 = Bot
		MessageState: MessageStateFinish, // 2 = Finish
		ContextToken: contextToken,
		ItemList:     []MessageItem{item},
	}

	if err := h.client.SendMessage(ctx, req); err != nil {
		logger.GetRunnerLogger().Errorf("[Weixin WS] Failed to send message: %v", err)
		return err
	}

	logger.GetRunnerLogger().Infof("[Weixin WS] Message sent to %s", receiveID)
	return nil
}

// generateClientID 生成客户端ID
func generateClientID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("go-%x", b)
}

// IsLoggedIn 检查是否已登录
func (h *WsHandler) IsLoggedIn() bool {
	tokenInfo, err := h.auth.LoadToken()
	if err != nil || tokenInfo == nil {
		return false
	}
	return h.auth.IsTokenValid(tokenInfo)
}

// NeedQRLogin 检查是否需要二维码登录
func (h *WsHandler) NeedQRLogin() bool {
	return !h.IsLoggedIn()
}

// GetQRCode 获取登录二维码图片（base64 data URL）
func (h *WsHandler) GetQRCode(ctx context.Context) (string, error) {
	img, _, err := h.auth.GetQRCodeImage(ctx)
	return img, err
}

// GetQRCodeWithData 获取登录二维码图片和原始数据字符串
// 返回 (base64图片, qrcode字符串, error)
func (h *WsHandler) GetQRCodeWithData(ctx context.Context) (string, string, error) {
	return h.auth.GetQRCodeImage(ctx)
}

// CompleteQRLogin 完成二维码登录
func (h *WsHandler) CompleteQRLogin(ctx context.Context, qrcode string, onStatus func(status string)) (*TokenInfo, error) {
	tokenInfo, err := h.auth.PollQRCodeStatus(ctx, qrcode, onStatus)
	if err != nil {
		return nil, err
	}

	// 更新 client 的 token
	h.client.SetToken(tokenInfo.Token)

	// 保存 token 到磁盘
	if err := h.auth.SaveToken(tokenInfo); err != nil {
		logger.GetRunnerLogger().Warnf("[Weixin WS] Failed to save token: %v", err)
	}

	logger.GetRunnerLogger().Info("[Weixin WS] QR login completed and token saved")
	return tokenInfo, nil
}
