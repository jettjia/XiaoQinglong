package channel

import (
	"io"

	"github.com/gin-gonic/gin"
)

// OutboundContext 出站上下文接口（用于避免循环依赖）
type OutboundContext interface {
	GetChannelCode() string
	GetRequest() any
}

// WebhookDispatcher Webhook调度器接口
type WebhookDispatcher interface {
	HandleCallback() gin.HandlerFunc
}

// MessageContext WS消息上下文
type MessageContext struct {
	ChannelCode string            // 渠道代码
	SessionID   string            // 会话ID (chat_id)
	UserID      string            // 用户ID (sender open_id)
	Content     string            // 消息内容
	Header      map[string]string // 额外头部信息
}

// ChannelContext 统一渠道上下文
type ChannelContext struct {
	ChannelCode string            // "web", "feishu", "wechat"
	ChannelID   string            // sys_channel.ulid
	SessionID   string            // 会话ID
	UserID      string            // 用户ID
	AgentID     string            // Agent ID
	Config      map[string]any    // 渠道配置（从 sys_channel 获取）
	Request     any               // 原始请求对象（由 InboundHandler 解析）
	Header      map[string]string // HTTP Header
}

// GetChannelCode 获取渠道代码
func (c *ChannelContext) GetChannelCode() string {
	return c.ChannelCode
}

// GetRequest 获取原始请求
func (c *ChannelContext) GetRequest() any {
	return c.Request
}

// ChatRunReq 统一聊天请求
type ChatRunReq struct {
	AgentID   string     `json:"agent_id"`
	UserID    string     `json:"user_id"`
	SessionID string     `json:"session_id"`
	Input     string     `json:"input"`
	Files     []FileInfo `json:"files"`
	IsTest    bool       `json:"is_test"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// InboundHandler 入站消息处理接口
type InboundHandler interface {
	// GetChannelCode 获取支持的渠道代码
	GetChannelCode() string

	// Validate 验证请求合法性（签名、token等）
	Validate(c *gin.Context) error

	// ParseRequest 解析请求，返回 ChannelContext
	ParseRequest(c *gin.Context) (*ChannelContext, error)
}

// OutboundHandler 出站消息处理接口
type OutboundHandler interface {
	// GetChannelCode 获取支持的渠道代码
	GetChannelCode() string

	// SendText 发送文本消息
	SendText(ctx OutboundContext, text string) error

	// SendRichText 发送富文本消息（markdown, cards等）
	SendRichText(ctx OutboundContext, content any) error

	// SendStream 发送流式消息（SSE等）
	SendStream(ctx OutboundContext, reader io.Reader) error

	// SendError 发送错误消息
	SendError(ctx OutboundContext, err error)

	// Ack 确认消息收到（用于 webhook 类渠道）
	Ack(ctx OutboundContext) error
}
