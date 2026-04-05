package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// MessageContext WebSocket 消息上下文
type MessageContext struct {
	ChannelCode string            // 渠道代码
	SessionID   string            // 会话ID (chat_id)
	UserID      string            // 用户ID (sender open_id)
	Content     string            // 消息内容
	Header      map[string]string // 额外头部信息
}

// WsHandler 飞书 WebSocket 长连接处理器
// 通过 WebSocket 与飞书服务器保持持久连接，接收消息推送
type WsHandler struct {
	appID             string
	appSecret         string
	domain            string // 飞书域名: "lark" 或 "feishu"
	verificationToken string
	encryptKey        string

	wsClient        *larkws.Client
	eventDispatcher *dispatcher.EventDispatcher
	httpClient      *lark.Client

	// 回调配置
	onMessage func(ctx *MessageContext) error

	mu sync.RWMutex
}

// WsHandlerConfig WebSocket 处理器配置
type WsHandlerConfig struct {
	AppID             string
	AppSecret         string
	Domain            string // "lark" 或 "feishu"
	VerificationToken string
	EncryptKey        string
	OnMessage         func(ctx *MessageContext) error // 消息回调
}

// NewWsHandler 创建 WebSocket 处理器
func NewWsHandler(cfg WsHandlerConfig) *WsHandler {
	if cfg.Domain == "" {
		cfg.Domain = "lark"
	}

	// 创建 HTTP client for sending messages
	httpClient := lark.NewClient(
		cfg.AppID,
		cfg.AppSecret,
		lark.WithAppType(larkcore.AppTypeSelfBuilt),
		lark.WithOpenBaseUrl(resolveDomain(cfg.Domain)),
	)

	h := &WsHandler{
		appID:             cfg.AppID,
		appSecret:         cfg.AppSecret,
		domain:            cfg.Domain,
		verificationToken: cfg.VerificationToken,
		encryptKey:        cfg.EncryptKey,
		httpClient:        httpClient,
		onMessage:         cfg.OnMessage,
	}

	// 创建事件分发器
	h.eventDispatcher = dispatcher.NewEventDispatcher(
		h.verificationToken,
		h.encryptKey,
	)

	// 注册事件处理器
	h.registerEventHandlers()

	return h
}

// resolveDomain 解析域名
func resolveDomain(domain string) string {
	if domain == "lark" {
		return lark.LarkBaseUrl
	}
	return lark.FeishuBaseUrl
}

