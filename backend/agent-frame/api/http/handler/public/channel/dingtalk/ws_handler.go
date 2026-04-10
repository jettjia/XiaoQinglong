package dingtalk

import (
	"context"
	"sync"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// MessageContext WebSocket 消息上下文
type MessageContext struct {
	ChannelCode string            // 渠道代码
	SessionID   string            // 会话ID (chat_id/conversation_id)
	UserID      string            // 用户ID (sender_staff_id)
	Content     string            // 消息内容
	Header      map[string]string // 额外头部信息
}

// WsHandler 钉钉 WebSocket 长连接处理器
// 通过 WebSocket 与钉钉服务器保持持久连接，接收消息推送
type WsHandler struct {
	clientID     string
	clientSecret string

	streamClient *client.StreamClient

	// 回调配置
	onMessage func(ctx *MessageContext) error

	// 存储 session webhook: chatID -> sessionWebhook
	sessionWebhooks sync.Map
}

// WsHandlerConfig WebSocket 处理器配置
type WsHandlerConfig struct {
	ClientID     string
	ClientSecret string
	OnMessage    func(ctx *MessageContext) error // 消息回调
}

// NewWsHandler 创建 WebSocket 处理器
func NewWsHandler(cfg WsHandlerConfig) *WsHandler {
	h := &WsHandler{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		onMessage:    cfg.OnMessage,
	}

	// 创建凭证配置
	cred := client.NewAppCredentialConfig(cfg.ClientID, cfg.ClientSecret)

	// 创建 stream client
	h.streamClient = client.NewStreamClient(
		client.WithAppCredential(cred),
		client.WithAutoReconnect(true),
	)

	// 注册 chatbot 回调处理器
	h.streamClient.RegisterChatBotCallbackRouter(h.onChatBotMessageReceived)

	return h
}

// Start 启动 WebSocket 连接
func (h *WsHandler) Start(ctx context.Context) error {
	logger.GetRunnerLogger().Infof("[DingTalk WS] Starting WebSocket connection, client_id=%s", h.clientID)

	// 启动 WebSocket 连接（会阻塞）
	go h.startWebSocket(ctx)

	return nil
}

// startWebSocket 启动 WebSocket 连接
func (h *WsHandler) startWebSocket(ctx context.Context) {
	logger.GetRunnerLogger().Info("[DingTalk WS] Starting DingTalk WebSocket connection")

	// Start blocks forever，streamClient 会自动处理重连
	if err := h.streamClient.Start(ctx); err != nil {
		logger.GetRunnerLogger().Errorf("[DingTalk WS] WebSocket error: %v", err)
	}

	logger.GetRunnerLogger().Info("[DingTalk WS] DingTalk WebSocket connection stopped")
}

// Stop 停止 WebSocket 连接
func (h *WsHandler) Stop() error {
	if h.streamClient != nil {
		h.streamClient.Close()
	}
	return nil
}

// onChatBotMessageReceived 处理钉钉机器人消息
func (h *WsHandler) onChatBotMessageReceived(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	// 提取消息内容
	content := data.Text.Content
	if content == "" {
		// 尝试从 Content interface{} 提取
		if contentMap, ok := data.Content.(map[string]interface{}); ok {
			if textContent, ok := contentMap["content"].(string); ok {
				content = textContent
			}
		}
	}

	if content == "" {
		logger.GetRunnerLogger().Debug("[DingTalk WS] Empty message content, ignoring")
		return nil, nil
	}

	senderID := data.SenderStaffId
	senderNick := data.SenderNick

	// 确定 chatID：私聊用 senderID，群聊用 conversationID
	chatID := senderID
	if data.ConversationType != "1" {
		// "1" = 私聊，"2" = 群聊
		chatID = data.ConversationId
	}

	logger.GetRunnerLogger().Infof("[DingTalk WS] Received message: chat_id=%s, conversation_type=%s, sender_id=%s, sender_nick=%s, content=%s",
		chatID, data.ConversationType, senderID, senderNick, content)

	// 存储 session webhook，以便后续回复
	h.sessionWebhooks.Store(chatID, data.SessionWebhook)

	// 构建 MessageContext
	wsCtx := &MessageContext{
		ChannelCode: "dingtalk",
		SessionID:   chatID,
		UserID:      senderID,
		Content:     content,
		Header: map[string]string{
			"conversation_id":   data.ConversationId,
			"conversation_type": data.ConversationType,
			"sender_nick":       senderNick,
			"session_webhook":   data.SessionWebhook,
		},
	}

	// 调用消息处理回调
	if h.onMessage != nil {
		if err := h.onMessage(wsCtx); err != nil {
			logger.GetRunnerLogger().Errorf("[DingTalk WS] onMessage error: %v", err)
		}
	}

	// 返回 nil 表示异步处理消息
	return nil, nil
}

// SendText 通过 WebSocket 发送文本消息（使用 markdown 格式）
func (h *WsHandler) SendText(ctx context.Context, receiveID, msgType, content string) error {
	logger.GetRunnerLogger().Infof("[DingTalk WS] Sending message to %s (length=%d)", receiveID, len(content))

	// 获取 session webhook
	webhookRaw, ok := h.sessionWebhooks.Load(receiveID)
	if !ok {
		logger.GetRunnerLogger().Errorf("[DingTalk WS] No session_webhook found for chat %s", receiveID)
		return nil
	}

	webhook, ok := webhookRaw.(string)
	if !ok {
		logger.GetRunnerLogger().Errorf("[DingTalk WS] Invalid session_webhook type for chat %s", receiveID)
		return nil
	}

	// 使用 chatbot replier 发送 markdown 消息
	replier := chatbot.NewChatbotReplier()
	err := replier.SimpleReplyMarkdown(
		ctx,
		webhook,
		[]byte("xiaoqinglong"),
		[]byte(content),
	)

	if err != nil {
		logger.GetRunnerLogger().Errorf("[DingTalk WS] Failed to send message: %v", err)
		return err
	}

	return nil
}
