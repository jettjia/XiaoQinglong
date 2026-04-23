package feishu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jettjia/xiaoqinglong/agent-frame/config"
)

// DeviceFlow endpoints for Feishu
const (
	FeishuAccountsURL = "https://accounts.feishu.cn"
	FeishuOpenURL     = "https://open.feishu.cn"

	DeviceAuthorizationPath = "/oauth/v1/device_authorization"
	TokenPath              = "/open-apis/authen/v2/oauth/token"
)

// DeviceAuthResponse 设备授权响应（参考 feishu-cli device_flow.go）
type DeviceAuthResponse struct {
	DeviceCode             string `json:"device_code"`
	UserCode               string `json:"user_code"`
	VerificationURI        string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn              int    `json:"expires_in"`
	Interval               int    `json:"interval"`
}

// DeviceFlowTokenData 设备流程Token数据（参考 feishu-cli device_flow.go）
type DeviceFlowTokenData struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn  int    `json:"refresh_token_expires_in"`
	Scope            string `json:"scope"`
}

// TokenResponse Token响应
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType       string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_token_expires_in"`
	Scope            string `json:"scope"`
}

// UserInfo 用户信息
type UserInfo struct {
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
	Email   string `json:"email"`
}

// AuthHandler 飞书授权处理器（参考 feishu-cli Device Flow 实现）
type AuthHandler struct {
	appID     string
	appSecret string
	baseURL   string
}

// NewAuthHandler 创建飞书授权处理器
func NewAuthHandler() *AuthHandler {
	cfg := config.NewConfig()

	appID := os.Getenv("FEISHU_APP_ID")
	if appID == "" && cfg.Third.Extra != nil {
		if v, ok := cfg.Third.Extra["feishu_app_id"].(string); ok {
			appID = v
		}
	}

	appSecret := os.Getenv("FEISHU_APP_SECRET")
	if appSecret == "" && cfg.Third.Extra != nil {
		if v, ok := cfg.Third.Extra["feishu_app_secret"].(string); ok {
			appSecret = v
		}
	}

	return &AuthHandler{
		appID:     appID,
		appSecret: appSecret,
		baseURL:   FeishuOpenURL,
	}
}

// RequestDeviceAuthorization 请求设备授权（参考 feishu-cli device_flow.go RequestDeviceAuthorization）
func (h *AuthHandler) RequestDeviceAuthorization(scope string) (*DeviceAuthResponse, error) {
	if h.appID == "" || h.appSecret == "" {
		return nil, fmt.Errorf("feishu app credentials are not configured")
	}

	// 确保包含 offline_access scope 以获取 refresh_token（参考 feishu-cli）
	if !strings.Contains(scope, "offline_access") {
		if scope != "" {
			scope = scope + " offline_access"
		} else {
			scope = "offline_access"
		}
	}

	// 飞书设备授权 endpoint
	authURL := FeishuAccountsURL + DeviceAuthorizationPath

	form := url.Values{}
	form.Set("client_id", h.appID)
	form.Set("scope", scope)

	req, err := http.NewRequest("POST", authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(h.appID+":"+h.appSecret)))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device authorization failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("device authorization failed: read body: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("device authorization failed: response not JSON: %s", string(body))
	}

	// 检查错误（参考 feishu-cli 错误处理）
	if errStr := getStr(data, "error"); errStr != "" {
		msg := getStr(data, "error_description")
		if msg == "" {
			msg = errStr
		}
		return nil, fmt.Errorf("device authorization failed: %s", msg)
	}

	expiresIn := getInt(data, "expires_in", 240)
	interval := getInt(data, "interval", 5)

	verificationURI := getStr(data, "verification_uri")
	verificationURIComplete := getStr(data, "verification_uri_complete")
	if verificationURIComplete == "" {
		verificationURIComplete = verificationURI
	}

	return &DeviceAuthResponse{
		DeviceCode:             getStr(data, "device_code"),
		UserCode:               getStr(data, "user_code"),
		VerificationURI:        verificationURI,
		VerificationURIComplete: verificationURIComplete,
		ExpiresIn:              expiresIn,
		Interval:               interval,
	}, nil
}

