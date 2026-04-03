package feishu

import "os"

// FeishuCallback 飞书事件回调结构
type FeishuCallback struct {
	Header struct {
		EventID    string `json:"event_id"`
		EventType string `json:"event_type"` // im.message.receive_v1
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				OpenID  string `json:"open_id"`
				UserID  string `json:"user_id"`
				UnionID string `json:"union_id"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"` // user, bot
			TenantKey  string `json:"tenant_key"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			CreateTime  string `json:"create_time"`
			ChatID      string `json:"chat_id"`
			Text        string `json:"text"`
			Content     string `json:"content"` // JSON string: {"text": "hello"}
			MessageType string `json:"msg_type"`
		} `json:"message"`
	} `json:"event"`
}

// FeishuMessageContent 解析后的消息内容
type FeishuMessageContent struct {
	Text string `json:"text"`
}

// FeishuConfig 飞书渠道配置
type FeishuConfig struct {
	AppID     string `json:"app_id"`
	BotName   string `json:"bot_name"`
}

// GetAppSecret 获取 AppSecret（从环境变量）
func (c *FeishuConfig) GetAppSecret() string {
	return os.Getenv("FEISHU_APP_SECRET")
}

// GetEncryptKey 获取 EncryptKey（从环境变量）
func (c *FeishuConfig) GetEncryptKey() string {
	return os.Getenv("FEISHU_ENCRYPT_KEY")
}
