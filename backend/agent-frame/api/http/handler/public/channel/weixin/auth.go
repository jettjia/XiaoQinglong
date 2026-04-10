package weixin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/logger"
	"github.com/jettjia/xiaoqinglong/agent-frame/pkg/xqldir"
	"github.com/skip2/go-qrcode"
)

// TokenStore manages token persistence
type TokenStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewTokenStore creates a new token store
func NewTokenStore() (*TokenStore, error) {
	baseDir := filepath.Join(xqldir.GetBaseDir(), "weixin", "accounts")

	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create token directory: %w", err)
	}

	return &TokenStore{baseDir: baseDir}, nil
}

// Save saves token info for an account
func (s *TokenStore) Save(accountID string, info *TokenInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.tokenPath(accountID)
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token info: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	logger.GetRunnerLogger().Infof("[Weixin Auth] Token saved for account: %s", accountID)
	return nil
}

// Load loads token info for an account
func (s *TokenStore) Load(accountID string) (*TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.tokenPath(accountID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No token file exists
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var info TokenInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token info: %w", err)
	}

	return &info, nil
}

// Delete removes token info for an account
func (s *TokenStore) Delete(accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.tokenPath(accountID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token file: %w", err)
	}

	return nil
}

// tokenPath returns the file path for an account's token
func (s *TokenStore) tokenPath(accountID string) string {
	return filepath.Join(s.baseDir, accountID+".json")
}

// WeixinAuth handles Weixin authentication
type WeixinAuth struct {
	accountID  string
	apiClient  *WeixinAPIClient
	tokenStore *TokenStore
}

// NewWeixinAuth creates a new auth handler
func NewWeixinAuth(accountID string) (*WeixinAuth, error) {
	tokenStore, err := NewTokenStore()
	if err != nil {
		return nil, err
	}

	apiClient := NewWeixinAPIClient(DefaultWeixinBaseURL, "")

	return &WeixinAuth{
		accountID:  accountID,
		apiClient:  apiClient,
		tokenStore: tokenStore,
	}, nil
}

// LoadToken loads the stored token
func (a *WeixinAuth) LoadToken() (*TokenInfo, error) {
	return a.tokenStore.Load(a.accountID)
}

// SaveToken saves the token
func (a *WeixinAuth) SaveToken(info *TokenInfo) error {
	return a.tokenStore.Save(a.accountID, info)
}

// DeleteToken removes the stored token
func (a *WeixinAuth) DeleteToken() error {
	return a.tokenStore.Delete(a.accountID)
}

// StartQRLogin initiates QR code login flow
func (a *WeixinAuth) StartQRLogin(ctx context.Context) (*QRCodeResponse, error) {
	resp, err := a.apiClient.GetBotQRCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get QR code: %w", err)
	}

	logger.GetRunnerLogger().Infof("[Weixin Auth] QR code generated, length: %d", len(resp.QRCodeImgContent))
	return resp, nil
}

// WaitForLogin waits for the user to scan the QR code
func (a *WeixinAuth) WaitForLogin(ctx context.Context, qrcode string, onStatus func(status string)) (*TokenInfo, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			status, err := a.apiClient.GetQRCodeStatus(ctx, qrcode)
			if err != nil {
				logger.GetRunnerLogger().Warnf("[Weixin Auth] Failed to check QR code status: %v", err)
				continue
			}

			if onStatus != nil {
				onStatus(status.Status)
			}

			switch status.Status {
			case "wait":
				logger.GetRunnerLogger().Debug("[Weixin Auth] Waiting for QR code scan...")
			case "scaned":
				logger.GetRunnerLogger().Info("[Weixin Auth] QR code scanned, waiting for confirmation")
			case "confirmed":
				logger.GetRunnerLogger().Info("[Weixin Auth] Login confirmed",
					"ilink_bot_id", status.ILinkBotID,
					"ilink_user_id", status.ILinkUserID)

				return &TokenInfo{
					Token:       status.BotToken,
					ILinkBotID:  status.ILinkBotID,
					ILinkUserID: status.ILinkUserID,
					BaseURL:     status.BaseURL,
				}, nil
			case "expired":
				return nil, fmt.Errorf("QR code expired")
			}
		}
	}
}

// LoginWithQRCode performs the complete QR code login flow
func (a *WeixinAuth) LoginWithQRCode(ctx context.Context, onStatus func(status string)) (*TokenInfo, error) {
	// Get QR code
	qrResp, err := a.StartQRLogin(ctx)
	if err != nil {
		return nil, err
	}

	// Wait for login
	tokenInfo, err := a.WaitForLogin(ctx, qrResp.QRCode, onStatus)
	if err != nil {
		return nil, err
	}

	// Save token
	if err := a.SaveToken(tokenInfo); err != nil {
		logger.GetRunnerLogger().Warnf("[Weixin Auth] Failed to save token: %v", err)
	}

	return tokenInfo, nil
}

// IsTokenValid checks if a token is still valid
func (a *WeixinAuth) IsTokenValid(info *TokenInfo) bool {
	if info == nil || info.Token == "" {
		return false
	}

	// Check expiration with 5 minute buffer
	if info.ExpiresAt > 0 && time.Now().Unix() > info.ExpiresAt-300 {
		return false
	}

	return true
}

// GetQRCodeImage returns the QR code image content for display
// Returns (base64Image, qrcodeString, error)
func (a *WeixinAuth) GetQRCodeImage(ctx context.Context) (string, string, error) {
	qrResp, err := a.StartQRLogin(ctx)
	if err != nil {
		return "", "", err
	}

	// Use QRCodeImgContent (the LiteApp URL) as the QR code data
	// When scanned, this opens the LiteApp page with login confirmation button
	qrData := qrResp.QRCodeImgContent
	if qrData == "" {
		return "", "", errors.New("empty QR code URL")
	}

	// Generate QR code image as PNG from the LiteApp URL
	pngData, err := qrcode.Encode(qrData, qrcode.Medium, 256)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Return as base64 data URL and the original qrcode string for polling
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData), qrResp.QRCode, nil
}

// PollQRCodeStatus polls the QR code status and returns when confirmed
func (a *WeixinAuth) PollQRCodeStatus(ctx context.Context, qrcode string, onStatus func(status string)) (*TokenInfo, error) {
	return a.WaitForLogin(ctx, qrcode, onStatus)
}
