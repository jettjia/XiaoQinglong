package weixin

// MessageContext WebSocket 消息上下文
type MessageContext struct {
	ChannelCode string            // 渠道代码
	SessionID   string            // 会话ID (from_user_id)
	UserID      string            // 用户ID (from_user_id)
	Content     string            // 消息内容
	Header      map[string]string // 额外头部信息
}

// ================== 微信协议类型 (参考 goclaw) ==================

// Message item types for Weixin (proto: MessageItemType)
const (
	MessageItemTypeNone  = 0
	MessageItemTypeText  = 1
	MessageItemTypeImage = 2
	MessageItemTypeVoice = 3
	MessageItemTypeFile  = 4
	MessageItemTypeVideo = 5
)

// Message types (proto: MessageType)
const (
	MessageTypeNone = 0
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

// Message states (proto: MessageState)
const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

// Typing status
const (
	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

// Upload media types
const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

// Default Weixin API endpoints
const (
	DefaultWeixinBaseURL = "https://ilinkai.weixin.qq.com"
	DefaultWeixinCDNURL  = "https://novac2c.cdn.weixin.qq.com/c2c"
	DefaultILinkBotType  = "3"
)

// WeixinMessage represents a message from Weixin
type WeixinMessage struct {
	Seq           int64         `json:"seq,omitempty"`
	MessageID     int64         `json:"message_id,omitempty"`
	FromUserID    string        `json:"from_user_id,omitempty"`
	ToUserID      string        `json:"to_user_id,omitempty"`
	ClientID      string        `json:"client_id,omitempty"`
	CreateTimeMs  int64         `json:"create_time_ms,omitempty"`
	UpdateTimeMs  int64         `json:"update_time_ms,omitempty"`
	DeleteTimeMs  int64         `json:"delete_time_ms,omitempty"`
	SessionID     string        `json:"session_id,omitempty"`
	GroupID       string        `json:"group_id,omitempty"`
	MessageType   int           `json:"message_type,omitempty"`
	MessageState  int           `json:"message_state,omitempty"`
	ItemList      []MessageItem `json:"item_list,omitempty"`
	ContextToken  string        `json:"context_token,omitempty"`
}

// MessageItem represents an item in a Weixin message
type MessageItem struct {
	Type         int         `json:"type,omitempty"`
	CreateTimeMs int64       `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64       `json:"update_time_ms,omitempty"`
	IsCompleted  bool        `json:"is_completed,omitempty"`
	MsgID        string      `json:"msg_id,omitempty"`
	RefMsg       *RefMessage `json:"ref_msg,omitempty"`
	TextItem     *TextItem   `json:"text_item,omitempty"`
	ImageItem    *ImageItem  `json:"image_item,omitempty"`
	VoiceItem    *VoiceItem  `json:"voice_item,omitempty"`
	FileItem     *FileItem   `json:"file_item,omitempty"`
	VideoItem    *VideoItem  `json:"video_item,omitempty"`
}

// RefMessage represents a referenced/quoted message
type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

// TextItem represents a text message item
type TextItem struct {
	Text string `json:"text,omitempty"`
}

// CDNMedia represents a CDN media reference
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

// ImageItem represents an image message item
type ImageItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	AESKey      string    `json:"aeskey,omitempty"`
	URL         string    `json:"url,omitempty"`
	MidSize     int       `json:"mid_size,omitempty"`
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	HDSize      int       `json:"hd_size,omitempty"`
}

// VoiceItem represents a voice message item
type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int       `json:"playtime,omitempty"`
	Text          string    `json:"text,omitempty"`
}

// FileItem represents a file message item
type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

// VideoItem represents a video message item
type VideoItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	VideoSize   int       `json:"video_size,omitempty"`
	PlayLength  int       `json:"play_length,omitempty"`
	VideoMD5    string    `json:"video_md5,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
}

// GetUpdatesReq is the request for getUpdates API
type GetUpdatesReq struct {
	SyncBuf       string `json:"sync_buf,omitempty"`
	GetUpdatesBuf string `json:"get_updates_buf,omitempty"`
}

// GetUpdatesResp is the response from getUpdates API
type GetUpdatesResp struct {
	Ret                 int64            `json:"ret,omitempty"`
	ErrCode             int              `json:"errcode,omitempty"`
	ErrMsg              string           `json:"errmsg,omitempty"`
	Msgs                []*WeixinMessage `json:"msgs,omitempty"`
	SyncBuf             string           `json:"sync_buf,omitempty"`
	GetUpdatesBuf       string           `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMs int             `json:"longpolling_timeout_ms,omitempty"`
}

// SendMessageReq is the request for sendMessage API
type SendMessageReq struct {
	ToUserID     string        `json:"to_user_id,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
}

// GetConfigResp is the response from getConfig API
type GetConfigResp struct {
	Ret          int    `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

// TokenInfo stores token information
type TokenInfo struct {
	Token       string `json:"token,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	ExpiresAt   int64  `json:"expires_at,omitempty"`
}

// QRCodeResponse is the response from get_bot_qrcode API
type QRCodeResponse struct {
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

// QRCodeStatusResponse is the response from get_qrcode_status API
type QRCodeStatusResponse struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token"`
	ILinkBotID  string `json:"ilink_bot_id"`
	BaseURL     string `json:"baseurl"`
	ILinkUserID string `json:"ilink_user_id"`
}