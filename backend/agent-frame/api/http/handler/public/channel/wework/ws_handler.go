package wework

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// MessageContext WebSocket 消息上下文
type MessageContext struct {
	ChannelCode string            // 渠道代码
	SessionID   string            // 会话ID (msg_id / chat_id)
	UserID      string            // 用户ID (from_user_id)
	Content     string            // 消息内容
	Header      map[string]string // 额外头部信息
}

// WsHandler 企业微信 WebSocket 长连接处理器
type WsHandler struct {
	botID   string
	secret  string
	url     string

	conn *websocket.Conn

	// 回调配置
	onMessage func(ctx *MessageContext) error

	// 待回复消息存储: msgID -> msgInfo
	waitResponseMsg map[string]weworkWsBotMsgInfo
	mu              sync.RWMutex

	// 停止通道
	stopChan chan struct{}
}

// WsHandlerConfig WebSocket 处理器配置
type WsHandlerConfig struct {
	BotID    string
	Secret   string
	URL      string
	OnMessage func(ctx *MessageContext) error // 消息回调
}

// weworkWsBotMsgInfo 消息信息
type weworkWsBotMsgInfo struct {
	ReqID      string
	MsgTime    int64
	FromUserID string
	ChatID     string
	StreamID   string
}

// NewWsHandler 创建 WebSocket 处理器
func NewWsHandler(cfg WsHandlerConfig) *WsHandler {
	if cfg.URL == "" {
		cfg.URL = "wss://openws.work.weixin.qq.com"
	}

	h := &WsHandler{
		botID:          cfg.BotID,
		secret:         cfg.Secret,
		url:            cfg.URL,
		onMessage:      cfg.OnMessage,
		waitResponseMsg: make(map[string]weworkWsBotMsgInfo),
		stopChan:       make(chan struct{}),
	}

	return h
}

// Start 启动 WebSocket 连接
func (h *WsHandler) Start(ctx context.Context) error {
	logger.GetRunnerLogger().Infof("[WeWork WS] Starting WebSocket connection, bot_id=%s, url=%s", h.botID, h.url)

	if err := h.doConnect(ctx); err != nil {
		return err
	}

	// 启动消息处理循环
	go h.handleMessages(ctx)

	return nil
}

// doConnect 执行实际的连接
func (h *WsHandler) doConnect(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, h.url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	h.conn = conn

	logger.GetRunnerLogger().Info("[WeWork WS] WebSocket connected")

	// 发送订阅请求
	subscribe := weworkWsBotRequest{
		Cmd: "aibot_subscribe",
		Header: map[string]string{
			"req_id": uuid.New().String(),
		},
		Body: weworkWsBotSubscribeBody{
			BotID: h.botID,
			Secret: h.secret,
		},
	}

	return conn.WriteJSON(subscribe)
}

// handleMessages 处理消息循环
func (h *WsHandler) handleMessages(ctx context.Context) {
	defer func() {
		logger.GetRunnerLogger().Warn("[WeWork WS] WebSocket disconnected")
	}()

	// 心跳定时器 (30秒)
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// 消息通道
	messageChan := make(chan []byte, 100)
	errorChan := make(chan error, 1)

	// 读取消息 goroutine
	go func() {
		for {
			if h.conn == nil {
				return
			}
			_, message, err := h.conn.ReadMessage()
			if err != nil {
				errorChan <- err
				return
			}
			messageChan <- message
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		case <-heartbeatTicker.C:
			h.sendHeartbeat()
		case message := <-messageChan:
			h.handleMessage(message)
		case err := <-errorChan:
			logger.GetRunnerLogger().Warnf("[WeWork WS] WebSocket read error: %v", err)
			return
		}
	}
}

// sendHeartbeat 发送心跳
func (h *WsHandler) sendHeartbeat() {
	if h.conn != nil {
		h.conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(
			"{\"cmd\":\"ping\",\"headers\":{\"req_id\":\"%s\"}}", uuid.New().String())))
	}

	// 清理过期的待回复消息 (超过24小时)
	h.mu.Lock()
	var removeKey []string
	now := time.Now().Unix()
	for k, v := range h.waitResponseMsg {
		if v.MsgTime == 0 || now-v.MsgTime > 24*60*60 {
			removeKey = append(removeKey, k)
		}
	}
	for _, k := range removeKey {
		delete(h.waitResponseMsg, k)
	}
	h.mu.Unlock()
}

// Stop 停止 WebSocket 连接
func (h *WsHandler) Stop() error {
	close(h.stopChan)
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

// handleMessage 处理接收到的消息
func (h *WsHandler) handleMessage(data []byte) {
	logger.GetRunnerLogger().Debugf("[WeWork WS] Received message: %s", string(data))

	var resp weworkWsBotResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		logger.GetRunnerLogger().Warnf("[WeWork WS] Unmarshal error: %v", err)
		return
	}

	if resp.Cmd == "" {
		logger.GetRunnerLogger().Debug("[WeWork WS] Received message with no cmd")
		return
	}

	msg := resp.Body
	reqID := resp.Header["req_id"]

	// 存储消息信息，用于后续回复
	h.mu.Lock()
	h.waitResponseMsg[msg.MsgID] = weworkWsBotMsgInfo{
		ReqID:      reqID,
		MsgTime:    time.Now().Unix(),
		FromUserID: msg.From.UserID,
		ChatID:     msg.ChatID,
		StreamID:   uuid.New().String(),
	}
	h.mu.Unlock()

	// 只处理文本消息
	if msg.MsgType == "text" {
		logger.GetRunnerLogger().Infof("[WeWork WS] Received text message: msg_id=%s, from=%s, content=%s",
			msg.MsgID, msg.From.UserID, msg.Text.Content)

		// 构建 MessageContext
		wsCtx := &MessageContext{
			ChannelCode: "wework",
			SessionID:   msg.MsgID, // 使用 msg_id 作为 session_id
			UserID:      msg.From.UserID,
			Content:     msg.Text.Content,
			Header: map[string]string{
				"req_id":   reqID,
				"chat_id":  msg.ChatID,
				"from":     msg.From.UserID,
				"stream_id": h.waitResponseMsg[msg.MsgID].StreamID,
			},
		}

		// 调用消息处理回调
		if h.onMessage != nil {
			if err := h.onMessage(wsCtx); err != nil {
				logger.GetRunnerLogger().Errorf("[WeWork WS] onMessage error: %v", err)
			}
		}
	} else {
		logger.GetRunnerLogger().Infof("[WeWork WS] Received non-text message: msg_type=%s", msg.MsgType)
		// 不支持的消息类型，直接回复错误
		h.sendTextReply(msg.MsgID, "不支持["+msg.MsgType+"]类型消息")
	}
}

