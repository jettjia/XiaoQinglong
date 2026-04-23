package feishu

// Plugin 飞书插件定义
type Plugin struct {
	authHandler *AuthHandler
}

// NewPlugin 创建飞书插件
func NewPlugin() *Plugin {
	return &Plugin{
		authHandler: NewAuthHandler(),
	}
}

// GetID 获取插件ID
func (p *Plugin) GetID() string {
	return "feishu"
}

// GetName 获取插件名称
func (p *Plugin) GetName() string {
	return "飞书"
}

// GetIcon 获取插件图标
func (p *Plugin) GetIcon() string {
	return "📦"
}

// GetDescription 获取插件描述
func (p *Plugin) GetDescription() string {
	return "搜索飞书文档、知识库，支持云文档、表格、幻灯片"
}

// GetAuthType 获取授权类型
func (p *Plugin) GetAuthType() string {
	return "device" // 飞书使用 Device Flow
}

// GetVersion 获取版本
func (p *Plugin) GetVersion() string {
	return "1.0.0"
}

// GetAuthor 获取作者
func (p *Plugin) GetAuthor() string {
	return "xiaoqinglong"
}

// GetAuthHandler 获取授权处理器
func (p *Plugin) GetAuthHandler() *AuthHandler {
	return p.authHandler
}

// DefaultScopes 默认权限范围
func (p *Plugin) DefaultScopes() string {
	return "docx:document:readonly wiki:space:read"
}
