package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	publicChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// OutboundHandler 飞书出站处理器
type OutboundHandler struct {
	appID        string
	appSecret    string
	token        string
	tokenExpiry  time.Time
	tokenMu      sync.Mutex
	httpClient   *http.Client
}

// NewOutboundHandler 创建飞书出站处理器
func NewOutboundHandler() *OutboundHandler {
	return &OutboundHandler{
		appID:     os.Getenv("FEISHU_APP_ID"),
		appSecret: os.Getenv("FEISHU_APP_SECRET"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewOutboundHandlerWithConfig 创建带配置的飞书出站处理器
func NewOutboundHandlerWithConfig(appID, appSecret string) *OutboundHandler {
	return &OutboundHandler{
		appID:     appID,
		appSecret: appSecret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetChannelCode 获取渠道代码
func (h *OutboundHandler) GetChannelCode() string {
	return "feishu"
}

// getTenantAccessToken 获取 tenant access token（带缓存）
func (h *OutboundHandler) getTenantAccessToken(ctx context.Context) (string, error) {
	h.tokenMu.Lock()
	defer h.tokenMu.Unlock()

	// 检查缓存的 token 是否有效（提前 5 分钟过期）
	if h.token != "" && time.Now().Before(h.tokenExpiry.Add(-5*time.Minute)) {
		return h.token, nil
	}

	// 获取新 token
	reqBody := map[string]string{
		"app_id":     h.appID,
		"app_secret": h.appSecret,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", errors.New("failed to get token: " + result.Msg)
	}

	h.token = result.TenantAccessToken
	h.tokenExpiry = time.Now().Add(time.Duration(result.Expire) * time.Second)

	return h.token, nil
}

// SendText 发送文本消息到飞书
func (h *OutboundHandler) SendText(ctx publicChannel.OutboundContext, text string) error {
	callback, ok := ctx.GetRequest().(FeishuCallback)
	if !ok {
		return errors.New("invalid request type")
	}

	token, err := h.getTenantAccessToken(context.Background())
	if err != nil {
		return err
	}

	// 获取接收者 ID
	receiveID := callback.Event.Sender.SenderID.OpenID
	if receiveID == "" {
		receiveID = callback.Event.Sender.SenderID.UserID
	}

	// 准备消息内容
	content := map[string]string{"text": text}
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return err
	}

	// 构建请求体
	reqBody := map[string]any{
		"receive_id":      receiveID,
		"msg_type":         "text",
		"content":          string(contentBytes),
	}

	// 根据 ID 类型设置 receive_id_type
	if callback.Event.Sender.SenderID.OpenID != "" {
		reqBody["receive_id_type"] = "open_id"
	} else if callback.Event.Sender.SenderID.UserID != "" {
		reqBody["receive_id_type"] = "user_id"
	} else {
		reqBody["receive_id_type"] = "union_id"
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		logger.GetRunnerLogger().Errorf("Feishu send failed: status=%d, result=%v", resp.StatusCode, result)
		return fmt.Errorf("failed to send message: %v", result)
	}

	return nil
}

// SendRichText 发送富文本消息（暂不支持）
func (h *OutboundHandler) SendRichText(ctx publicChannel.OutboundContext, content any) error {
	// TODO: 实现飞书富文本消息（post、卡片等）
	return errors.New("rich text not implemented yet")
}

// SendStream 发送流式消息（飞书不支持 SSE）
func (h *OutboundHandler) SendStream(ctx publicChannel.OutboundContext, reader io.Reader) error {
	// 飞书不支持 SSE，这里先读取所有内容然后发送
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return h.SendText(ctx, string(data))
}

// SendError 发送错误消息
func (h *OutboundHandler) SendError(ctx publicChannel.OutboundContext, err error) {
	errMsg := "Sorry, an error occurred. Please try again later."
	if err != nil {
		errMsg = "Error: " + err.Error()
	}
	if sendErr := h.SendText(ctx, errMsg); sendErr != nil {
		logger.GetRunnerLogger().WithError(sendErr).Errorf("Failed to send error message to Feishu")
	}
}

// Ack 确认消息收到（飞书不需要显式确认）
func (h *OutboundHandler) Ack(ctx publicChannel.OutboundContext) error {
	// 飞书的事件回调不需要显式确认，直接返回 200 即可
	// Dispatcher 已经处理了响应
	return nil
}
