package channel

// ChannelService 渠道服务
// 注意：此服务仅用于渠道业务逻辑，WS连接管理在 handler 层实现
type ChannelService struct {
}

// NewChannelService 创建渠道服务
func NewChannelService() *ChannelService {
	return &ChannelService{}
}