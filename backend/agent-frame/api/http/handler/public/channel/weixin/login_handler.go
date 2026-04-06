package weixin

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
)

// LoginHandler 微信登录处理器
type LoginHandler struct {
	handler     *WsHandler
	qrcode      string
	loginStatus string
}

// NewLoginHandler 创建登录处理器
func NewLoginHandler() *LoginHandler {
	return &LoginHandler{
		loginStatus: "waiting", // waiting, scanning, confirmed, expired
	}
}

// ensureHandler 确保 handler 已初始化
func (h *LoginHandler) ensureHandler() (*WsHandler, error) {
	manager := GetWsManager()

	if manager.GetHandler() != nil {
		return manager.GetHandler(), nil
	}

	accountID := os.Getenv("WEIXIN_ACCOUNT_ID")
	if accountID == "" {
		accountID = "default"
	}

	logger.GetRunnerLogger().Infof("[Weixin Login] Initializing handler for account: %s", accountID)

	handler, err := NewWsHandler(WsHandlerConfig{
		AccountID: accountID,
		OnMessage: func(ctx *MessageContext) error {
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	manager.SetHandler(handler)
	return handler, nil
}

// Login 扫码登录
// GET /weixin/login
// 返回HTML页面显示二维码，登录成功后自动启动长轮询
func (h *LoginHandler) Login(c *gin.Context) {
	handler, err := h.ensureHandler()
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, `<html><body><h1>Error</h1><p>`+err.Error()+`</p></body></html>`)
		return
	}

	// 已经登录，直接返回成功页面
	if !handler.NeedQRLogin() {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, loginSuccessHTML)
		return
	}

	// 获取二维码图片和原始数据
	qrcodeImg, qrcodeData, err := handler.GetQRCodeWithData(c.Request.Context())
	if err != nil {
		logger.GetRunnerLogger().Errorf("[Weixin Login] Failed to get QR code: %v", err)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, `<html><body><h1>Error</h1><p>failed to get QR code: `+err.Error()+`</p></body></html>`)
		return
	}

	// 保存 handler 和 qrcode 字符串（用于后续 polling）
	h.handler = handler
	h.qrcode = qrcodeData

	// 启动后台登录监控（登录成功/失败都会自动启动长轮询）
	go h.monitorLoginStatus()

	logger.GetRunnerLogger().Info("[Weixin Login] QR code generated, monitoring login status")

	// 返回HTML登录页面（嵌入二维码）
	html := buildLoginHTML(qrcodeImg)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// buildLoginHTML 构建登录页面HTML
func buildLoginHTML(qrcodeImg string) string {
	// 确保二维码是完整的 data URL
	imgSrc := qrcodeImg
	if len(imgSrc) > 50 && !strings.HasPrefix(imgSrc, "data:") {
		imgSrc = "data:image/png;base64," + imgSrc
	}

	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>微信登录 - 青龙</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 400px; margin: 50px auto; text-align: center; }
        .container { padding: 40px; background: #f8f9fa; border-radius: 12px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; margin-bottom: 30px; font-size: 24px; }
        .qrcode { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; display: inline-block; }
        .qrcode img { width: 200px; height: 200px; display: block; }
        .status { font-size: 16px; color: #666; margin: 20px 0; }
        .instructions { font-size: 14px; color: #888; line-height: 1.6; }
        .success { color: #52c41a; }
        .waiting { color: #1890ff; }
        .scaned { color: #faad14; }
        .expired { color: #f5222d; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        .waiting-indicator { animation: pulse 1.5s infinite; }
    </style>
</head>
<body>
    <div class="container">
        <h1>微信扫码登录</h1>
        <div class="qrcode">
            <img src="` + imgSrc + `" alt="QR Code" id="qrcode">
        </div>
        <div class="status" id="status">
            <span class="waiting-indicator">请使用微信扫描二维码</span>
        </div>
        <div class="instructions">
            <p>1. 打开微信扫一扫</p>
            <p>2. 扫描上方二维码</p>
            <p>3. 在手机上确认登录</p>
        </div>
    </div>
    <script>
        let lastStatus = 'waiting';
        function pollStatus() {
            // 使用相对路径，适配不同部署路径
            fetch('login/status')
                .then(r => r.json())
                .then(data => {
                    const statusEl = document.getElementById('status');
                    const qrcodeEl = document.getElementById('qrcode');
                    const status = data.data.status;
                    if (status !== lastStatus) {
                        lastStatus = status;
                        if (status === 'scaned') {
                            statusEl.innerHTML = '<span class="scaned">已扫描，请在手机上确认登录</span>';
                        } else if (status === 'confirmed') {
                            statusEl.innerHTML = '<span class="success">登录成功！正在启动对话服务...</span>';
                            qrcodeEl.style.display = 'none';
                        } else if (status === 'expired') {
                            statusEl.innerHTML = '<span class="expired">二维码已过期，<a href="login">请刷新页面</a></span>';
                        }
                    }
                    if (status !== 'confirmed' && status !== 'expired') {
                        setTimeout(pollStatus, 1000);
                    }
                })
                .catch(() => setTimeout(pollStatus, 2000));
        }
        setTimeout(pollStatus, 1000);
    </script>
</body>
</html>`
}

const loginSuccessHTML = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>微信已登录 - 青龙</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 400px; margin: 50px auto; text-align: center; }
        .container { padding: 40px; background: #f8f9fa; border-radius: 12px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #52c41a; margin-bottom: 20px; font-size: 24px; }
        p { color: #666; font-size: 16px; line-height: 1.6; }
    </style>
</head>
<body>
    <div class="container">
        <h1>✓ 已登录</h1>
        <p>微信已成功连接！</p>
        <p>现在可以在微信中发送消息了。</p>
    </div>
</body>
</html>`

// monitorLoginStatus 后台监控登录状态
func (h *LoginHandler) monitorLoginStatus() {
	handler, err := h.ensureHandler()
	if err != nil {
		logger.GetRunnerLogger().Errorf("[Weixin Login] monitorLoginStatus: %v", err)
		return
	}

	onStatus := func(status string) {
		h.loginStatus = status
		logger.GetRunnerLogger().Infof("[Weixin Login] Status changed: %s", status)
	}

	tokenInfo, err := handler.CompleteQRLogin(context.Background(), h.qrcode, onStatus)
	if err != nil {
		logger.GetRunnerLogger().Warnf("[Weixin Login] Login failed/expired: %v", err)
		return
	}

	logger.GetRunnerLogger().Infof("[Weixin Login] Login successful, token saved, starting long polling")
	manager := GetWsManager()
	if err := manager.StartAfterLogin(context.Background()); err != nil {
		logger.GetRunnerLogger().Errorf("[Weixin Login] Failed to start long polling: %v", err)
	}
	logger.GetRunnerLogger().Infof("[Weixin Login] Token: bot_id=%s, user_id=%s", tokenInfo.ILinkBotID, tokenInfo.ILinkUserID)
}

// Status 登录状态
// GET /weixin/login/status
func (h *LoginHandler) Status(c *gin.Context) {
	handler, err := h.ensureHandler()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"logged_in": false,
				"status":    h.loginStatus,
				"error":     err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"logged_in": !handler.NeedQRLogin(),
			"status":    h.loginStatus,
		},
	})
}
