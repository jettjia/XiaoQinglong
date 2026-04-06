package weixin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WeixinAPIClient is the HTTP client for Weixin API
type WeixinAPIClient struct {
	baseURL    string
	token     string
	httpClient *http.Client
}

// NewWeixinAPIClient creates a new Weixin API client
func NewWeixinAPIClient(baseURL, token string) *WeixinAPIClient {
	if baseURL == "" {
		baseURL = DefaultWeixinBaseURL
	}
	return &WeixinAPIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetToken sets the authentication token
func (c *WeixinAPIClient) SetToken(token string) {
	c.token = token
}

// GetToken returns the current token
func (c *WeixinAPIClient) GetToken() string {
	return c.token
}

// generateWechatUIN generates a random X-WECHAT-UIN header value
func generateWechatUIN() string {
	b := make([]byte, 4)
	rand.Read(b)
	uint32Val := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", uint32Val)))
}

// buildHeaders builds the common headers for API requests
func buildHeaders(token string, bodyLength int) map[string]string {
	headers := map[string]string{
		"Content-Type":      "application/json",
		"AuthorizationType": "ilink_bot_token",
		"X-WECHAT-UIN":      generateWechatUIN(),
	}
	if bodyLength > 0 {
		headers["Content-Length"] = fmt.Sprintf("%d", bodyLength)
	}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

// ensureTrailingSlash ensures URL ends with /
func ensureTrailingSlash(u string) string {
	if len(u) > 0 && u[len(u)-1] != '/' {
		return u + "/"
	}
	return u
}

// ReaderAt wraps a byte slice to implement io.Reader
type ReaderAt struct {
	data []byte
	pos  int
}

// NewReaderAt creates a new ReaderAt
func NewReaderAt(data []byte) *ReaderAt {
	return &ReaderAt{data: data}
}

// Read implements io.Reader
func (r *ReaderAt) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// doJSONRequest performs a POST JSON request to the Weixin API
func (c *WeixinAPIClient) doJSONRequest(ctx context.Context, endpoint string, reqBody, respBody interface{}) error {
	var bodyReader io.Reader
	var bodyLen int

	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = NewReaderAt(jsonData)
		bodyLen = len(jsonData)
	}

	baseURL := ensureTrailingSlash(c.baseURL)
	fullURL := baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	headers := buildHeaders(c.token, bodyLen)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respData))
	}

	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// GetUpdates fetches new messages using long polling
func (c *WeixinAPIClient) GetUpdates(ctx context.Context, req *GetUpdatesReq) (*GetUpdatesResp, error) {
	body := map[string]interface{}{
		"get_updates_buf": req.GetUpdatesBuf,
		"base_info": map[string]string{
			"channel_version": "1.0.0",
		},
	}

	resp := &GetUpdatesResp{}
	if err := c.doJSONRequest(ctx, "ilink/bot/getupdates", body, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendMessage sends a message to a user
func (c *WeixinAPIClient) SendMessage(ctx context.Context, req *SendMessageReq) error {
	body := map[string]interface{}{
		"msg":        req,
		"base_info": map[string]string{
			"channel_version": "1.0.0",
		},
	}
	return c.doJSONRequest(ctx, "ilink/bot/sendmessage", body, nil)
}

// GetConfig gets account configuration including typing ticket
func (c *WeixinAPIClient) GetConfig(ctx context.Context, ilinkUserID, contextToken string) (*GetConfigResp, error) {
	body := map[string]interface{}{
		"ilink_user_id": ilinkUserID,
		"context_token":  contextToken,
		"base_info": map[string]string{
			"channel_version": "1.0.0",
		},
	}

	resp := &GetConfigResp{}
	if err := c.doJSONRequest(ctx, "ilink/bot/getconfig", body, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendTyping sends typing indicator
func (c *WeixinAPIClient) SendTyping(ctx context.Context, ilinkUserID, typingTicket string, status int) error {
	body := map[string]interface{}{
		"ilink_user_id": ilinkUserID,
		"typing_ticket":  typingTicket,
		"status":         status,
		"base_info": map[string]string{
			"channel_version": "1.0.0",
		},
	}
	return c.doJSONRequest(ctx, "ilink/bot/sendtyping", body, nil)
}

// GetBotQRCode gets the QR code for bot login (GET request)
func (c *WeixinAPIClient) GetBotQRCode(ctx context.Context) (*QRCodeResponse, error) {
	baseURL := ensureTrailingSlash(c.baseURL)
	endpoint := fmt.Sprintf("ilink/bot/get_bot_qrcode?bot_type=%s", url.QueryEscape(DefaultILinkBotType))
	fullURL := baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch QR code: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("QR code request failed with status %d: %s", resp.StatusCode, string(respData))
	}

	var qrResp QRCodeResponse
	if err := json.Unmarshal(respData, &qrResp); err != nil {
		return nil, fmt.Errorf("failed to parse QR code response: %w", err)
	}

	return &qrResp, nil
}

// FetchQRCodeImage fetches the QR code image from the given URL and returns as base64 data URL
func (c *WeixinAPIClient) FetchQRCodeImage(ctx context.Context, imageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch QR code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch QR code failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Return as data URL
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

// GetQRCodeStatus checks the QR code scan status (GET request)
func (c *WeixinAPIClient) GetQRCodeStatus(ctx context.Context, qrcode string) (*QRCodeStatusResponse, error) {
	baseURL := ensureTrailingSlash(c.baseURL)
	endpoint := fmt.Sprintf("ilink/bot/get_qrcode_status?qrcode=%s", url.QueryEscape(qrcode))
	fullURL := baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("iLink-App-ClientVersion", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to poll QR status: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("QR status request failed with status %d: %s", resp.StatusCode, string(respData))
	}

	var statusResp QRCodeStatusResponse
	if err := json.Unmarshal(respData, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse QR status response: %w", err)
	}

	return &statusResp, nil
}
