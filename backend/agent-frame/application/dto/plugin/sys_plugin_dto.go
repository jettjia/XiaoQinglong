package plugin

// 请求对象
type (
	// GetPluginsReq 获取插件列表请求
	GetPluginsReq struct {
	}

	// GetUserInstancesReq 获取用户插件实例请求
	GetUserInstancesReq struct {
	}

	// StartAuthReq 开始授权请求
	StartAuthReq struct {
		PluginID    string `json:"plugin_id" validate:"required"`     // 插件ID
		CallbackURL string `json:"callback_url"`                      // 回调URL
	}

	// PollAuthReq 轮询授权状态请求
	PollAuthReq struct {
		State string `json:"state" validate:"required"` // OAuth state
	}

	// DeleteInstanceReq 删除插件实例请求
	DeleteInstanceReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"` // ulid
	}

	// RefreshTokenReq 刷新令牌请求
	RefreshTokenReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"` // ulid
	}

	// GetInstanceByIdReq 获取实例详情请求
	GetInstanceByIdReq struct {
		Ulid string `validate:"required" uri:"ulid" json:"ulid"` // ulid
	}
)

// 输出对象
type (
	// GetPluginsRsp 获取插件列表响应
	GetPluginsRsp struct {
		Plugins []PluginItem `json:"plugins"`
	}

	// PluginItem 插件项
	PluginItem struct {
		ID          string `json:"id"`           // 插件ID
		Name        string `json:"name"`         // 插件名称
		Icon        string `json:"icon"`         // 插件图标
		Description string `json:"description"`  // 插件描述
		AuthType    string `json:"auth_type"`   // 授权类型
		Version     string `json:"version"`     // 插件版本
		Author      string `json:"author"`       // 插件作者
		Status      string `json:"status"`       // 状态: available, installed, authorized
		InstanceID  string `json:"instance_id"`  // 实例ID（如果已安装）
	}

	// GetUserInstancesRsp 获取用户插件实例响应
	GetUserInstancesRsp struct {
		Instances []InstanceItem `json:"instances"`
	}

	// InstanceItem 实例项
	InstanceItem struct {
		Ulid         string          `json:"ulid"`          // 实例ID
		PluginID     string          `json:"plugin_id"`     // 插件ID
		Status       string          `json:"status"`        // 状态: active, revoked, expired
		UserInfo     *PluginUserInfo `json:"user_info"`     // 用户信息
		AuthorizedAt int64           `json:"authorized_at"` // 授权时间
		ExpiresAt    *int64          `json:"expires_at"`   // 过期时间
	}

	// PluginUserInfo 用户信息
	PluginUserInfo struct {
		OpenID string `json:"open_id"` // 数据源用户ID
		Name   string `json:"name"`   // 用户名称
		Avatar string `json:"avatar"` // 用户头像
		Email  string `json:"email"`  // 用户邮箱
	}

	// StartAuthRsp 开始授权响应
	StartAuthRsp struct {
		AuthType       string `json:"auth_type"`         // 授权类型: oauth2, device
		AuthURL        string `json:"auth_url"`          // 授权URL
		State          string `json:"state"`             // OAuth state
		DeviceCode     string `json:"device_code,omitempty"` // 设备码（device flow）
		UserCode       string `json:"user_code,omitempty"`   // 用户码（device flow）
		VerificationURL string `json:"verification_url,omitempty"` // 验证URL（device flow）
		ExpiresIn      int    `json:"expires_in,omitempty"`      // 过期时间（秒）
		Interval       int    `json:"interval,omitempty"`        // 轮询间隔（秒）
	}

	// PollAuthRsp 轮询授权状态响应
	PollAuthRsp struct {
		Status     string          `json:"status"`      // pending, authorized, expired
		InstanceID string          `json:"instance_id"`  // 实例ID（授权成功后）
		UserInfo   *PluginUserInfo `json:"user_info"`   // 用户信息（授权成功后）
	}

	// DeleteInstanceRsp 删除插件实例响应
	DeleteInstanceRsp struct {
	}

	// RefreshTokenRsp 刷新令牌响应
	RefreshTokenRsp struct {
		Status string `json:"status"` // active, expired
	}

	// GetInstanceByIdRsp 获取实例详情响应
	GetInstanceByIdRsp struct {
		Ulid         string          `json:"ulid"`
		PluginID     string          `json:"plugin_id"`
		Status       string          `json:"status"`
		Config       string          `json:"config"`
		UserInfo     *PluginUserInfo `json:"user_info"`
		AuthorizedAt int64           `json:"authorized_at"`
		ExpiresAt    *int64          `json:"expires_at"`
		CreatedAt    int64           `json:"created_at"`
		UpdatedAt    int64           `json:"updated_at"`
	}

	// GetPublicKeyRsp 获取RSA公钥响应
	GetPublicKeyRsp struct {
		PublicKey string `json:"public_key"` // PEM格式的RSA公钥
	}
)