// Start 启动 WebSocket 连接
func (h *WsHandler) Start(ctx context.Context) error {
	logger.GetRunnerLogger().Infof("[Feishu WS] Starting WebSocket connection, app_id=%s, domain=%s", h.appID, h.domain)

	// 创建 WebSocket 客户端
	h.wsClient = larkws.NewClient(
		h.appID,
		h.appSecret,
		larkws.WithEventHandler(h.eventDispatcher),
		larkws.WithDomain(resolveDomain(h.domain)),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	// 启动 WebSocket 连接（会阻塞）
	go h.startWebSocket(ctx)

	return nil
}

// startWebSocket 启动 WebSocket 连接
func (h *WsHandler) startWebSocket(ctx context.Context) {
	logger.GetRunnerLogger().Info("[Feishu WS] Starting Feishu WebSocket connection")

	// Start blocks forever, wsClient 会自动处理重连
	if err := h.wsClient.Start(ctx); err != nil {
		logger.GetRunnerLogger().Errorf("[Feishu WS] WebSocket error: %v", err)
	}

	logger.GetRunnerLogger().Info("[Feishu WS] Feishu WebSocket connection stopped")
}

// Stop 停止 WebSocket 连接
// 注意：ws.Client 不支持主动关闭，只能通过取消 context 来停止
func (h *WsHandler) Stop() error {
	// ws.Client.Start() 会阻塞直到 context 被取消
	// 调用者应该取消传递给 Start() 的 context
	return nil
}

// registerEventHandlers 注册事件处理器
func (h *WsHandler) registerEventHandlers() {
	// 注册消息接收处理器
	h.eventDispatcher.OnP2MessageReceiveV1(h.handleMessageReceived)
}

// handleMessageReceived 处理接收到的消息
func (h *WsHandler) handleMessageReceived(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Sender == nil || event.Event.Message == nil {
		logger.GetRunnerLogger().Debug("[Feishu WS] Message event has nil fields")
		return nil
	}

	// 提取关键信息
	senderID := ""
	if event.Event.Sender.SenderId != nil {
		if event.Event.Sender.SenderId.OpenId != nil {
			senderID = *event.Event.Sender.SenderId.OpenId
		} else if event.Event.Sender.SenderId.UserId != nil {
			senderID = *event.Event.Sender.SenderId.UserId
		}
	}

	chatID := ""
	if event.Event.Message.ChatId != nil {
		chatID = *event.Event.Message.ChatId
	}

	messageID := ""
	if event.Event.Message.MessageId != nil {
		messageID = *event.Event.Message.MessageId
	}

	chatType := "unknown"
	if event.Event.Message.ChatType != nil {
		chatType = *event.Event.Message.ChatType
	}

	messageType := "unknown"
	if event.Event.Message.MessageType != nil {
		messageType = *event.Event.Message.MessageType
	}

	messageContent := ""
	if event.Event.Message.Content != nil {
		messageContent = *event.Event.Message.Content
	}

	logger.GetRunnerLogger().Infof("[Feishu WS] Received message: chat_id=%s, chat_type=%s, sender_id=%s, msg_type=%s, content=%s",
		chatID, chatType, senderID, messageType, messageContent)

	// 跳过 bot 自己的消息
	if event.Event.Sender.SenderType != nil && *event.Event.Sender.SenderType == "bot" {
		logger.GetRunnerLogger().Debug("[Feishu WS] Skipping bot message")
		return nil
	}

	// 使用 message_id 做去重，避免 Feishu 重试导致重复处理
	if messageID != "" {
		h.mu.Lock()
		if _, seen := h.processedMsgs[messageID]; seen {
			h.mu.Unlock()
			logger.GetRunnerLogger().Infof("[Feishu WS] Skipping duplicate message: %s", messageID)
			return nil
		}
		h.processedMsgs[messageID] = true
		// 限制去重缓存大小
		if len(h.processedMsgs) > 1000 {
			h.processedMsgs = make(map[string]bool)
		}
		h.mu.Unlock()
	}

	// 解析消息内容
	content, _ := h.extractMessageContent(event.Event.Message)
	if content == "" {
		logger.GetRunnerLogger().Debug("[Feishu WS] Empty message content")
		return nil
	}

	// 构建 MessageContext
	wsCtx := &MessageContext{
		ChannelCode: "feishu",
		SessionID:   chatID,
		UserID:      senderID,
		Content:     content,
		Header: map[string]string{
			"message_id": messageID,
			"chat_id":    chatID,
			"chat_type":  chatType,
		},
	}

	// 调用消息处理回调
	if h.onMessage != nil {
		return h.onMessage(wsCtx)
	}

	return nil
}

// extractMessageContent 从消息中提取文本内容
func (h *WsHandler) extractMessageContent(msg *larkim.EventMessage) (string, error) {
	if msg.Content == nil {
		return "", nil
	}

	contentRaw := *msg.Content

	msgType := "text"
	if msg.MessageType != nil {
		msgType = *msg.MessageType
	}

	switch msgType {
	case "text":
		var content map[string]string
		if err := json.Unmarshal([]byte(contentRaw), &content); err != nil {
			return "", err
		}
		return content["text"], nil

	case "post":
		// 富文本消息，简化处理
		return "[富文本消息]", nil

	default:
		return contentRaw, nil
	}
}

// SendText 通过 WebSocket 发送文本消息
func (h *WsHandler) SendText(ctx context.Context, receiveID, msgType, content string) error {
	logger.GetRunnerLogger().Infof("[Feishu WS] Sending message to %s: %s", receiveID, content)

	// 飞书文本消息的 content 必须是 JSON 字符串格式: {"text":"..."}
	jsonContent := map[string]string{"text": content}
	contentBytes, err := json.Marshal(jsonContent)
	if err != nil {
		logger.GetRunnerLogger().Errorf("[Feishu WS] Failed to marshal content: %v", err)
		return err
	}
	jsonStr := string(contentBytes)

	body := &larkim.CreateMessageReqBody{
		ReceiveId: &receiveID,
		MsgType:   &msgType,
		Content:   &jsonStr,
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(body).
		Build()

	resp, err := h.httpClient.Im.Message.Create(ctx, req)
	if err != nil {
		logger.GetRunnerLogger().Errorf("[Feishu WS] Failed to send message: %v", err)
		return err
	}

	if !resp.Success() {
		logger.GetRunnerLogger().Errorf("[Feishu WS] Send message failed: code=%d, msg=%s", resp.Code, resp.Msg)
		return fmt.Errorf("send failed: %s", resp.Msg)
	}

	return nil
}
