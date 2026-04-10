package feishu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// 确保 Handler 实现 channel.InboundHandler 接口
var _ publicChannel.InboundHandler = (*Handler)(nil)

// Handler 飞书入站处理器（Webhook 模式）
type Handler struct {
	encryptKey        string
	verificationToken string
}

// NewHandler 创建飞书处理器
func NewHandler() *Handler {
	return &Handler{
		encryptKey:        os.Getenv("FEISHU_ENCRYPT_KEY"),
		verificationToken: os.Getenv("FEISHU_VERIFICATION_TOKEN"),
	}
}

// GetChannelCode 获取渠道代码
func (h *Handler) GetChannelCode() string {
	return "feishu"
}

// Validate 验证飞书请求
// GET 请求时：验证 verification_token（URL 验证）
// POST 请求时：验证 X-Lark-Signature（消息签名）
func (h *Handler) Validate(c *gin.Context) error {
	// GET 请求：飞书验证 URL
	if c.Request.Method == "GET" {
		return h.validateVerification(c)
	}

	// POST 请求：验证消息签名
	return h.validateSignature(c)
}

// validateVerification 验证 URL 验证请求
func (h *Handler) validateVerification(c *gin.Context) error {
	challenge := c.Query("challenge")
	if challenge == "" {
		return errors.New("missing challenge parameter")
	}

	// 验证 token（可选，但推荐验证）
	token := c.Query("token")
	if h.verificationToken != "" && token != h.verificationToken {
		return errors.New("invalid verification token")
	}

	// 飞书要求返回 challenge
	c.JSON(200, gin.H{
		"challenge": challenge,
	})
	return nil
}

// validateSignature 验证消息签名
func (h *Handler) validateSignature(c *gin.Context) error {
	signature := c.GetHeader("X-Lark-Signature")
	if signature == "" {
		return errors.New("missing X-Lark-Signature header")
	}

	// 读取 body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return errors.New("failed to read request body")
	}

	// 重新写入 body 以便后续处理
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

	// 计算期望的签名
	expected := h.computeSignature(body)
	if signature != expected {
		logger.GetRunnerLogger().Warnf("Feishu signature mismatch: got %s, expected %s", signature, expected)
		return errors.New("invalid signature")
	}

	return nil
}

// computeSignature 计算签名
// 签名算法：HMAC-SHA256(encrypt_key, body)，然后 base64 编码
func (h *Handler) computeSignature(body []byte) string {
	if h.encryptKey == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(h.encryptKey))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ParseRequest 解析飞书回调请求
func (h *Handler) ParseRequest(c *gin.Context) (*publicChannel.ChannelContext, error) {
	// GET 请求已经在 Validate 中处理了 URL 验证，直接返回 nil
	if c.Request.Method == "GET" {
		return nil, errors.New("URL verification completed")
	}

	var callback FeishuCallback
	if err := c.ShouldBindJSON(&callback); err != nil {
		return nil, errors.New("failed to parse Feishu callback: " + err.Error())
	}

	// 只处理 im.message.receive_v1 事件
	if callback.Header.EventType != "im.message.receive_v1" {
		return nil, errors.New("unsupported event type: " + callback.Header.EventType)
	}

	// 跳过 bot 自己的消息
	if callback.Event.Sender.SenderType == "bot" {
		return nil, errors.New("skipping bot message")
	}

	// 解析消息内容（Content 字段是 JSON 字符串）
	var msgContent FeishuMessageContent
	contentStr := callback.Event.Message.Content
	if contentStr != "" {
		if err := json.Unmarshal([]byte(contentStr), &msgContent); err != nil {
			// 如果解析失败，可能是纯文本
			msgContent.Text = contentStr
		}
	}

	// 如果 Text 字段有值，优先使用
	if callback.Event.Message.Text != "" {
		msgContent.Text = callback.Event.Message.Text
	}

	if msgContent.Text == "" {
		return nil, errors.New("empty message content")
	}

	return &publicChannel.ChannelContext{
		ChannelCode: "feishu",
		SessionID:   callback.Event.Message.ChatID,
		UserID:      callback.Event.Sender.SenderID.OpenID,
		Request:     callback,
		Header: map[string]string{
			"event_id":   callback.Header.EventID,
			"message_id": callback.Event.Message.MessageID,
			"chat_id":    callback.Event.Message.ChatID,
		},
	}, nil
}