// SendText 通过 WebSocket 发送文本消息
func (h *WsHandler) SendText(ctx context.Context, receiveID, msgType, content string) error {
	logger.GetRunnerLogger().Infof("[WeWork WS] Sending message to %s (length=%d)", receiveID, len(content))

	// 先检查是否有待回复的消息（被动回复）
	h.mu.RLock()
	v, hasReply := h.waitResponseMsg[receiveID]
	h.mu.RUnlock()

	if hasReply && v.MsgTime > 0 {
		// 被动回复模式：有收到的消息需要回复
		return h.sendTextReply(receiveID, content)
	}

	// 主动推送模式：没有收到消息，用主动推送
	return h.sendTextPush(receiveID, content)
}

// sendTextReply 被动回复（收到消息后回复）
func (h *WsHandler) sendTextReply(msgID, content string) error {
	h.mu.RLock()
	v := h.waitResponseMsg[msgID]
	h.mu.RUnlock()

	if v.ReqID == "" {
		return fmt.Errorf("msg not found or expired: %s", msgID)
	}

	resp := weworkWsBotRequest{
		Cmd: "aibot_respond_msg",
		Header: map[string]string{
			"req_id": v.ReqID,
		},
		Body: weworkWsBotRespondBody{
			MsgType: "stream",
			Stream: weworkWsBotStreamBody{
				ID:      v.StreamID,
				Finish:  true,
				Content: content,
			},
		},
	}

	return h.sendMessage(resp)
}

// sendTextPush 主动推送
func (h *WsHandler) sendTextPush(chatID, content string) error {
	resp := weworkWsBotRequest{
		Cmd: "aibot_send_msg",
		Header: map[string]string{
			"req_id": uuid.New().String(),
		},
		Body: weworkWsBotPushBody{
			MsgType: "markdown",
			ChatID:  chatID,
			Markdown: weworkWsBotMarkdownBody{
				Content: content,
			},
		},
	}

	return h.sendMessage(resp)
}

// sendMessage 发送 WebSocket 消息
func (h *WsHandler) sendMessage(v any) error {
	if h.conn == nil {
		return fmt.Errorf("websocket not connected")
	}
	return h.conn.WriteJSON(v)
}

// ================== 企业微信协议类型 ==================

type weworkWsBotRequest struct {
	Cmd    string            `json:"cmd"`
	Header map[string]string `json:"headers"`
	Body   any               `json:"body,omitempty"`
}

type weworkWsBotResponse struct {
	Cmd     string                  `json:"cmd"`
	Header  map[string]string      `json:"headers"`
	ErrCode int                     `json:"errcode"`
	ErrMsg  string                  `json:"errmsg"`
	Body    weworkWsBotResponseData `json:"body"`
}

type weworkWsBotResponseFrom struct {
	UserID string `json:"userid"`
}

type weworkWsBotResponseText struct {
	Content string `json:"content"`
}

type weworkWsBotResponseData struct {
	MsgID    string                    `json:"msgid"`
	AibotID  string                    `json:"aibotid"`
	ChatID   string                    `json:"chatid"`
	ChatType string                    `json:"chattype"`
	From     weworkWsBotResponseFrom    `json:"from"`
	MsgType  string                    `json:"msgtype"`
	Text     weworkWsBotResponseText   `json:"text"`
}

type weworkWsBotSubscribeBody struct {
	BotID  string `json:"bot_id"`
	Secret string `json:"secret"`
}

type weworkWsBotMsgResponse struct {
	MsgType string `json:"msgtype"`
	Stream  struct {
		ID      string `json:"id"`
		Finish  bool   `json:"finish"`
		Content string `json:"content"`
	} `json:"stream"`
}

type weworkWsBotMsgPushData struct {
	ChatID   string `json:"chatid"`
	MsgType  string `json:"msgtype"`
	Markdown struct {
		Content string `json:"content"`
	} `json:"markdown"`
}

// 回复消息体
type weworkWsBotRespondBody struct {
	MsgType string            `json:"msgtype"`
	Stream  weworkWsBotStreamBody `json:"stream"`
}

type weworkWsBotStreamBody struct {
	ID      string `json:"id"`
	Finish  bool   `json:"finish"`
	Content string `json:"content"`
}

// 推送消息体
type weworkWsBotPushBody struct {
	MsgType  string                 `json:"msgtype"`
	ChatID   string                 `json:"chatid"`
	Markdown weworkWsBotMarkdownBody `json:"markdown"`
}

type weworkWsBotMarkdownBody struct {
	Content string `json:"content"`
}