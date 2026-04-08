package public

import (
	"github.com/gin-gonic/gin"

	handUser "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/user"
	handConfig "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/config"
	handModel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/model"
	handKnowledgeBase "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/knowledge_base"
	handSkill "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/skill"
	handAgent "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/agent"
	handChannel "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	handChat "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/chat"
	handRunner "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/runner"
	handJob "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/job"
	handDashboard "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/dashboard"
	handCommand "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/command"

	channelDispatcher "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel"
	feishuHandler "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel/feishu"
	weixinHandler "github.com/jettjia/xiaoqinglong/agent-frame/api/http/handler/public/channel/weixin"
)

func SetPublicRouter(Router *gin.RouterGroup) {
	handUser := handUser.NewHandler()
	handConfig := handConfig.NewHandler()
	handModel := handModel.NewHandler()
	handKnowledgeBase := handKnowledgeBase.NewHandler()
	handSkill := handSkill.NewHandler()
	handAgent := handAgent.NewHandler()
	handChannel := handChannel.NewHandler()
	handChat := handChat.NewHandler()
	handRunner := handRunner.NewHandler()
	handJob := handJob.NewHandler()
	handDashboard := handDashboard.NewHandler()
	handCommand := handCommand.NewHandler()

	GRouter := Router.Group("/user")
	{

		// user
		GRouter.POST("/user", handUser.CreateSysUser)              // 创建
		GRouter.DELETE("/user/:ulid", handUser.DeleteSysUser)      // 删除
		GRouter.PUT("/user/:ulid", handUser.UpdateSysUser)         // 修改
		GRouter.GET("/user/:ulid", handUser.FindSysUserById)       // 查询ByID
		GRouter.POST("/user/byQuery", handUser.FindSysUserByQuery) // 查询ByQuery
		GRouter.POST("/user/byAll", handUser.FindSysUserAll)       // 查询ByAll
		GRouter.POST("/userPage", handUser.FindSysUserPage)        // 查询分页
	}

	// config
	CRouter := Router.Group("/config")
	{
		CRouter.GET("/app", handConfig.GetAppConfig)               // 获取应用配置
		CRouter.PUT("/app", handConfig.SaveAppConfig)             // 保存应用配置
		CRouter.GET("/skills", handConfig.GetSkillsConfig)        // 获取技能配置
		CRouter.PUT("/skills", handConfig.SaveSkillsConfig)       // 保存技能配置
	}

	// model
	MRouter := Router.Group("/model")
	{
		MRouter.POST("", handModel.CreateSysModel)              // 创建
		MRouter.DELETE("/:ulid", handModel.DeleteSysModel)     // 删除
		MRouter.PUT("/:ulid", handModel.UpdateSysModel)        // 修改
		MRouter.GET("/:ulid", handModel.FindSysModelById)      // 查询ByID
		MRouter.POST("/all", handModel.FindSysModelAll)         // 查询所有
		MRouter.POST("/page", handModel.FindSysModelPage)       // 分页查询
	}

	// knowledge_base
	KBRouter := Router.Group("/knowledge_base")
	{
		KBRouter.POST("", handKnowledgeBase.CreateSysKnowledgeBase)              // 创建
		KBRouter.DELETE("/:ulid", handKnowledgeBase.DeleteSysKnowledgeBase)     // 删除
		KBRouter.PUT("/:ulid", handKnowledgeBase.UpdateSysKnowledgeBase)        // 修改
		KBRouter.GET("/:ulid", handKnowledgeBase.FindSysKnowledgeBaseById)      // 查询ByID
		KBRouter.POST("/all", handKnowledgeBase.FindSysKnowledgeBaseAll)         // 查询所有
		KBRouter.POST("/page", handKnowledgeBase.FindSysKnowledgeBasePage)      // 分页查询
		KBRouter.POST("/:ulid/recall", handKnowledgeBase.RecallTest)            // 召回测试
	}

	// skill
	SkillRouter := Router.Group("/skill")
	{
		SkillRouter.POST("", handSkill.CreateSysSkill)              // 创建
		SkillRouter.DELETE("/:ulid", handSkill.DeleteSysSkill)     // 删除
		SkillRouter.PUT("/:ulid", handSkill.UpdateSysSkill)        // 修改
		SkillRouter.GET("/:ulid", handSkill.FindSysSkillById)      // 查询ByID
		SkillRouter.POST("/all", handSkill.FindSysSkillAll)         // 查询所有
		SkillRouter.POST("/page", handSkill.FindSysSkillPage)      // 分页查询
		SkillRouter.POST("/upload", handSkill.UploadSysSkill)      // 上传安装
		SkillRouter.POST("/check-name", handSkill.CheckSkillName)  // 检查同名
	}

	// agent
	AgentRouter := Router.Group("/agent")
	{
		AgentRouter.POST("", handAgent.CreateSysAgent)              // 创建
		AgentRouter.DELETE("/:ulid", handAgent.DeleteSysAgent)     // 删除
		AgentRouter.PUT("/:ulid", handAgent.UpdateSysAgent)        // 修改
		AgentRouter.GET("/:ulid", handAgent.FindSysAgentById)      // 查询ByID
		AgentRouter.POST("/all", handAgent.FindSysAgentAll)         // 查询所有
		AgentRouter.POST("/page", handAgent.FindSysAgentPage)      // 分页查询
		AgentRouter.POST("/upload", handAgent.UploadSysAgent)      // 上传导入
		AgentRouter.PUT("/:ulid/enabled", handAgent.UpdateSysAgentEnabled) // 修改启用状态
	}

	// channel
	ChannelRouter := Router.Group("/channel")
	{
		ChannelRouter.POST("", handChannel.CreateSysChannel)              // 创建
		ChannelRouter.DELETE("/:ulid", handChannel.DeleteSysChannel)     // 删除
		ChannelRouter.PUT("/:ulid", handChannel.UpdateSysChannel)        // 修改
		ChannelRouter.GET("/:ulid", handChannel.FindSysChannelById)      // 查询ByID
		ChannelRouter.POST("/all", handChannel.FindSysChannelAll)         // 查询所有
		ChannelRouter.POST("/page", handChannel.FindSysChannelPage)      // 分页查询
	}

	// channel callback (飞书、微信等渠道回调)
	dispatcherSvc := channelDispatcher.NewChannelDispatcher()
	feishuHdlr := feishuHandler.NewHandler()
	feishuOut := feishuHandler.NewOutboundHandler()
	dispatcherSvc.RegisterInboundHandler("feishu", feishuHdlr)
	dispatcherSvc.RegisterOutboundHandler("feishu", feishuOut)

	CallbackRouter := Router.Group("/callback")
	{
		CallbackRouter.POST("/:channel", dispatcherSvc.HandleCallback()) // 渠道回调
	}

	// weixin login
	weixinLoginHdlr := weixinHandler.NewLoginHandler()
	WeixinLoginRouter := Router.Group("/weixin")
	{
		WeixinLoginRouter.GET("/login", weixinLoginHdlr.Login)   // 扫码登录（获取二维码+后台监控）
		WeixinLoginRouter.GET("/login/status", weixinLoginHdlr.Status) // 查询登录状态
	}

	// chat
	ChatRouter := Router.Group("/chat")
	{
		// session
		ChatRouter.POST("/session", handChat.CreateChatSession)                              // 创建会话
		ChatRouter.DELETE("/session/:ulid", handChat.DeleteChatSession)                     // 删除会话
		ChatRouter.PUT("/session", handChat.UpdateChatSession)                              // 更新会话
		ChatRouter.PUT("/session/status", handChat.UpdateChatSessionStatus)                 // 更新会话状态
		ChatRouter.GET("/session/:ulid", handChat.FindChatSessionById)                     // 查询会话byId
		ChatRouter.POST("/session/byUserId", handChat.FindChatSessionsByUserId)             // 查询用户会话列表
		ChatRouter.POST("/session/page", handChat.FindChatSessionPage)                      // 分页查询会话

		// message
		ChatRouter.POST("/message", handChat.CreateChatMessage)                              // 创建消息
		ChatRouter.PUT("/message", handChat.UpdateChatMessage)                              // 更新消息
		ChatRouter.PUT("/message/status", handChat.UpdateChatMessageStatus)                 // 更新消息状态
		ChatRouter.GET("/message/:ulid", handChat.FindChatMessageById)                     // 查询消息byId
		ChatRouter.POST("/message/bySessionId", handChat.FindChatMessagesBySessionId)       // 查询会话消息列表

		// approval
		ChatRouter.POST("/approval", handChat.CreateChatApproval)                          // 创建审批
		ChatRouter.PUT("/approval/approve", handChat.ApproveChatApproval)                 // 批准审批
		ChatRouter.PUT("/approval/reject", handChat.RejectChatApproval)                   // 拒绝审批
		ChatRouter.GET("/approval/:ulid", handChat.FindChatApprovalById)                  // 查询审批byId
		ChatRouter.POST("/approval/byMessageId", handChat.FindChatApprovalByMessageId)   // 查询消息审批
		ChatRouter.POST("/approval/pending", handChat.FindPendingChatApprovals)          // 查询待审批列表
		ChatRouter.POST("/approval/byUserId", handChat.FindChatApprovalsByUserId)          // 查询用户审批列表
	}

	// runner proxy
	RunnerRouter := Router.Group("/runner")
	{
		RunnerRouter.POST("/run", handRunner.Run)          // 代理runner run请求
		RunnerRouter.POST("/resume", handRunner.Resume)    // 代理runner resume请求
		RunnerRouter.POST("/stop", handRunner.Stop)        // 代理runner stop请求
		RunnerRouter.POST("/upload", handRunner.Upload)      // 文件上传
		RunnerRouter.POST("/memory", handRunner.SaveMemoriesHandler) // runner回调保存记忆
		RunnerRouter.GET("/reports/:sessionID/:filename", handRunner.ServeReports) // HTML报告文件访问
	}

	// job execution
	JobRouter := Router.Group("/job")
	{
		JobRouter.GET("/execution/:ulid", handJob.FindJobExecutionById)                      // 查询执行记录byId
		JobRouter.GET("/execution/byAgentId", handJob.FindJobExecutionByAgentId)             // 根据AgentId查询执行记录
		JobRouter.POST("/execution/page", handJob.FindJobExecutionPage)                       // 分页查询执行记录
	}

	// dashboard
	DashboardRouter := Router.Group("/dashboard")
	{
		DashboardRouter.GET("/overview", handDashboard.GetOverview)                 // 概览统计
		DashboardRouter.GET("/token-ranking", handDashboard.GetTokenRanking)      // Token排行
		DashboardRouter.GET("/channel-activity", handDashboard.GetChannelActivity)  // 渠道活动
		DashboardRouter.GET("/recent-sessions", handDashboard.GetRecentSessions)   // 最近会话
	}

	// command - 魔法盒命令执行
	CommandRouter := Router.Group("/command")
	{
		CommandRouter.POST("/execute", handCommand.Execute) // 执行命令
	}
}