// PollDeviceToken 轮询设备Token（参考 feishu-cli device_flow.go PollDeviceToken）
func (h *AuthHandler) PollDeviceToken(ctx context.Context, deviceCode string, interval, expiresIn int) (*DeviceFlowTokenData, error) {
	if h.appID == "" || h.appSecret == "" {
		return nil, fmt.Errorf("feishu app credentials are not configured")
	}

	tokenURL := FeishuOpenURL + TokenPath

	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	currentInterval := interval

	const maxPollInterval = 60
	const maxPollAttempts = 200
	attempts := 0

	for time.Now().Before(deadline) && attempts < maxPollAttempts {
		attempts++

		select {
		case <-time.After(time.Duration(currentInterval) * time.Second):
		case <-ctx.Done():
			return nil, fmt.Errorf("polling cancelled")
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", deviceCode)
		form.Set("client_id", h.appID)
		form.Set("client_secret", h.appSecret)

		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			currentInterval = minInt(currentInterval+1, maxPollInterval)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			currentInterval = minInt(currentInterval+1, maxPollInterval)
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			currentInterval = minInt(currentInterval+1, maxPollInterval)
			continue
		}

		errStr := getStr(data, "error")

		// 授权成功
		if errStr == "" && getStr(data, "access_token") != "" {
			tokenExpiresIn := getInt(data, "expires_in", 7200)
			refreshExpiresIn := getInt(data, "refresh_token_expires_in", 604800)
			refreshToken := getStr(data, "refresh_token")
			if refreshToken == "" {
				refreshExpiresIn = tokenExpiresIn
			}

			return &DeviceFlowTokenData{
				AccessToken:     getStr(data, "access_token"),
				RefreshToken:    refreshToken,
				ExpiresIn:       tokenExpiresIn,
				RefreshExpiresIn: refreshExpiresIn,
				Scope:           getStr(data, "scope"),
			}, nil
		}

		// 处理错误（参考 feishu-cli）
		switch errStr {
		case "authorization_pending":
			continue
		case "slow_down":
			currentInterval = minInt(currentInterval+5, maxPollInterval)
			continue
		case "access_denied":
			return nil, fmt.Errorf("authorization denied by user")
		case "expired_token", "invalid_grant":
			return nil, fmt.Errorf("device code expired, please try again")
		default:
			desc := getStr(data, "error_description")
			if desc == "" {
				desc = errStr
			}
			if desc == "" {
				desc = "Unknown error"
			}
			return nil, fmt.Errorf("authorization failed: %s", desc)
		}
	}

	if attempts >= maxPollAttempts {
		return nil, fmt.Errorf("max poll attempts (%d) reached", maxPollAttempts)
	}

	return nil, fmt.Errorf("authorization timed out")
}

// RefreshToken 刷新token（参考 feishu-cli uat_client.go doRefreshToken）
func (h *AuthHandler) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if h.appID == "" || h.appSecret == "" {
		return nil, fmt.Errorf("feishu app credentials are not configured")
	}

	tokenURL := FeishuOpenURL + TokenPath

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", h.appID)
	form.Set("client_secret", h.appSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	if errStr := getStr(data, "error"); errStr != "" {
		return nil, fmt.Errorf("token refresh failed: %s", getStr(data, "error_description"))
	}

	return &TokenResponse{
		AccessToken:      getStr(data, "access_token"),
		RefreshToken:     getStr(data, "refresh_token"),
		TokenType:        getStr(data, "token_type"),
		ExpiresIn:         getInt(data, "expires_in", 7200),
		RefreshExpiresIn:  getInt(data, "refresh_token_expires_in", 604800),
		Scope:             getStr(data, "scope"),
	}, nil
}

// GetUserInfo 获取用户信息
func (h *AuthHandler) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	apiURL := FeishuOpenURL + "/open-apis/authen/v1/user_info"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data UserInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, err
	}

	if userResp.Code != 0 {
		return nil, fmt.Errorf("get user info failed: %s", userResp.Msg)
	}

	return &userResp.Data, nil
}

// ExchangeCodeForToken 用code交换token（Web OAuth备选）
func (h *AuthHandler) ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error) {
	if h.appID == "" || h.appSecret == "" {
		return nil, fmt.Errorf("feishu app credentials are not configured")
	}

	tokenURL := FeishuOpenURL + TokenPath

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", h.appID)
	form.Set("client_secret", h.appSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	if errStr := getStr(data, "error"); errStr != "" {
		return nil, fmt.Errorf("token exchange failed: %s", getStr(data, "error_description"))
	}

	return &TokenResponse{
		AccessToken:      getStr(data, "access_token"),
		RefreshToken:     getStr(data, "refresh_token"),
		TokenType:        getStr(data, "token_type"),
		ExpiresIn:         getInt(data, "expires_in", 7200),
		RefreshExpiresIn:  getInt(data, "refresh_token_expires_in", 604800),
		Scope:             getStr(data, "scope"),
	}, nil
}

// helpers（参考 feishu-cli device_flow.go）

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string, fallback int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return fallback
}
